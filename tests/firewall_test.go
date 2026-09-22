package tests

import (
	"strings"
	"testing"

	"discord-bypass/src/pkg/firewall"
)

func TestFirewallDrivers(t *testing.T) {
	nft := firewall.NewNFTablesManager("discord_bypass", true)
	if nft.DriverName() != "nftables" {
		t.Errorf("Expected driver name 'nftables', got %s", nft.DriverName())
	}

	ipt := firewall.NewIPTablesManager(true)
	if ipt.DriverName() != "iptables" {
		t.Errorf("Expected driver name 'iptables', got %s", ipt.DriverName())
	}
}

func TestNFQWSMarkConstant(t *testing.T) {
	if firewall.NFQWSDesyncFWMark != "0x40000000" {
		t.Fatalf("unexpected zapret nfqws fwmark: %s", firewall.NFQWSDesyncFWMark)
	}
	if !strings.HasPrefix(firewall.NFQWSDesyncFWMark, "0x") {
		t.Fatal("fwmark must be emitted as an nft-compatible hexadecimal literal")
	}
}
