package tests

import (
	"os/exec"
	"strings"
	"testing"

	"discord-bypass/src/pkg/firewall"
)

// TestCIDRSuperonlineCase verifies Requirement 7a:
// 162.159.128.0/20 + 162.159.135.0/24 => only /20
func TestCIDRSuperonlineCase(t *testing.T) {
	input := []string{
		"162.159.128.0/20",
		"162.159.135.0/24",
	}

	v4, v6, err := firewall.NormalizeCIDRs(input)
	if err != nil {
		t.Fatalf("NormalizeCIDRs failed: %v", err)
	}

	if len(v6) != 0 {
		t.Fatalf("Expected 0 IPv6 CIDRs, got %d", len(v6))
	}

	if len(v4) != 1 {
		t.Fatalf("Expected exactly 1 IPv4 CIDR, got %d: %v", len(v4), v4)
	}

	if v4[0] != "162.159.128.0/20" {
		t.Errorf("Expected 162.159.128.0/20, got %s", v4[0])
	}
}

// TestMultipleNestedCIDRs verifies Requirement 7b:
// Multiple nested CIDRs (/8, /16, /24, /32) => only /8
func TestMultipleNestedCIDRs(t *testing.T) {
	input := []string{
		"10.1.1.1/32",
		"10.1.1.0/24",
		"10.0.0.0/8",
		"10.1.0.0/16",
	}

	v4, _, err := firewall.NormalizeCIDRs(input)
	if err != nil {
		t.Fatalf("NormalizeCIDRs failed: %v", err)
	}

	if len(v4) != 1 {
		t.Fatalf("Expected 1 CIDR, got %d: %v", len(v4), v4)
	}

	if v4[0] != "10.0.0.0/8" {
		t.Errorf("Expected 10.0.0.0/8, got %s", v4[0])
	}
}

// TestAdjacentNonOverlappingCIDRs verifies Requirement 7c:
// Adjacent CIDRs that do not overlap are both preserved
func TestAdjacentNonOverlappingCIDRs(t *testing.T) {
	input := []string{
		"192.168.0.0/24",
		"192.168.1.0/24",
	}

	v4, _, err := firewall.NormalizeCIDRs(input)
	if err != nil {
		t.Fatalf("NormalizeCIDRs failed: %v", err)
	}

	if len(v4) != 2 {
		t.Fatalf("Expected 2 CIDRs, got %d: %v", len(v4), v4)
	}

	expected := map[string]bool{
		"192.168.0.0/24": true,
		"192.168.1.0/24": true,
	}

	for _, c := range v4 {
		if !expected[c] {
			t.Errorf("Unexpected CIDR in output: %s", c)
		}
	}
}

// TestDuplicateCIDRs verifies Requirement 7d:
// Duplicate CIDRs are deduplicated to a single entry
func TestDuplicateCIDRs(t *testing.T) {
	input := []string{
		"172.16.0.0/16",
		"172.16.0.0/16",
		"172.16.0.0/16",
	}

	v4, _, err := firewall.NormalizeCIDRs(input)
	if err != nil {
		t.Fatalf("NormalizeCIDRs failed: %v", err)
	}

	if len(v4) != 1 {
		t.Fatalf("Expected 1 CIDR, got %d: %v", len(v4), v4)
	}

	if v4[0] != "172.16.0.0/16" {
		t.Errorf("Expected 172.16.0.0/16, got %s", v4[0])
	}
}

// TestIPv6NestedPrefixes verifies Requirement 7e:
// IPv6 nested prefixes are pruned correctly
func TestIPv6NestedPrefixes(t *testing.T) {
	input := []string{
		"2606:4700::/32",
		"2606:4700:10::/48",
		"2606:4700:10::1/128",
		"2001:db8::/32",
	}

	_, v6, err := firewall.NormalizeCIDRs(input)
	if err != nil {
		t.Fatalf("NormalizeCIDRs failed: %v", err)
	}

	if len(v6) != 2 {
		t.Fatalf("Expected 2 IPv6 CIDRs, got %d: %v", len(v6), v6)
	}

	expected := map[string]bool{
		"2606:4700::/32": true,
		"2001:db8::/32":  true,
	}

	for _, c := range v6 {
		if !expected[c] {
			t.Errorf("Unexpected IPv6 CIDR: %s", c)
		}
	}
}

// TestEmptyInput verifies Requirement 7f:
// Empty input returns empty slices without error
func TestEmptyInput(t *testing.T) {
	v4, v6, err := firewall.NormalizeCIDRs([]string{})
	if err != nil {
		t.Fatalf("NormalizeCIDRs failed on empty input: %v", err)
	}

	if len(v4) != 0 || len(v6) != 0 {
		t.Fatalf("Expected empty output, got v4=%v, v6=%v", v4, v6)
	}
}

// TestDefaultDiscordCIDRsNormalization verifies the actual project Discord CIDR list
func TestDefaultDiscordCIDRsNormalization(t *testing.T) {
	v4, v6, err := firewall.NormalizeCIDRs(firewall.DefaultDiscordIPv4CIDRs)
	if err != nil {
		t.Fatalf("NormalizeCIDRs failed: %v", err)
	}

	// 162.159.135.0/24, 136.0/24, 137.0/24, 138.0/24 should all be pruned
	for _, c := range v4 {
		if strings.HasPrefix(c, "162.159.13") {
			t.Errorf("Subnet %s was not pruned and conflicts with 162.159.128.0/20", c)
		}
	}

	// Verify that 162.159.128.0/20, 104.16.0.0/13, 104.24.0.0/14, 172.64.0.0/13, 188.114.96.0/20 are present
	expected := []string{
		"104.16.0.0/13",
		"104.24.0.0/14",
		"162.159.128.0/20",
		"172.64.0.0/13",
		"188.114.96.0/20",
	}

	if len(v4) != len(expected) {
		t.Fatalf("Expected %d normalized v4 CIDRs, got %d: %v", len(expected), len(v4), v4)
	}

	for i, exp := range expected {
		if v4[i] != exp {
			t.Errorf("At index %d: expected %s, got %s", i, exp, v4[i])
		}
	}

	_, normV6, err := firewall.NormalizeCIDRs(firewall.DefaultDiscordIPv6CIDRs)
	if err != nil {
		t.Fatalf("NormalizeCIDRs v6 failed: %v", err)
	}
	if len(normV6) != 1 || normV6[0] != "2606:4700::/32" {
		t.Errorf("Expected [2606:4700::/32], got %v", normV6)
	}
	_ = v6
}

// TestNFTablesIntervalSyntaxValidation verifies that nft -c accepts the normalized set without error
func TestNFTablesIntervalSyntaxValidation(t *testing.T) {
	if _, err := exec.LookPath("nft"); err != nil {
		t.Skip("nft not installed, skipping nft -c test")
	}

	v4, _, err := firewall.NormalizeCIDRs(firewall.DefaultDiscordIPv4CIDRs)
	if err != nil {
		t.Fatalf("NormalizeCIDRs failed: %v", err)
	}

	script := "add table inet test_validation_table\n" +
		"add set inet test_validation_table s { type ipv4_addr; flags interval; elements = { " +
		strings.Join(v4, ", ") + " }; }\n"

	// Validate using unshare user namespace to avoid requiring root for nft -c
	cmd := exec.Command("unshare", "-r", "-n", "nft", "-c", "-f", "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "Operation not permitted") {
			t.Skip("network namespaces are unavailable in this test environment")
		}
		t.Fatalf("nft -c failed on normalized set: %s (%v)", string(out), err)
	}
}
