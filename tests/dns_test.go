package tests

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"

	"discord-bypass/src/pkg/dns"
)

func TestBuildDNSQuery(t *testing.T) {
	packet, err := dns.BuildDNSQuery("discord.com", dns.TypeA)
	if err != nil {
		t.Fatalf("BuildDNSQuery failed: %v", err)
	}

	if len(packet) < 12 {
		t.Fatalf("DNS query packet too short: %d bytes", len(packet))
	}

	// Flags should have RD=1 (0x0100)
	flags := binary.BigEndian.Uint16(packet[2:4])
	if flags != 0x0100 {
		t.Errorf("Expected flags 0x0100, got 0x%04x", flags)
	}

	// Question count should be 1
	qdCount := binary.BigEndian.Uint16(packet[4:6])
	if qdCount != 1 {
		t.Errorf("Expected QDCOUNT 1, got %d", qdCount)
	}

	// Check domain encoding: \x07discord\x03com\x00
	expectedQName := []byte("\x07discord\x03com\x00")
	if !bytes.Contains(packet, expectedQName) {
		t.Errorf("Packet does not contain expected encoded domain")
	}
}

func TestParseDNSResponse(t *testing.T) {
	// Synthesize a minimal DNS response packet for discord.com -> 162.159.135.232
	buf := new(bytes.Buffer)
	// Header (12 bytes)
	_ = binary.Write(buf, binary.BigEndian, uint16(0x1234)) // ID
	_ = binary.Write(buf, binary.BigEndian, uint16(0x8180)) // Flags: Response, RD, RA
	_ = binary.Write(buf, binary.BigEndian, uint16(1))      // QDCOUNT: 1
	_ = binary.Write(buf, binary.BigEndian, uint16(1))      // ANCOUNT: 1
	_ = binary.Write(buf, binary.BigEndian, uint16(0))      // NSCOUNT
	_ = binary.Write(buf, binary.BigEndian, uint16(0))      // ARCOUNT

	// Question section: \x07discord\x03com\x00, QTYPE=1, QCLASS=1
	buf.Write([]byte("\x07discord\x03com\x00"))
	_ = binary.Write(buf, binary.BigEndian, uint16(1))
	_ = binary.Write(buf, binary.BigEndian, uint16(1))

	// Answer section: Pointer to QNAME (0xc00c), TYPE=1, CLASS=1, TTL=300, RDLENGTH=4, RDATA=162.159.135.232
	_ = binary.Write(buf, binary.BigEndian, uint16(0xc00c))
	_ = binary.Write(buf, binary.BigEndian, uint16(1))   // Type A
	_ = binary.Write(buf, binary.BigEndian, uint16(1))   // Class IN
	_ = binary.Write(buf, binary.BigEndian, uint32(300)) // TTL
	_ = binary.Write(buf, binary.BigEndian, uint16(4))   // RDLENGTH
	buf.Write([]byte{162, 159, 135, 232})                // RDATA

	ips, err := dns.ParseDNSResponse(buf.Bytes())
	if err != nil {
		t.Fatalf("ParseDNSResponse failed: %v", err)
	}

	if len(ips) != 1 {
		t.Fatalf("Expected 1 IP, got %d", len(ips))
	}

	expectedIP := net.IPv4(162, 159, 135, 232)
	if !ips[0].Equal(expectedIP) {
		t.Errorf("Expected IP %s, got %s", expectedIP, ips[0])
	}
}

func TestPoisoningPatternDetection(t *testing.T) {
	poisonedIPs := []net.IP{
		net.IPv4zero,              // 0.0.0.0
		net.IPv4(127, 0, 0, 1),    // 127.0.0.1
		net.IPv4(192, 168, 1, 1),  // Private RFC1918 captive portal
	}

	for _, ip := range poisonedIPs {
		isPoison := ip.IsLoopback() || ip.IsUnspecified() || ip.Equal(net.IPv4zero) || ip.IsPrivate()
		if !isPoison {
			t.Errorf("Failed to detect poisoned IP: %s", ip)
		}
	}
}

func TestSinkholeDetection(t *testing.T) {
	sinkholes := []string{
		"195.175.254.2",
		"2a01:358:4014:a00::3",
		"195.175.254.1",
		"0.0.0.0",
		"127.0.0.1",
		"::1",
		"10.0.0.1",
	}

	for _, s := range sinkholes {
		ip := net.ParseIP(s)
		if ip == nil {
			t.Fatalf("Failed to parse test IP %s", s)
		}
		if !dns.IsSinkholeIP(ip) {
			t.Errorf("Expected IsSinkholeIP(%s) to be true, got false", s)
		}
	}

	cleanIPs := []string{
		"162.159.135.232",
		"104.16.248.249",
		"1.1.1.1",
		"8.8.8.8",
		"2606:4700:4700::1111",
	}

	for _, c := range cleanIPs {
		ip := net.ParseIP(c)
		if ip == nil {
			t.Fatalf("Failed to parse test IP %s", c)
		}
		if dns.IsSinkholeIP(ip) {
			t.Errorf("Expected IsSinkholeIP(%s) to be false, got true", c)
		}
	}
}
