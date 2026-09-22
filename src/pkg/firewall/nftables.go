package firewall

import (
	"bytes"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"discord-bypass/src/pkg/utils"
)

type NFTablesManager struct {
	tableName  string
	manageIPv6 bool
	mu         sync.Mutex
	active     bool
}

func NewNFTablesManager(tableName string, manageIPv6 bool) *NFTablesManager {
	if tableName == "" {
		tableName = "discord_bypass"
	}
	return &NFTablesManager{
		tableName:  tableName,
		manageIPv6: manageIPv6,
	}
}

func (m *NFTablesManager) DriverName() string {
	return "nftables"
}

func (m *NFTablesManager) Setup(mode string, queueNum int, proxyPort int, blockQUIC bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var script bytes.Buffer

	// Clean any pre-existing instance of our table
	script.WriteString(fmt.Sprintf("add table inet %s\n", m.tableName))
	script.WriteString(fmt.Sprintf("delete table inet %s\n", m.tableName))
	script.WriteString(fmt.Sprintf("add table inet %s\n", m.tableName))

	// Define IP sets with interval flag and pre-seeded Discord Cloudflare subnets
	script.WriteString(fmt.Sprintf("add set inet %s discord_v4 { type ipv4_addr; flags interval; elements = { 162.159.128.0/20, 162.159.135.0/24, 162.159.136.0/24, 162.159.137.0/24, 162.159.138.0/24, 104.16.0.0/13, 104.24.0.0/14, 172.64.0.0/13, 188.114.96.0/20 }; }\n", m.tableName))
	if m.manageIPv6 {
		script.WriteString(fmt.Sprintf("add set inet %s discord_v6 { type ipv6_addr; flags interval; elements = { 2606:4700::/32 }; }\n", m.tableName))
	}

	if mode == "nfqueue" {
		script.WriteString(fmt.Sprintf("add chain inet %s output { type filter hook output priority 0; policy accept; }\n", m.tableName))
		if blockQUIC {
			// Only drop UDP port 443 if explicitly configured via block_quic = true
			script.WriteString(fmt.Sprintf("add rule inet %s output ip daddr @discord_v4 udp dport 443 counter drop\n", m.tableName))
		}
		script.WriteString(fmt.Sprintf("add rule inet %s output ip daddr @discord_v4 tcp dport 443 counter queue num %d bypass\n", m.tableName, queueNum))

		if m.manageIPv6 {
			if blockQUIC {
				script.WriteString(fmt.Sprintf("add rule inet %s output ip6 daddr @discord_v6 udp dport 443 counter drop\n", m.tableName))
			}
			script.WriteString(fmt.Sprintf("add rule inet %s output ip6 daddr @discord_v6 tcp dport 443 counter queue num %d bypass\n", m.tableName, queueNum))
		}
	} else if mode == "redirect" {
		script.WriteString(fmt.Sprintf("add chain inet %s output { type nat hook output priority -100; policy accept; }\n", m.tableName))
		script.WriteString(fmt.Sprintf("add rule inet %s output ip daddr @discord_v4 tcp dport 443 counter redirect to :%d\n", m.tableName, proxyPort))
		if m.manageIPv6 {
			script.WriteString(fmt.Sprintf("add rule inet %s output ip6 daddr @discord_v6 tcp dport 443 counter redirect to :%d\n", m.tableName, proxyPort))
		}
	} else {
		return fmt.Errorf("unknown mode: %s", mode)
	}

	cmd := exec.Command("nft", "-f", "-")
	cmd.Stdin = &script
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("nft setup failed: %s (%w)", string(out), err)
	}

	m.active = true
	utils.Info("nftables table 'inet %s' created successfully (mode=%s, block_quic=%v)", m.tableName, mode, blockQUIC)
	return nil
}

func (m *NFTablesManager) Teardown() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cmd := exec.Command("nft", "delete", "table", "inet", m.tableName)
	out, err := cmd.CombinedOutput()
	if err != nil && !isNotExistErr(string(out)) {
		return fmt.Errorf("nft teardown failed: %s (%w)", string(out), err)
	}

	m.active = false
	utils.Info("nftables table 'inet %s' removed cleanly", m.tableName)
	return nil
}

func (m *NFTablesManager) UpdateIPSets(v4 []net.IP, v6 []net.IP) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.active && !m.IsActive() {
		return fmt.Errorf("nftables table %s is not active", m.tableName)
	}

	var script bytes.Buffer

	// Flush old elements first or add elements with auto-merging intervals
	if len(v4) > 0 {
		var v4Strs []string
		for _, ip := range v4 {
			if ip4 := ip.To4(); ip4 != nil {
				v4Strs = append(v4Strs, ip4.String())
			}
		}
		if len(v4Strs) > 0 {
			script.WriteString(fmt.Sprintf("add element inet %s discord_v4 { %s }\n", m.tableName, strings.Join(v4Strs, ", ")))
		}
	}

	if m.manageIPv6 && len(v6) > 0 {
		var v6Strs []string
		for _, ip := range v6 {
			if ip.To4() == nil && ip.To16() != nil {
				v6Strs = append(v6Strs, ip.String())
			}
		}
		if len(v6Strs) > 0 {
			script.WriteString(fmt.Sprintf("add element inet %s discord_v6 { %s }\n", m.tableName, strings.Join(v6Strs, ", ")))
		}
	}

	if script.Len() == 0 {
		return nil
	}

	cmd := exec.Command("nft", "-f", "-")
	cmd.Stdin = &script
	out, err := cmd.CombinedOutput()
	if err != nil {
		// If element already exists, nft may warn or succeed depending on flags; ignore duplicate error
		if !strings.Contains(string(out), "File exists") {
			return fmt.Errorf("nft set update failed: %s (%w)", string(out), err)
		}
	}

	utils.Debug("Updated nftables sets with %d IPv4 and %d IPv6 entries", len(v4), len(v6))
	return nil
}

func (m *NFTablesManager) IsActive() bool {
	cmd := exec.Command("nft", "list", "table", "inet", m.tableName)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return true
	}
	// If unprivileged, nft returns "Operation not permitted"
	if strings.Contains(string(out), "Operation not permitted") {
		// If daemon marked active, return true
		return m.active
	}
	return false
}

func (m *NFTablesManager) GetStats() (packets uint64, bytes uint64, err error) {
	cmd := exec.Command("nft", "list", "table", "inet", m.tableName)
	out, err := cmd.Output()
	if err != nil {
		return 0, 0, err
	}

	// Parse 'counter packets 123 bytes 45678'
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.Contains(line, "counter packets") {
			fields := strings.Fields(line)
			for i, f := range fields {
				if f == "packets" && i+1 < len(fields) {
					if p, e := strconv.ParseUint(fields[i+1], 10, 64); e == nil {
						packets += p
					}
				}
				if f == "bytes" && i+1 < len(fields) {
					if b, e := strconv.ParseUint(fields[i+1], 10, 64); e == nil {
						bytes += b
					}
				}
			}
		}
	}

	return packets, bytes, nil
}
