package tests

import (
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
