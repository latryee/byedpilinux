#include "desync.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <strings.h>
#include <unistd.h>
#include <arpa/inet.h>
#include <sys/socket.h>

bool hostlist_load(HostList *hl, const char *filename) {
    FILE *f = fopen(filename, "r");
    if (!f) return false;

    hl->count = 0;
    char line[512];
    while (fgets(line, sizeof(line), f) && hl->count < MAX_HOSTS) {
        // Strip newline and whitespace
        char *p = line;
        while (*p == ' ' || *p == '\t') p++;
        char *end = p + strlen(p) - 1;
        while (end >= p && (*end == '\n' || *end == '\r' || *end == ' ' || *end == '\t')) {
            *end = '\0';
            end--;
        }
        if (*p == '\0' || *p == '#') continue;

        snprintf(hl->hosts[hl->count], MAX_HOST_LEN, "%s", p);
        hl->count++;
    }
    fclose(f);
    return true;
}

bool hostlist_matches(const HostList *hl, const char *hostname) {
    if (!hl || hl->count == 0) return true; // No hostlist means match all
    if (!hostname || *hostname == '\0') return false;

    for (size_t i = 0; i < hl->count; i++) {
        const char *pattern = hl->hosts[i];
        if (strcasecmp(pattern, hostname) == 0) {
            return true;
        }
        // Subdomain matching: if pattern is "discord.com", match "*.discord.com"
        size_t hlen = strlen(hostname);
        size_t plen = strlen(pattern);
        if (hlen > plen && hostname[hlen - plen - 1] == '.') {
            if (strcasecmp(hostname + (hlen - plen), pattern) == 0) {
                return true;
            }
        }
    }
    return false;
}

bool parse_tls_sni(const uint8_t *payload, size_t payload_len, char *out_sni, size_t max_sni_len) {
    if (payload_len < 44) return false;

    // Byte 0: ContentType 0x16 (Handshake)
    if (payload[0] != 0x16) return false;

    // Bytes 1-2: TLS Version
    if (payload[1] != 0x03) return false;

    // Bytes 3-4: Length
    uint16_t record_len = (payload[3] << 8) | payload[4];
    if ((size_t)record_len + 5 > payload_len) {
        // Truncated or fragmented record
    }

    // Byte 5: HandshakeType 0x01 (ClientHello)
    if (payload[5] != 0x01) return false;

    size_t pos = 43; // 5 (record header) + 4 (handshake header) + 2 (client_version) + 32 (random)

    if (pos >= payload_len) return false;
    uint8_t session_id_len = payload[pos];
    pos += 1 + session_id_len;

    if (pos + 2 > payload_len) return false;
    uint16_t cipher_suites_len = (payload[pos] << 8) | payload[pos + 1];
    pos += 2 + cipher_suites_len;

    if (pos + 1 > payload_len) return false;
    uint8_t compression_len = payload[pos];
    pos += 1 + compression_len;

    if (pos + 2 > payload_len) return false;
    uint16_t extensions_len = (payload[pos] << 8) | payload[pos + 1];
    pos += 2;

    size_t ext_end = pos + extensions_len;
    if (ext_end > payload_len) ext_end = payload_len;

    while (pos + 4 <= ext_end) {
        uint16_t ext_type = (payload[pos] << 8) | payload[pos + 1];
        uint16_t ext_len = (payload[pos + 2] << 8) | payload[pos + 3];
        pos += 4;

        if (pos + ext_len > ext_end) break;

        if (ext_type == 0x0000) { // server_name extension
            if (ext_len < 5) break;
            // pos + 0..1: server_name_list_length
            // pos + 2: server_name_type (0 = host_name)
            if (payload[pos + 2] == 0x00) {
                uint16_t name_len = (payload[pos + 3] << 8) | payload[pos + 4];
                if (pos + 5 + name_len <= ext_end && name_len < max_sni_len) {
                    memcpy(out_sni, &payload[pos + 5], name_len);
                    out_sni[name_len] = '\0';
                    return true;
                }
            }
            break;
        }
        pos += ext_len;
    }

    return false;
}

uint16_t checksum(const void *buf, size_t len) {
    uint32_t sum = 0;
    const uint16_t *p = (const uint16_t *)buf;

    while (len > 1) {
        sum += *p++;
        len -= 2;
    }

    if (len == 1) {
        sum += *(const uint8_t *)p;
    }

    while (sum >> 16) {
        sum = (sum & 0xffff) + (sum >> 16);
    }

    return ~sum;
}

uint16_t tcp_checksum(const void *ip_hdr, const struct tcphdr *tcp, const uint8_t *payload, size_t payload_len, bool is_ipv6) {
    uint32_t sum = 0;
    uint16_t tcp_len = (tcp->doff * 4) + payload_len;

    if (!is_ipv6) {
        const struct iphdr *ip = (const struct iphdr *)ip_hdr;
        struct {
            uint32_t src;
            uint32_t dst;
            uint8_t zero;
            uint8_t protocol;
            uint16_t tcp_len;
        } pseudo = {
            .src = ip->saddr,
            .dst = ip->daddr,
            .zero = 0,
            .protocol = IPPROTO_TCP,
            .tcp_len = htons(tcp_len),
        };

        const uint16_t *p = (const uint16_t *)&pseudo;
        for (size_t i = 0; i < sizeof(pseudo) / 2; i++) {
            sum += p[i];
        }
    } else {
        const struct ip6_hdr *ip6 = (const struct ip6_hdr *)ip_hdr;
        struct {
            struct in6_addr src;
            struct in6_addr dst;
            uint32_t tcp_len;
            uint8_t zero[3];
            uint8_t next_hdr;
        } pseudo6 = {
            .src = ip6->ip6_src,
            .dst = ip6->ip6_dst,
            .tcp_len = htonl(tcp_len),
            .zero = {0, 0, 0},
            .next_hdr = IPPROTO_TCP,
        };

        const uint16_t *p = (const uint16_t *)&pseudo6;
        for (size_t i = 0; i < sizeof(pseudo6) / 2; i++) {
            sum += p[i];
        }
    }

    // Add TCP header with zeroed checksum
    struct tcphdr tcp_copy;
    memcpy(&tcp_copy, tcp, sizeof(struct tcphdr));
    tcp_copy.check = 0;

    const uint16_t *ptcp = (const uint16_t *)&tcp_copy;
    for (size_t i = 0; i < sizeof(struct tcphdr) / 2; i++) {
        sum += ptcp[i];
    }

    // Add TCP options if any
    size_t opt_len = (tcp->doff * 4) - sizeof(struct tcphdr);
    if (opt_len > 0) {
        const uint16_t *popt = (const uint16_t *)((const uint8_t *)tcp + sizeof(struct tcphdr));
        while (opt_len > 1) {
            sum += *popt++;
            opt_len -= 2;
        }
        if (opt_len == 1) {
            sum += *(const uint8_t *)popt;
        }
    }

    // Add payload
    const uint16_t *ppay = (const uint16_t *)payload;
    size_t plen = payload_len;
    while (plen > 1) {
        sum += *ppay++;
        plen -= 2;
    }
    if (plen == 1) {
        sum += *(const uint8_t *)ppay;
    }

    while (sum >> 16) {
        sum = (sum & 0xffff) + (sum >> 16);
    }

    return ~sum;
}

bool desync_tcp_packet(int raw_sock4, int raw_sock6, const uint8_t *pkt_data, size_t pkt_len, const DesyncConfig *cfg) {
    if (pkt_len < 40) return false;

    bool is_ipv6 = false;
    size_t ip_hl = 0;

    if ((pkt_data[0] >> 4) == 4) {
        const struct iphdr *ip = (const struct iphdr *)pkt_data;
        if (ip->protocol != IPPROTO_TCP) return false;
        ip_hl = ip->ihl * 4;
    } else if ((pkt_data[0] >> 4) == 6) {
        const struct ip6_hdr *ip6 = (const struct ip6_hdr *)pkt_data;
        if (ip6->ip6_nxt != IPPROTO_TCP) return false;
        ip_hl = 40;
        is_ipv6 = true;
    } else {
        return false;
    }

    if (pkt_len < ip_hl + sizeof(struct tcphdr)) return false;

    const struct tcphdr *tcp = (const struct tcphdr *)(pkt_data + ip_hl);
    size_t tcp_hl = tcp->doff * 4;

    if (pkt_len < ip_hl + tcp_hl) return false;

    const uint8_t *payload = pkt_data + ip_hl + tcp_hl;
    size_t payload_len = pkt_len - ip_hl - tcp_hl;

    if (payload_len <= (size_t)cfg->split_pos) return false;

    // Check SNI matching if hostlist is configured
    char sni[MAX_HOST_LEN] = {0};
    if (parse_tls_sni(payload, payload_len, sni, sizeof(sni))) {
        if (cfg->has_hostlist && !hostlist_matches(&cfg->hostlist, sni)) {
            // Not in targeted domains, let kernel handle normally
            return false;
        }
    } else {
        // Not a TLS ClientHello
        return false;
    }

    int raw_sock = is_ipv6 ? raw_sock6 : raw_sock4;
    if (raw_sock < 0) return false;

    struct sockaddr_storage dst_addr;
    memset(&dst_addr, 0, sizeof(dst_addr));

    if (!is_ipv6) {
        const struct iphdr *ip = (const struct iphdr *)pkt_data;
        struct sockaddr_in *sin = (struct sockaddr_in *)&dst_addr;
        sin->sin_family = AF_INET;
        sin->sin_addr.s_addr = ip->daddr;
        sin->sin_port = tcp->dest;
    } else {
        const struct ip6_hdr *ip6 = (const struct ip6_hdr *)pkt_data;
        struct sockaddr_in6 *sin6 = (struct sockaddr_in6 *)&dst_addr;
        sin6->sin6_family = AF_INET6;
        sin6->sin6_addr = ip6->ip6_dst;
        sin6->sin6_port = tcp->dest;
    }

    // Optional FAKE packet injection (Strategy D)
    if (cfg->mode == DESYNC_FAKE) {
        uint8_t fake_buf[256];
        size_t fake_payload_len = 16;
        size_t fake_pkt_len = ip_hl + tcp_hl + fake_payload_len;

        memcpy(fake_buf, pkt_data, ip_hl + tcp_hl);
        memset(fake_buf + ip_hl + tcp_hl, 0x00, fake_payload_len); // zero/fake payload

        if (!is_ipv6) {
            struct iphdr *fake_ip = (struct iphdr *)fake_buf;
            fake_ip->tot_len = htons(fake_pkt_len);
            fake_ip->ttl = (uint8_t)cfg->fake_ttl; // low TTL
            fake_ip->check = 0;
            fake_ip->check = checksum(fake_ip, ip_hl);

            struct tcphdr *fake_tcp = (struct tcphdr *)(fake_buf + ip_hl);
            fake_tcp->check = tcp_checksum(fake_ip, fake_tcp, fake_buf + ip_hl + tcp_hl, fake_payload_len, false);
        }

        sendto(raw_sock, fake_buf, fake_pkt_len, 0, (struct sockaddr *)&dst_addr,
               is_ipv6 ? sizeof(struct sockaddr_in6) : sizeof(struct sockaddr_in));
    }

    // SPLIT2 TCP SEGMENTATION:
    // Packet 1: First split_pos bytes (e.g. 2 bytes)
    size_t p1_payload_len = cfg->split_pos;
    size_t p1_len = ip_hl + tcp_hl + p1_payload_len;
    uint8_t *pkt1 = (uint8_t *)malloc(p1_len);
    if (!pkt1) return false;

    memcpy(pkt1, pkt_data, ip_hl + tcp_hl);
    memcpy(pkt1 + ip_hl + tcp_hl, payload, p1_payload_len);

    if (!is_ipv6) {
        struct iphdr *ip1 = (struct iphdr *)pkt1;
        ip1->tot_len = htons(p1_len);
        ip1->check = 0;
        ip1->check = checksum(ip1, ip_hl);

        struct tcphdr *tcp1 = (struct tcphdr *)(pkt1 + ip_hl);
        tcp1->check = tcp_checksum(ip1, tcp1, pkt1 + ip_hl + tcp_hl, p1_payload_len, false);
    } else {
        struct ip6_hdr *ip6_1 = (struct ip6_hdr *)pkt1;
        ip6_1->ip6_plen = htons(tcp_hl + p1_payload_len);

        struct tcphdr *tcp1 = (struct tcphdr *)(pkt1 + ip_hl);
        tcp1->check = tcp_checksum(ip6_1, tcp1, pkt1 + ip_hl + tcp_hl, p1_payload_len, true);
    }

    sendto(raw_sock, pkt1, p1_len, 0, (struct sockaddr *)&dst_addr,
           is_ipv6 ? sizeof(struct sockaddr_in6) : sizeof(struct sockaddr_in));
    free(pkt1);

    usleep(1000); // 1 millisecond delay

    // Packet 2: Remaining payload
    size_t p2_payload_len = payload_len - p1_payload_len;
    size_t p2_len = ip_hl + tcp_hl + p2_payload_len;
    uint8_t *pkt2 = (uint8_t *)malloc(p2_len);
    if (!pkt2) return false;

    memcpy(pkt2, pkt_data, ip_hl + tcp_hl);
    memcpy(pkt2 + ip_hl + tcp_hl, payload + p1_payload_len, p2_payload_len);

    struct tcphdr *tcp2 = (struct tcphdr *)(pkt2 + ip_hl);
    tcp2->seq = htonl(ntohl(tcp->seq) + p1_payload_len);

    if (!is_ipv6) {
        struct iphdr *ip2 = (struct iphdr *)pkt2;
        ip2->tot_len = htons(p2_len);
        ip2->check = 0;
        ip2->check = checksum(ip2, ip_hl);

        tcp2->check = tcp_checksum(ip2, tcp2, pkt2 + ip_hl + tcp_hl, p2_payload_len, false);
    } else {
        struct ip6_hdr *ip6_2 = (struct ip6_hdr *)pkt2;
        ip6_2->ip6_plen = htons(tcp_hl + p2_payload_len);

        tcp2->check = tcp_checksum(ip6_2, tcp2, pkt2 + ip_hl + tcp_hl, p2_payload_len, true);
    }

    sendto(raw_sock, pkt2, p2_len, 0, (struct sockaddr *)&dst_addr,
           is_ipv6 ? sizeof(struct sockaddr_in6) : sizeof(struct sockaddr_in));
    free(pkt2);

    return true; // Packets injected successfully, caller drops original
}
