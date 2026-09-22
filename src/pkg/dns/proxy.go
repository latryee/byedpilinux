package dns
import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"discord-bypass/src/pkg/utils"
)

// DNSProxy is a domain-routing local DNS proxy.
// Queries for Discord domains are resolved over secure DoH (Cloudflare 1.1.1.1).
// Queries for all other domains (e.g. google.com, local network) are transparently
// forwarded to the upstream router/ISP DNS server (e.g. 192.168.1.1:53).
type DNSProxy struct {
	listenAddr    string
	upstreamDNS   string
	dohURL        string
	preferIPv4    bool
	hasGlobalIPv6 bool
	domains       []string
	udpConn       *net.UDPConn
	tcpLn         net.Listener
	httpClient    *http.Client
	mu            sync.Mutex
	isRunning     bool
	cancel        context.CancelFunc
}

func NewDNSProxy(listenAddr string, dohProvider string, preferIPv4 bool, upstreamDNS string, domains []string) *DNSProxy {
	if listenAddr == "" {
		listenAddr = "127.0.0.1:5354"
	}
	url, ok := ProviderURLs[strings.ToLower(dohProvider)]
	if !ok {
		url = ProviderURLs["cloudflare"]
	}

	if upstreamDNS == "" {
		upstreamDNS = detectUpstreamDNS()
	}
	// Ensure port 53 if not specified
	if !strings.Contains(upstreamDNS, ":") {
		upstreamDNS = net.JoinHostPort(upstreamDNS, "53")
	}

	// Normalize target domains for fast matching
	var normDomains []string
	for _, d := range domains {
		trimmed := strings.ToLower(strings.Trim(strings.TrimSpace(d), "."))
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			normDomains = append(normDomains, trimmed)
		}
	}

	hasGlobalIPv6 := checkGlobalIPv6Route()

	return &DNSProxy{
		listenAddr:    listenAddr,
		upstreamDNS:   upstreamDNS,
		dohURL:        url,
		preferIPv4:    preferIPv4,
		hasGlobalIPv6: hasGlobalIPv6,
		domains:       normDomains,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				IdleConnTimeout:     90 * time.Second,
				DisableCompression: true,
			},
		},
	}
}

func (p *DNSProxy) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.isRunning {
		p.mu.Unlock()
		return nil
	}

	uAddr, err := net.ResolveUDPAddr("udp", p.listenAddr)
	if err != nil {
		p.mu.Unlock()
		return err
	}

	uConn, err := net.ListenUDP("udp", uAddr)
	if err != nil {
		p.mu.Unlock()
		return fmt.Errorf("failed to bind local DNS proxy UDP on %s: %w", p.listenAddr, err)
	}

	tLn, err := net.Listen("tcp", p.listenAddr)
	if err != nil {
		_ = uConn.Close()
		p.mu.Unlock()
		return fmt.Errorf("failed to bind local DNS proxy TCP on %s: %w", p.listenAddr, err)
	}

	subCtx, cancel := context.WithCancel(ctx)
	p.udpConn = uConn
	p.tcpLn = tLn
	p.cancel = cancel
	p.isRunning = true
	p.mu.Unlock()

	utils.Info("Local Domain-Routing DNS Proxy listening on %s (Discord -> %s, Normal -> %s)",
		p.listenAddr, p.dohURL, p.upstreamDNS)

	go p.udpLoop(subCtx)
	go p.tcpLoop(subCtx)
	return nil
}

func (p *DNSProxy) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.isRunning {
		return
	}
	p.isRunning = false
	if p.cancel != nil {
		p.cancel()
	}
	if p.udpConn != nil {
		_ = p.udpConn.Close()
	}
	if p.tcpLn != nil {
		_ = p.tcpLn.Close()
	}
	utils.Info("Local Domain-Routing DNS Proxy stopped")
}

func (p *DNSProxy) UpstreamDNS() string {
	return p.upstreamDNS
}

func (p *DNSProxy) SetUpstreamDNS(addr string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !strings.Contains(addr, ":") {
		addr = net.JoinHostPort(addr, "53")
	}
	p.upstreamDNS = addr
}

func (p *DNSProxy) IsDiscordDomain(domain string) bool {
	norm := strings.ToLower(strings.Trim(domain, "."))
	for _, target := range p.domains {
		if norm == target || strings.HasSuffix(norm, "."+target) {
			return true
		}
	}
	return false
}

func (p *DNSProxy) udpLoop(ctx context.Context) {
	buf := make([]byte, 4096)

	for {
		n, remoteAddr, err := p.udpConn.ReadFrom(buf)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				return
			}
		}

		queryData := make([]byte, n)
		copy(queryData, buf[:n])

		go func(q []byte, rAddr net.Addr) {
			resp := p.resolveQuery(ctx, q, false)
			if len(resp) > 0 {
				_, _ = p.udpConn.WriteTo(resp, rAddr)
			}
		}(queryData, remoteAddr)
	}
}

func (p *DNSProxy) tcpLoop(ctx context.Context) {
	for {
		conn, err := p.tcpLn.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				return
			}
		}

		go func(c net.Conn) {
			defer c.Close()
			_ = c.SetDeadline(time.Now().Add(5 * time.Second))

			lenBuf := make([]byte, 2)
			if _, err := io.ReadFull(c, lenBuf); err != nil {
				return
			}
			msgLen := int(binary.BigEndian.Uint16(lenBuf))
			if msgLen <= 0 || msgLen > 4096 {
				return
			}

			queryData := make([]byte, msgLen)
			if _, err := io.ReadFull(c, queryData); err != nil {
				return
			}

			resp := p.resolveQuery(ctx, queryData, true)
			if len(resp) > 0 {
				respLenBuf := make([]byte, 2)
				binary.BigEndian.PutUint16(respLenBuf, uint16(len(resp)))
				_, _ = c.Write(respLenBuf)
				_, _ = c.Write(resp)
			}
		}(conn)
	}
}

// resolveQuery inspects the query, determines whether it targets a Discord domain,
// and routes it either through DoH or forwards to the local upstream DNS.
func (p *DNSProxy) resolveQuery(ctx context.Context, queryData []byte, isTCP bool) []byte {
	qname, qtype, err := ParseDNSQuestion(queryData)
	if err != nil {
		// Cannot parse question; forward to upstream as safe fallback
		return p.forwardUpstream(ctx, queryData, isTCP)
	}

	if p.IsDiscordDomain(qname) {
		// If client is asking for IPv6 (AAAA) but preferIPv4 is set and host has no global IPv6 routing,
		// synthesize an empty NODATA response to avoid timeouts against blackholed IPv6 routes.
		if qtype == TypeAAAA && p.preferIPv4 && !p.hasGlobalIPv6 {
			utils.Debug("[DNS Proxy] Pruning AAAA for %s (IPv4 preferred)", qname)
			return makeEmptyResponse(queryData)
		}

		utils.Debug("[DNS Proxy] Resolving Discord domain %s (type %d) via Secure DoH (%s)", qname, qtype, p.dohURL)
		resp, err := p.forwardDoH(ctx, queryData)
		if err == nil && len(resp) > 0 {
			return resp
		}
		utils.Warn("[DNS Proxy] DoH resolution failed for %s: %v; falling back to upstream", qname, err)
	}

	// Normal domain (e.g. google.com) or DoH fallback: forward to router/ISP DNS
	return p.forwardUpstream(ctx, queryData, isTCP)
}

func (p *DNSProxy) forwardDoH(ctx context.Context, queryData []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", p.dohURL, bytes.NewReader(queryData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/dns-message")
	req.Header.Set("Accept", "application/dns-message")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DoH HTTP status: %d", resp.StatusCode)
	}

	return io.ReadAll(io.LimitReader(resp.Body, 65535))
}

func (p *DNSProxy) forwardUpstream(ctx context.Context, queryData []byte, isTCP bool) []byte {
	upstream := p.upstreamDNS
	if isTCP {
		d := net.Dialer{Timeout: 3 * time.Second}
		conn, err := d.DialContext(ctx, "tcp", upstream)
		if err != nil {
			utils.Debug("[DNS Proxy] TCP upstream forward failed to %s: %v", upstream, err)
			return nil
		}
		defer conn.Close()

		lenBuf := make([]byte, 2)
		binary.BigEndian.PutUint16(lenBuf, uint16(len(queryData)))
		if _, err := conn.Write(append(lenBuf, queryData...)); err != nil {
			return nil
		}

		respLenBuf := make([]byte, 2)
		if _, err := io.ReadFull(conn, respLenBuf); err != nil {
			return nil
		}
		respLen := int(binary.BigEndian.Uint16(respLenBuf))
		respData := make([]byte, respLen)
		if _, err := io.ReadFull(conn, respData); err != nil {
			return nil
		}
		return respData
	}

	// UDP forward
	uAddr, err := net.ResolveUDPAddr("udp", upstream)
	if err != nil {
		return nil
	}
	conn, err := net.DialUDP("udp", nil, uAddr)
	if err != nil {
		return nil
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Write(queryData); err != nil {
		return nil
	}

	respBuf := make([]byte, 4096)
	n, err := conn.Read(respBuf)
	if err != nil {
		return nil
	}
	return respBuf[:n]
}

// ParseDNSQuestion extracts the domain name and query type from an RFC 1035 question section.
func ParseDNSQuestion(data []byte) (string, uint16, error) {
	if len(data) < 12 {
		return "", 0, errors.New("DNS packet too short")
	}

	qdCount := binary.BigEndian.Uint16(data[4:6])
	if qdCount == 0 {
		return "", 0, errors.New("no question section in DNS packet")
	}

	offset := 12
	var labels []string

	for {
		if offset >= len(data) {
			return "", 0, errors.New("malformed question QNAME: unexpected end of packet")
		}
		length := int(data[offset])
		if length == 0 {
			offset++
			break
		}
		if length&0xC0 == 0xC0 {
			// Pointer in question
			offset += 2
			break
		}
		offset++
		if offset+length > len(data) {
			return "", 0, errors.New("label length out of bounds")
		}
		labels = append(labels, string(data[offset:offset+length]))
		offset += length
	}

	if offset+4 > len(data) {
		return "", 0, errors.New("packet truncated before QTYPE/QCLASS")
	}

	qtype := binary.BigEndian.Uint16(data[offset : offset+2])
	domain := strings.ToLower(strings.Join(labels, "."))
	return domain, qtype, nil
}

// makeEmptyResponse builds a standard DNS NOERROR answer with 0 records (NODATA).
func makeEmptyResponse(query []byte) []byte {
	if len(query) < 12 {
		return nil
	}
	// Copy transaction ID
	txID := binary.BigEndian.Uint16(query[0:2])

	// Find question end
	offset := 12
	for offset < len(query) {
		l := int(query[offset])
		if l == 0 {
			offset++
			break
		}
		if l&0xC0 == 0xC0 {
			offset += 2
			break
		}
		offset += 1 + l
	}
	offset += 4 // Include QTYPE and QCLASS
	if offset > len(query) {
		offset = len(query)
	}

	resp := make([]byte, offset)
	copy(resp, query[:offset])

	binary.BigEndian.PutUint16(resp[0:2], txID)
	// Flags: QR=1 (response), AA=0, TC=0, RD=1, RA=1, RCODE=0 -> 0x8180
	binary.BigEndian.PutUint16(resp[2:4], 0x8180)
	binary.BigEndian.PutUint16(resp[4:6], 1) // QDCOUNT = 1
	binary.BigEndian.PutUint16(resp[6:8], 0) // ANCOUNT = 0
	binary.BigEndian.PutUint16(resp[8:10], 0)
	binary.BigEndian.PutUint16(resp[10:12], 0)

	return resp
}

func detectUpstreamDNS() string {
	// Try resolvectl dns first
	if out, err := exec.Command("resolvectl", "dns").Output(); err == nil {
		lines := strings.Split(string(out), "\n")
		for _, l := range lines {
			l = strings.TrimSpace(l)
			if strings.HasPrefix(l, "Link") && strings.Contains(l, ":") {
				parts := strings.SplitN(l, ":", 2)
				if len(parts) == 2 {
					servers := strings.Fields(parts[1])
					for _, s := range servers {
						if ip := net.ParseIP(s); ip != nil && !ip.IsLoopback() {
							return s + ":53"
						}
					}
				}
			}
		}
	}

	// Try default gateway IP from /proc/net/route
	if gw := getDefaultGateway(); gw != "" {
		return gw + ":53"
	}

	return "1.1.1.1:53"
}

func getDefaultGateway() string {
	f, err := os.Open("/proc/net/route")
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 3 && fields[1] == "00000000" {
			// Gateway hex in little endian
			var gwHex uint32
			if _, err := fmt.Sscanf(fields[2], "%X", &gwHex); err == nil && gwHex != 0 {
				ip := net.IPv4(byte(gwHex), byte(gwHex>>8), byte(gwHex>>16), byte(gwHex>>24))
				return ip.String()
			}
		}
	}
	return ""
}

func checkGlobalIPv6Route() bool {
	d := net.Dialer{Timeout: 500 * time.Millisecond}
	conn, err := d.Dial("udp6", "[2606:4700:4700::1111]:53")
	if err == nil {
		conn.Close()
		return true
	}
	return false
}
