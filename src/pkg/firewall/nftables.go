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
	tableName     string
	manageIPv6    bool
	mu            sync.Mutex
	active        bool
	currentV4Nets []*net.IPNet
	currentV6Nets []*net.IPNet
}

// NFQWSDesyncFWMark is zapret nfqws v72.13's Linux default. nfqws applies it
// to raw injected packets so this table accepts them before its NFQUEUE rule.
const NFQWSDesyncFWMark = "0x40000000"

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

	// Normalize CIDRs to eliminate any nested/conflicting intervals
	normV4, _, err := NormalizeCIDRs(DefaultDiscordIPv4CIDRs)
	if err != nil {
		return fmt.Errorf("failed to normalize IPv4 CIDRs: %w", err)
	}

	var normV6 []string
	if m.manageIPv6 {
		_, normV6, err = NormalizeCIDRs(DefaultDiscordIPv6CIDRs)
		if err != nil {
			return fmt.Errorf("failed to normalize IPv6 CIDRs: %w", err)
		}
	}

	// Cache normalized IP networks for runtime duplicate/subsumption checking
	m.currentV4Nets = nil
	for _, s := range normV4 {
		if n, e := ParseCIDRorIP(s); e == nil {
			m.currentV4Nets = append(m.currentV4Nets, n)
		}
	}
	m.currentV6Nets = nil
	for _, s := range normV6 {
		if n, e := ParseCIDRorIP(s); e == nil {
			m.currentV6Nets = append(m.currentV6Nets, n)
		}
	}

	scriptStr, err := m.BuildScript(mode, queueNum, proxyPort, blockQUIC)
	if err != nil {
		return err
	}

	// Remove only a prior instance of our dedicated table. The subsequent
	// script is a single nft transaction; on failure its partial table is
	// removed below and unrelated firewall state is never touched.
	_ = exec.Command("nft", "delete", "table", "inet", m.tableName).Run()

	cmd := exec.Command("nft", "-f", "-")
	cmd.Stdin = strings.NewReader(scriptStr)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Transactional rollback: cleanly remove any partially created table on failure
		_ = exec.Command("nft", "delete", "table", "inet", m.tableName).Run()
		m.active = false
		m.currentV4Nets = nil
		m.currentV6Nets = nil
		return fmt.Errorf("nft setup failed: %s (%w)", strings.TrimSpace(string(out)), err)
	}

	m.active = true
	utils.Info("nftables table 'inet %s' created successfully (mode=%s, block_quic=%v, v4_intervals=%d, v6_intervals=%d)",
		m.tableName, mode, blockQUIC, len(normV4), len(normV6))
	return nil
}

// BuildScript formats the complete nftables ruleset definition for table creation.
func (m *NFTablesManager) BuildScript(mode string, queueNum int, proxyPort int, blockQUIC bool) (string, error) {
	normV4, _, err := NormalizeCIDRs(DefaultDiscordIPv4CIDRs)
	if err != nil {
		return "", fmt.Errorf("failed to normalize IPv4 CIDRs: %w", err)
	}

	var normV6 []string
	if m.manageIPv6 {
		_, normV6, err = NormalizeCIDRs(DefaultDiscordIPv6CIDRs)
		if err != nil {
			return "", fmt.Errorf("failed to normalize IPv6 CIDRs: %w", err)
		}
	}

	var script bytes.Buffer
	script.WriteString(fmt.Sprintf("add table inet %s\n", m.tableName))
	script.WriteString(fmt.Sprintf("add set inet %s discord_v4 { type ipv4_addr; flags interval; elements = { %s }; }\n", m.tableName, strings.Join(normV4, ", ")))
	if m.manageIPv6 && len(normV6) > 0 {
		script.WriteString(fmt.Sprintf("add set inet %s discord_v6 { type ipv6_addr; flags interval; elements = { %s }; }\n", m.tableName, strings.Join(normV6, ", ")))
	}

	if mode == "nfqueue" {
		script.WriteString(fmt.Sprintf("add chain inet %s output { type filter hook output priority 0; policy accept; }\n", m.tableName))
		// This must precede NFQUEUE: raw packets injected by nfqws have its
		// upstream-supported SO_MARK and must not be queued a second time.
		script.WriteString(fmt.Sprintf("add rule inet %s output meta mark %s counter accept\n", m.tableName, NFQWSDesyncFWMark))
		if blockQUIC {
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
		return "", fmt.Errorf("unknown mode: %s", mode)
	}

	return script.String(), nil
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
	m.currentV4Nets = nil
	m.currentV6Nets = nil
	utils.Info("nftables table 'inet %s' removed cleanly", m.tableName)
	return nil
}

func (m *NFTablesManager) UpdateIPSets(v4 []net.IP, v6 []net.IP) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.active && !m.IsActive() {
		return fmt.Errorf("nftables table %s is not active", m.tableName)
	}

	// Filter out any IP that is already covered by existing CIDRs to avoid conflicting intervals
	var newV4Strs []string
	for _, ip := range v4 {
		if ip4 := ip.To4(); ip4 != nil {
			if !IsCoveredIP(ip4, m.currentV4Nets) {
				newV4Strs = append(newV4Strs, ip4.String())
				if n, e := ParseCIDRorIP(ip4.String()); e == nil {
					m.currentV4Nets = append(m.currentV4Nets, n)
				}
			}
		}
	}

	var newV6Strs []string
	if m.manageIPv6 {
		for _, ip := range v6 {
			if ip.To4() == nil && ip.To16() != nil {
				if !IsCoveredIP(ip, m.currentV6Nets) {
					newV6Strs = append(newV6Strs, ip.String())
					if n, e := ParseCIDRorIP(ip.String()); e == nil {
						m.currentV6Nets = append(m.currentV6Nets, n)
					}
				}
			}
		}
	}

	if len(newV4Strs) == 0 && len(newV6Strs) == 0 {
		return nil
	}

	var script bytes.Buffer
	if len(newV4Strs) > 0 {
		script.WriteString(fmt.Sprintf("add element inet %s discord_v4 { %s }\n", m.tableName, strings.Join(newV4Strs, ", ")))
	}
	if len(newV6Strs) > 0 {
		script.WriteString(fmt.Sprintf("add element inet %s discord_v6 { %s }\n", m.tableName, strings.Join(newV6Strs, ", ")))
	}

	cmd := exec.Command("nft", "-f", "-")
	cmd.Stdin = &script
	out, err := cmd.CombinedOutput()
	if err != nil {
		if !strings.Contains(string(out), "File exists") {
			return fmt.Errorf("nft set update failed: %s (%w)", strings.TrimSpace(string(out)), err)
		}
	}

	utils.Debug("Updated nftables sets: added %d new IPv4 and %d new IPv6 entries", len(newV4Strs), len(newV6Strs))
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
