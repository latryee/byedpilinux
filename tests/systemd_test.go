package tests

import (
	"bytes"
	"context"
	"encoding/binary"
	"net"
	"strings"
	"testing"
	"time"

	"discord-bypass/src/pkg/dns"
)

func TestParseDNSQuestion(t *testing.T) {
	query, err := dns.BuildDNSQuery("discord.com", dns.TypeA)
	if err != nil {
		t.Fatalf("BuildDNSQuery failed: %v", err)
	}

	qname, qtype, err := dns.ParseDNSQuestion(query)
	if err != nil {
		t.Fatalf("ParseDNSQuestion failed: %v", err)
	}

	if qname != "discord.com" {
		t.Errorf("Expected qname 'discord.com', got '%s'", qname)
	}
	if qtype != dns.TypeA {
		t.Errorf("Expected qtype %d, got %d", dns.TypeA, qtype)
	}

	// Test malformed/short packet
	_, _, err = dns.ParseDNSQuestion([]byte{0x01, 0x02})
	if err == nil {
		t.Errorf("Expected error on truncated packet, got nil")
	}
}

func TestDNSProxyDomainMatching(t *testing.T) {
	targetDomains := []string{
		"discord.com",
		"gateway.discord.gg",
		"discordapp.com",
		"discordapp.net",
		"discord.media",
	}

	proxy := dns.NewDNSProxy("127.0.0.1:0", "cloudflare", true, "127.0.0.1:53", targetDomains)

	tests := []struct {
		domain   string
		expected bool
	}{
		{"discord.com", true},
		{"DISCORD.COM", true},
		{"gateway.discord.gg", true},
		{"sub.discord.com", true},
		{"cdn.discordapp.com", true},
		{"voice.discord.media", true},
		{"google.com", false},
		{"fake-discord.com.attacker.com", false},
		{"notdiscord.com", false},
		{"zorin.com", false},
		{"router.home", false},
	}

	for _, tc := range tests {
		got := proxy.IsDiscordDomain(tc.domain)
		if got != tc.expected {
			t.Errorf("IsDiscordDomain(%q) = %v; expected %v", tc.domain, got, tc.expected)
		}
	}
}

func TestDNSProxyRoutingIntegration(t *testing.T) {
	// 1. Set up a mock upstream DNS server on localhost (simulating 192.168.1.1 router DNS)
	upstreamConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		if strings.Contains(err.Error(), "operation not permitted") {
			t.Skip("local UDP sockets are unavailable in this test environment")
		}
		t.Fatalf("Failed to start mock upstream DNS: %v", err)
	}
	defer upstreamConn.Close()
	upstreamAddr := upstreamConn.LocalAddr().String()

	// Mock upstream server handler: returns 142.251.38.238 for google.com
	go func() {
		buf := make([]byte, 1024)
		for {
			n, rAddr, err := upstreamConn.ReadFrom(buf)
			if err != nil {
				return
			}
			req := buf[:n]
			txID := binary.BigEndian.Uint16(req[0:2])

			// Create response with answer 142.251.38.238
			var resp bytes.Buffer
			_ = binary.Write(&resp, binary.BigEndian, txID)
			_ = binary.Write(&resp, binary.BigEndian, uint16(0x8180)) // Response, NoError
			_ = binary.Write(&resp, binary.BigEndian, uint16(1))      // QDCOUNT
			_ = binary.Write(&resp, binary.BigEndian, uint16(1))      // ANCOUNT
			_ = binary.Write(&resp, binary.BigEndian, uint16(0))
			_ = binary.Write(&resp, binary.BigEndian, uint16(0))
			// Echo question
			resp.Write(req[12:])
			// Answer: pointer to question, Type A, Class IN, TTL 300, len 4, 142.251.38.238
			_ = binary.Write(&resp, binary.BigEndian, uint16(0xc00c))
			_ = binary.Write(&resp, binary.BigEndian, uint16(1))
			_ = binary.Write(&resp, binary.BigEndian, uint16(1))
			_ = binary.Write(&resp, binary.BigEndian, uint32(300))
			_ = binary.Write(&resp, binary.BigEndian, uint16(4))
			resp.Write([]byte{142, 251, 38, 238})

			_, _ = upstreamConn.WriteTo(resp.Bytes(), rAddr)
		}
	}()

	// 2. Start SmartDNSProxy pointing upstream to our mock server
	targetDomains := []string{"discord.com", "gateway.discord.gg"}
	// Listen on an ephemeral local port
	proxy := dns.NewDNSProxy("127.0.0.1:15354", "cloudflare", true, upstreamAddr, targetDomains)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := proxy.Start(ctx); err != nil {
		t.Fatalf("Failed to start proxy: %v", err)
	}
	defer proxy.Stop()

	// Wait for listener to initialize
	time.Sleep(50 * time.Millisecond)

	// 3. Query normal domain (google.com) through the proxy
	// Must be forwarded to upstream mock server and return 142.251.38.238
	googleQuery, _ := dns.BuildDNSQuery("google.com", dns.TypeA)
	proxyAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:15354")
	clientConn, err := net.DialUDP("udp", nil, proxyAddr)
	if err != nil {
		t.Fatalf("Dial proxy failed: %v", err)
	}
	defer clientConn.Close()

	_ = clientConn.SetDeadline(time.Now().Add(3 * time.Second))
	_, err = clientConn.Write(googleQuery)
	if err != nil {
		t.Fatalf("Failed to send google query: %v", err)
	}

	googleResp := make([]byte, 1024)
	gn, _, err := clientConn.ReadFrom(googleResp)
	if err != nil {
		t.Fatalf("Failed to read google response from proxy: %v", err)
	}

	googleIPs, err := dns.ParseDNSResponse(googleResp[:gn])
	if err != nil {
		t.Fatalf("Failed to parse google response: %v", err)
	}
	if len(googleIPs) == 0 || !googleIPs[0].Equal(net.IPv4(142, 251, 38, 238)) {
		t.Errorf("Expected google.com to resolve via upstream to 142.251.38.238, got: %v", googleIPs)
	}

	// 4. Query Discord domain (discord.com) through the proxy
	// Must be forwarded via DoH and return real Discord/Cloudflare IPs (NOT sinkhole)
	discordQuery, _ := dns.BuildDNSQuery("discord.com", dns.TypeA)
	_, err = clientConn.Write(discordQuery)
	if err != nil {
		t.Fatalf("Failed to send discord query: %v", err)
	}

	discordResp := make([]byte, 2048)
	dn, _, err := clientConn.ReadFrom(discordResp)
	if err != nil {
		t.Fatalf("Failed to read discord response from proxy: %v", err)
	}

	discordIPs, err := dns.ParseDNSResponse(discordResp[:dn])
	if err != nil {
		t.Fatalf("Failed to parse discord response: %v", err)
	}
	if len(discordIPs) == 0 {
		t.Fatalf("Expected real IPs for discord.com, got 0 answers")
	}
	for _, ip := range discordIPs {
		if dns.IsSinkholeIP(ip) {
			t.Errorf("Discord domain resolved to sinkhole IP: %s!", ip)
		}
	}
	t.Logf("discord.com cleanly resolved through proxy DoH: %v", discordIPs)
}

func TestGetInterfaceDNSHelper(t *testing.T) {
	// Should return empty string or non-panic on non-existent interface
	dnsServer := dns.GetInterfaceDNS("nonexistent_iface_999")
	if dnsServer != "" {
		t.Errorf("Expected empty string for nonexistent interface, got %s", dnsServer)
	}
}
