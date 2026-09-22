#include "desync.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <signal.h>
#include <unistd.h>
#include <getopt.h>
#include <sys/socket.h>
#include <netinet/in.h>
#include <linux/netfilter.h>
#include <libnetfilter_queue/libnetfilter_queue.h>

static volatile sig_atomic_t g_running = 1;
static DesyncConfig g_cfg;
static int g_raw_sock4 = -1;
static int g_raw_sock6 = -1;

static void handle_signal(int sig) {
    (void)sig;
    g_running = 0;
}

static int pkt_callback(struct nfq_q_handle *qh, struct nfgenmsg *nfmsg,
                        struct nfq_data *nfa, void *data) {
    (void)nfmsg;
    (void)data;

    uint32_t id = 0;
    struct nfqnl_msg_packet_hdr *ph = nfq_get_msg_packet_hdr(nfa);
    if (ph) {
        id = ntohl(ph->packet_id);
    }

    unsigned char *payload = NULL;
    int payload_len = nfq_get_payload(nfa, &payload);

    if (payload_len > 0 && payload != NULL) {
        bool desynced = desync_tcp_packet(g_raw_sock4, g_raw_sock6, payload, (size_t)payload_len, &g_cfg);
        if (desynced) {
            // Drop original packet because desync injected split/fake segments
            return nfq_set_verdict(qh, id, NF_DROP, 0, NULL);
        }
    }

    // Accept unmodified packet
    return nfq_set_verdict(qh, id, NF_ACCEPT, 0, NULL);
}

static void print_usage(const char *prog) {
    printf("Usage: %s [options]\n", prog);
    printf("Options:\n");
    printf("  --qnum=<num>                 NFQUEUE number (default 200)\n");
    printf("  --hostlist=<file>            Allowlist of domains to desynchronize\n");
    printf("  --dpi-desync=<mode>          Desync mode: split2, fake, disorder2\n");
    printf("  --dpi-desync-ttl=<num>       TTL for fake packets (default 4)\n");
    printf("  --dpi-desync-split-pos=<num> Byte offset to split ClientHello (default 2)\n");
    printf("  --help                       Show this help\n");
}

int main(int argc, char **argv) {
    signal(SIGINT, handle_signal);
    signal(SIGTERM, handle_signal);

    memset(&g_cfg, 0, sizeof(g_cfg));
    g_cfg.qnum = 200;
    g_cfg.mode = DESYNC_SPLIT2;
    g_cfg.split_pos = 2;
    g_cfg.fake_ttl = 4;

    static struct option long_options[] = {
        {"qnum", required_argument, 0, 'q'},
        {"hostlist", required_argument, 0, 'h'},
        {"dpi-desync", required_argument, 0, 'd'},
        {"dpi-desync-ttl", required_argument, 0, 't'},
        {"dpi-desync-split-pos", required_argument, 0, 's'},
        {"help", no_argument, 0, '?'},
        {0, 0, 0, 0}
    };

    int opt;
    while ((opt = getopt_long_only(argc, argv, "", long_options, NULL)) != -1) {
        switch (opt) {
        case 'q':
            g_cfg.qnum = atoi(optarg);
            break;
        case 'h':
            strncpy(g_cfg.hostlist_file, optarg, sizeof(g_cfg.hostlist_file) - 1);
            if (hostlist_load(&g_cfg.hostlist, g_cfg.hostlist_file)) {
                g_cfg.has_hostlist = true;
                printf("[nfqws] Loaded %zu domains from %s\n", g_cfg.hostlist.count, g_cfg.hostlist_file);
            } else {
                fprintf(stderr, "[nfqws] Warning: could not load hostlist from %s\n", g_cfg.hostlist_file);
            }
            break;
        case 'd':
            if (strstr(optarg, "fake")) {
                g_cfg.mode = DESYNC_FAKE;
            } else if (strstr(optarg, "disorder")) {
                g_cfg.mode = DESYNC_DISORDER2;
            } else {
                g_cfg.mode = DESYNC_SPLIT2;
            }
            break;
        case 't':
            g_cfg.fake_ttl = atoi(optarg);
            break;
        case 's':
            g_cfg.split_pos = atoi(optarg);
            if (g_cfg.split_pos <= 0) g_cfg.split_pos = 2;
            break;
        case '?':
            print_usage(argv[0]);
            return 0;
        default:
            break;
        }
    }

    // 1. Open raw sockets for packet injection
    g_raw_sock4 = socket(AF_INET, SOCK_RAW, IPPROTO_RAW);
    if (g_raw_sock4 >= 0) {
        int one = 1;
        setsockopt(g_raw_sock4, IPPROTO_IP, IP_HDRINCL, &one, sizeof(one));
    } else {
        perror("[nfqws] socket(AF_INET, SOCK_RAW) error");
    }

    g_raw_sock6 = socket(AF_INET6, SOCK_RAW, IPPROTO_RAW);
    if (g_raw_sock6 >= 0) {
        int one = 1;
        setsockopt(g_raw_sock6, IPPROTO_IPV6, IPV6_CHECKSUM, &one, sizeof(one));
    }

    // 2. Setup Netfilter Queue
    struct nfq_handle *h = nfq_open();
    if (!h) {
        fprintf(stderr, "[nfqws] Error: nfq_open() failed (are you root with CAP_NET_ADMIN?)\n");
        return 1;
    }

    if (nfq_unbind_pf(h, AF_INET) < 0) {
        // Not fatal
    }

    if (nfq_bind_pf(h, AF_INET) < 0) {
        // Not fatal
    }

    struct nfq_q_handle *qh = nfq_create_queue(h, (uint16_t)g_cfg.qnum, &pkt_callback, NULL);
    if (!qh) {
        fprintf(stderr, "[nfqws] Error: nfq_create_queue(%d) failed\n", g_cfg.qnum);
        nfq_close(h);
        return 1;
    }

    if (nfq_set_mode(qh, NFQNL_COPY_PACKET, 0xffff) < 0) {
        fprintf(stderr, "[nfqws] Error: nfq_set_mode(NFQNL_COPY_PACKET) failed\n");
        nfq_destroy_queue(qh);
        nfq_close(h);
        return 1;
    }

    int fd = nfq_fd(h);
    printf("[nfqws] Listening on NFQUEUE num=%d (mode=%s, split_pos=%d)\n",
           g_cfg.qnum,
           g_cfg.mode == DESYNC_FAKE ? "fake+split2" : "split2",
           g_cfg.split_pos);

    char buf[65536] __attribute__((aligned));
    while (g_running) {
        int rv = recv(fd, buf, sizeof(buf), 0);
        if (rv >= 0) {
            nfq_handle_packet(h, buf, rv);
        } else {
            if (!g_running) break;
        }
    }

    printf("[nfqws] Shutting down queue %d...\n", g_cfg.qnum);
    nfq_destroy_queue(qh);
    nfq_close(h);
    if (g_raw_sock4 >= 0) close(g_raw_sock4);
    if (g_raw_sock6 >= 0) close(g_raw_sock6);

    return 0;
}
