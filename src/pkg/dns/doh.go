package dns

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	TypeA    uint16 = 1
	TypeAAAA uint16 = 28
	ClassIN  uint16 = 1
)

var ProviderURLs = map[string]string{
	"cloudflare": "https://1.1.1.1/dns-query",
	"google":     "https://dns.google/dns-query",
	"quad9":      "https://dns.quad9.net/dns-query",
}

// BuildDNSQuery constructs an RFC 1035 wire-format query packet
func BuildDNSQuery(domain string, qtype uint16) ([]byte, error) {
	buf := new(bytes.Buffer)

	// Transaction ID (random 2 bytes)
	var txID uint16
	b := make([]byte, 2)
	_, _ = rand.Read(b)
	txID = binary.BigEndian.Uint16(b)
	_ = binary.Write(buf, binary.BigEndian, txID)

	// Flags: Standard query, recursion desired (RD=1) -> 0x0100
	_ = binary.Write(buf, binary.BigEndian, uint16(0x0100))

	// QDCOUNT: 1 question
	_ = binary.Write(buf, binary.BigEndian, uint16(1))
	// ANCOUNT, NSCOUNT, ARCOUNT: 0
	_ = binary.Write(buf, binary.BigEndian, uint16(0))
	_ = binary.Write(buf, binary.BigEndian, uint16(0))
	_ = binary.Write(buf, binary.BigEndian, uint16(0))

	// QNAME: length-prefixed labels
	labels := strings.Split(strings.Trim(domain, "."), ".")
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 {
			return nil, fmt.Errorf("invalid DNS label: %s", label)
		}
		buf.WriteByte(byte(len(label)))
		buf.WriteString(label)
	}
	buf.WriteByte(0) // Null byte ends QNAME

	// QTYPE and QCLASS
	_ = binary.Write(buf, binary.BigEndian, qtype)
	_ = binary.Write(buf, binary.BigEndian, ClassIN)

	return buf.Bytes(), nil
}

// ParseDNSResponse parses an RFC 1035 wire-format response packet and extracts IP answers
func ParseDNSResponse(data []byte) ([]net.IP, error) {
	if len(data) < 12 {
		return nil, errors.New("DNS packet too short")
	}

	// Flags check
	flags := binary.BigEndian.Uint16(data[2:4])
	rcode := flags & 0x000F
	if rcode != 0 {
		return nil, fmt.Errorf("DNS error RCODE: %d", rcode)
	}

	qdCount := binary.BigEndian.Uint16(data[4:6])
	anCount := binary.BigEndian.Uint16(data[6:8])

	offset := 12

	// Skip Question section
	for i := 0; i < int(qdCount); i++ {
		for {
			if offset >= len(data) {
				return nil, errors.New("malformed DNS question section")
			}
			length := int(data[offset])
			if length == 0 {
				offset++
				break
			}
			if length&0xC0 == 0xC0 { // Pointer
				offset += 2
				break
			}
			offset += 1 + length
		}
		offset += 4 // Skip QTYPE and QCLASS
	}

	// Parse Answer section
	var ips []net.IP
	for i := 0; i < int(anCount); i++ {
		if offset >= len(data) {
			break
		}

		// Skip NAME (could be pointer or label)
		if data[offset]&0xC0 == 0xC0 {
			offset += 2
		} else {
			for {
				if offset >= len(data) {
					return ips, errors.New("malformed answer name")
				}
				length := int(data[offset])
				if length == 0 {
					offset++
					break
				}
				if length&0xC0 == 0xC0 {
					offset += 2
					break
				}
				offset += 1 + length
			}
		}

		if offset+10 > len(data) {
			break
		}

		rrType := binary.BigEndian.Uint16(data[offset : offset+2])
		// rrClass := binary.BigEndian.Uint16(data[offset+2 : offset+4])
		// ttl := binary.BigEndian.Uint32(data[offset+4 : offset+8])
		rdLength := binary.BigEndian.Uint16(data[offset+8 : offset+10])
		offset += 10

		if offset+int(rdLength) > len(data) {
			break
		}

		rdata := data[offset : offset+int(rdLength)]
		offset += int(rdLength)

		if rrType == TypeA && rdLength == 4 {
			ip := net.IPv4(rdata[0], rdata[1], rdata[2], rdata[3])
			ips = append(ips, ip)
		} else if rrType == TypeAAAA && rdLength == 16 {
			ip := make(net.IP, 16)
			copy(ip, rdata)
			ips = append(ips, ip)
		}
	}

	return ips, nil
}

// QueryDoH executes a DNS query over HTTPS using RFC 8484
func QueryDoH(ctx context.Context, domain string, qtype uint16, providerURL string) ([]net.IP, error) {
	queryPacket, err := BuildDNSQuery(domain, qtype)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", providerURL, bytes.NewReader(queryPacket))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/dns-message")
	req.Header.Set("Accept", "application/dns-message")

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("DoH request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DoH HTTP status: %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed reading DoH response: %w", err)
	}

	return ParseDNSResponse(respBody)
}

// ResolveBoth queries both IPv4 (A) and IPv6 (AAAA) records via DoH
func ResolveBoth(ctx context.Context, domain string, provider string) (v4 []net.IP, v6 []net.IP, err error) {
	url, ok := ProviderURLs[strings.ToLower(provider)]
	if !ok {
		if strings.HasPrefix(provider, "https://") {
			url = provider
		} else {
			url = ProviderURLs["cloudflare"]
		}
	}

	v4, err = QueryDoH(ctx, domain, TypeA, url)
	if err != nil {
		return nil, nil, fmt.Errorf("A record lookup failed: %w", err)
	}

	v6, _ = QueryDoH(ctx, domain, TypeAAAA, url)
	return v4, v6, nil
}

var KnownSinkholeIPs = []string{
	"195.175.254.2",
	"195.175.254.1",
	"2a01:358:4014:a00::3",
	"2a01:358:4014:a00::1",
	"2a01:358:4014:a00::2",
	"0.0.0.0",
}

// IsSinkholeIP checks if an IP matches known Turkish ISP/BTK sinkholes, loopback, or private ranges
func IsSinkholeIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.Equal(net.IPv4zero) || ip.IsLoopback() || ip.IsUnspecified() || ip.IsPrivate() {
		return true
	}
	ipStr := ip.String()
	for _, s := range KnownSinkholeIPs {
		if ipStr == s {
			return true
		}
	}
	return false
}

// DetectPoisoning checks if local system DNS resolution for a domain differs from trusted DoH,
// or if system DNS returns poisoned IPs like 0.0.0.0, 127.0.0.1, or Turkish ISP/BTK sinkholes.
func DetectPoisoning(domain string, provider string) (isPoisoned bool, sysIPs []net.IP, dohIPs []net.IP, details string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. System DNS resolution
	sysIPStrings, sysErr := net.LookupHost(domain)
	for _, s := range sysIPStrings {
		if ip := net.ParseIP(s); ip != nil {
			sysIPs = append(sysIPs, ip)
		}
	}

	// 2. DoH resolution
	dohV4, dohV6, dohErr := ResolveBoth(ctx, domain, provider)
	if dohErr != nil {
		return false, sysIPs, nil, "Failed to contact DoH provider for comparison", dohErr
	}
	dohIPs = append(dohIPs, dohV4...)
	dohIPs = append(dohIPs, dohV6...)

	// Check system error when DoH succeeded
	if sysErr != nil && len(dohIPs) > 0 {
		return true, sysIPs, dohIPs, fmt.Sprintf("System DNS failed (%v) while DoH returned %d valid IPs", sysErr, len(dohIPs)), nil
	}

	// Check for known poisoned address patterns and Turkish ISP/BTK sinkholes
	for _, ip := range sysIPs {
		if IsSinkholeIP(ip) {
			return true, sysIPs, dohIPs, fmt.Sprintf("BTK/ISP Court-Order Block Sinkhole IP detected: %s", ip), nil
		}
	}

	// Compare overlap
	if len(sysIPs) > 0 && len(dohIPs) > 0 {
		hasOverlap := false
		for _, s := range sysIPs {
			for _, d := range dohIPs {
				if s.Equal(d) {
					hasOverlap = true
					break
				}
			}
			if hasOverlap {
				break
			}
		}

		if !hasOverlap {
			return true, sysIPs, dohIPs, fmt.Sprintf("DNS Mismatch/Poisoning: System resolver returned %v (different from trusted DoH %v)", sysIPs, dohIPs), nil
		}
	}

	return false, sysIPs, dohIPs, "System DNS matches valid external resolution", nil
}
