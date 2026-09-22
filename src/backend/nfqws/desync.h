#ifndef DESYNC_H
#define DESYNC_H

#include <stdint.h>
#include <stddef.h>
#include <stdbool.h>
#include <netinet/in.h>
#include <netinet/ip.h>
#include <netinet/ip6.h>
#include <netinet/tcp.h>
#include <netinet/udp.h>

#define MAX_HOSTS 1024
#define MAX_HOST_LEN 256

typedef struct {
    char hosts[MAX_HOSTS][MAX_HOST_LEN];
    size_t count;
} HostList;

typedef enum {
    DESYNC_NONE = 0,
    DESYNC_SPLIT2,
    DESYNC_FAKE,
    DESYNC_DISORDER2,
} DesyncMode;

typedef struct {
    int qnum;
    DesyncMode mode;
    int split_pos;
    int fake_ttl;
    char hostlist_file[512];
    bool has_hostlist;
    HostList hostlist;
} DesyncConfig;

// Hostlist functions
bool hostlist_load(HostList *hl, const char *filename);
bool hostlist_matches(const HostList *hl, const char *hostname);

// TLS inspection
bool parse_tls_sni(const uint8_t *payload, size_t payload_len, char *out_sni, size_t max_sni_len);

// Raw packet sending and checksums
uint16_t checksum(const void *buf, size_t len);
uint16_t tcp_checksum(const void *ip_hdr, const struct tcphdr *tcp, const uint8_t *payload, size_t payload_len, bool is_ipv6);

// Desync execution
bool desync_tcp_packet(int raw_sock4, int raw_sock6, const uint8_t *pkt_data, size_t pkt_len, const DesyncConfig *cfg);

#endif // DESYNC_H
