# DPI Circumvention Techniques: Engineering Reference

Deep Packet Inspection (DPI) refers to network equipment (middleboxes) that inspect not only Layer 3 and Layer 4 headers (IP and port numbers) but also the Layer 7 application payload.

This document details each technique implemented in `discord-bypass`, the middlebox behavior it defeats, and its trade-offs.

---

## 1. TCP Segmentation (`split2`) - Primary Technique

### The Filtering Mechanism
When a client establishes an HTTPS connection to `discord.com`:
1. The TCP 3-way handshake succeeds.
2. The client transmits the **TLS ClientHello** packet.
3. The DPI middlebox parses:
   - Byte 0: `0x16` (Handshake)
   - Bytes 1-2: `0x03 0x01` or `0x03 0x03` (TLS record version)
   - Bytes 3-4: Record length
   - Extensions -> Server Name Indication (SNI): `discord.com`
4. The middlebox identifies `discord.com` and generates a forged TCP packet with the `RST` (Reset) flag set, sent to both the client and the server.

### The Bypass Mechanism
`discord-bypass` splits the single TLS ClientHello TCP segment into two distinct IP packets:
- **Segment 1:** Length = 2 bytes (`0x16 0x03`).
- **Segment 2:** Length = remaining bytes (from byte 2 to the end of the ClientHello, including the SNI).

### Why It Works
To inspect network traffic at line rate (hundreds of gigabits per second), DPI middleboxes inspect packets in isolation. Stateful TCP reassembly across multi-packet windows requires maintaining flow buffers in high-speed RAM for millions of concurrent flows—an operation too costly for most ISP middlebox deployments.
- When Segment 1 arrives, the DPI box sees only `0x16 0x03` without a complete TLS record header or SNI extension.
- When Segment 2 arrives, it does not begin with `0x16` (it begins at byte 2 of the TLS record header). The middlebox fails to parse the protocol and lets it pass.
- The destination Discord edge server (Cloudflare / Discord infrastructure) implements the standard TCP stack (RFC 793). It reassembles the segments in kernel memory and establishes the TLS session normally.

### Trade-Offs & Side Effects
- **Pros:** Does not rely on TTL estimation; immune to routing changes; works across all major Turkish ISPs.
- **Cons:** Slightly increases packet count for the initial handshake (one extra 54-byte TCP packet).

---

## 2. Fake Packet Desynchronization (`fake,split2`) - Strategy D

### The Filtering Mechanism
Some aggressive or stateful DPI middleboxes maintain a small buffer for out-of-order or segmented packets (typically up to 1-2 packets per flow).

### The Bypass Mechanism
1. The engine transmits a **Fake ClientHello** packet containing an unblocked SNI (or random bytes), configured with a low Time-To-Live (TTL) value (e.g. `TTL = 4`).
2. The fake packet reaches the ISP's DPI middlebox (which is typically 2-6 hops away within the ISP's internal network). The middlebox parses the fake SNI, marks the TCP flow as benign, and stops inspecting subsequent packets on this 4-tuple.
3. Because the fake packet has a low TTL, it expires (drops to TTL=0) at an intermediate router before reaching the destination Discord server.
4. The engine then transmits the real ClientHello (using `split2`), which reaches Discord and completes the connection.

### Trade-Offs & Side Effects
- **Pros:** Defeats stateful middleboxes that maintain reassembly buffers.
- **Cons:** Requires a properly tuned TTL. If the TTL is too low, the fake packet never reaches the DPI box; if the TTL is too high, the fake packet reaches the destination server and triggers a TLS error (`bad_record_mac` or connection reset).

---

## 3. Out-of-Order Delivery (`disorder2`)

### The Filtering Mechanism
DPI inspection engines typically process packets in the chronological order of packet arrival.

### The Bypass Mechanism
1. Segment 2 of the ClientHello is transmitted first.
2. Segment 1 of the ClientHello is transmitted second (after a sub-millisecond delay).

### Why It Works
Simple middleboxes expect sequential byte streams. If the middlebox encounters data starting at sequence number `seq + 2` without having seen bytes `0..2`, it cannot parse the protocol header. The receiving server's TCP stack buffers the out-of-order segment until the missing first segment arrives, then delivers the complete stream to the application.

---

## 4. DNS over HTTPS (DoH) vs Port 53 Hijacking

### The Filtering Mechanism
Standard DNS queries use UDP port 53 in plaintext. ISPs deploy two forms of DNS interference:
1. **DNS Poisoning:** The ISP's recursive DNS server answers requests for `discord.com` with `0.0.0.0`, `127.0.0.1`, or an ISP block warning portal IP.
2. **Transparent DNS Interception:** Even if the user configures `8.8.8.8` or `1.1.1.1` in `/etc/resolv.conf`, the ISP's edge router intercepts all outbound UDP 53 packets and responds with forged DNS answers.

### The Bypass Mechanism
`discord-bypass` sends all DNS queries to upstream providers (Cloudflare, Google, Quad9) via **DNS over HTTPS (RFC 8484)** on TCP port 443. The query is encrypted inside an authenticated TLS tunnel, preventing middleboxes from intercepting or modifying the response.

---

## 5. WebRTC UDP Voice & QUIC Handling

### The Problem
- Modern browsers and Discord use **QUIC (HTTP/3)** over UDP port 443 where possible.
- Discord Voice and Video channels connect to voice gateways using **WebRTC (UDP ports 50000-65535 or UDP 443)**.
- If an ISP partially filters UDP 443 or tampers with the initial DTLS handshake, the Discord client can hang in the "RTC Connecting" or "Voice Connected (No Route)" state.

### The Solution
1. In `nftables`, outbound UDP 443 traffic to Discord IPs is dropped. This forces the client to automatically fall back to standard HTTP/2 over TCP 443 TLS, where `split2` segmentation operates cleanly.
2. WebRTC voice traffic on standard voice ports is allowed directly through, and `prefer_ipv4` ensures voice UDP packets are routed over IPv4 without stalling on broken IPv6 routes.
