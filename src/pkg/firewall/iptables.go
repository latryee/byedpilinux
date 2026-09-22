package firewall

import (
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"discord-bypass/src/pkg/utils"
)

type IPTablesManager struct {
	manageIPv6 bool
	mu         sync.Mutex
	active     bool
	mode       string
	queueNum   int
	proxyPort  int
	blockQUIC  bool
	currentV4  []net.IP
	currentV6  []net.IP
}

func NewIPTablesManager(manageIPv6 bool) *IPTablesManager {
	return &IPTablesManager{
		manageIPv6: manageIPv6,
	}
}

func (m *IPTablesManager) DriverName() string {
	return "iptables"
}

func (m *IPTablesManager) Setup(mode string, queueNum int, proxyPort int, blockQUIC bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.mode = mode
	m.queueNum = queueNum
	m.proxyPort = proxyPort
	m.blockQUIC = blockQUIC

	_ = m.cleanupUnlocked()

	if mode == "nfqueue" {
		// IPv4 mangle table chain
		if err := exec.Command("iptables", "-t", "mangle", "-N", "DISCORD_BYPASS").Run(); err != nil {
			return fmt.Errorf("failed to create iptables chain: %w", err)
		}
		if err := exec.Command("iptables", "-t", "mangle", "-I", "OUTPUT", "1", "-j", "DISCORD_BYPASS").Run(); err != nil {
			return fmt.Errorf("failed to insert jump to DISCORD_BYPASS: %w", err)
		}

		if m.manageIPv6 {
			_ = exec.Command("ip6tables", "-t", "mangle", "-N", "DISCORD_BYPASS").Run()
			_ = exec.Command("ip6tables", "-t", "mangle", "-I", "OUTPUT", "1", "-j", "DISCORD_BYPASS").Run()
		}
	} else if mode == "redirect" {
		if err := exec.Command("iptables", "-t", "nat", "-N", "DISCORD_BYPASS_NAT").Run(); err != nil {
			return fmt.Errorf("failed to create iptables nat chain: %w", err)
		}
		if err := exec.Command("iptables", "-t", "nat", "-I", "OUTPUT", "1", "-j", "DISCORD_BYPASS_NAT").Run(); err != nil {
			return fmt.Errorf("failed to insert nat jump: %w", err)
		}

		if m.manageIPv6 {
			_ = exec.Command("ip6tables", "-t", "nat", "-N", "DISCORD_BYPASS_NAT").Run()
			_ = exec.Command("ip6tables", "-t", "nat", "-I", "OUTPUT", "1", "-j", "DISCORD_BYPASS_NAT").Run()
		}
	}

	m.active = true
	utils.Info("iptables chains configured successfully (mode=%s)", mode)
	return nil
}

func (m *IPTablesManager) Teardown() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cleanupUnlocked()
}

func (m *IPTablesManager) cleanupUnlocked() error {
	_ = exec.Command("iptables", "-t", "mangle", "-D", "OUTPUT", "-j", "DISCORD_BYPASS").Run()
	_ = exec.Command("iptables", "-t", "mangle", "-F", "DISCORD_BYPASS").Run()
	_ = exec.Command("iptables", "-t", "mangle", "-X", "DISCORD_BYPASS").Run()

	_ = exec.Command("iptables", "-t", "nat", "-D", "OUTPUT", "-j", "DISCORD_BYPASS_NAT").Run()
	_ = exec.Command("iptables", "-t", "nat", "-F", "DISCORD_BYPASS_NAT").Run()
	_ = exec.Command("iptables", "-t", "nat", "-X", "DISCORD_BYPASS_NAT").Run()

	if m.manageIPv6 {
		_ = exec.Command("ip6tables", "-t", "mangle", "-D", "OUTPUT", "-j", "DISCORD_BYPASS").Run()
		_ = exec.Command("ip6tables", "-t", "mangle", "-F", "DISCORD_BYPASS").Run()
		_ = exec.Command("ip6tables", "-t", "mangle", "-X", "DISCORD_BYPASS").Run()

		_ = exec.Command("ip6tables", "-t", "nat", "-D", "OUTPUT", "-j", "DISCORD_BYPASS_NAT").Run()
		_ = exec.Command("ip6tables", "-t", "nat", "-F", "DISCORD_BYPASS_NAT").Run()
		_ = exec.Command("ip6tables", "-t", "nat", "-X", "DISCORD_BYPASS_NAT").Run()
	}

	m.active = false
	return nil
}

func (m *IPTablesManager) UpdateIPSets(v4 []net.IP, v6 []net.IP) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.active {
		return fmt.Errorf("iptables is not active")
	}

	// Flush and re-populate rules
	if m.mode == "nfqueue" {
		_ = exec.Command("iptables", "-t", "mangle", "-F", "DISCORD_BYPASS").Run()
		for _, ip := range v4 {
			if ip4 := ip.To4(); ip4 != nil {
				if m.blockQUIC {
					_ = exec.Command("iptables", "-t", "mangle", "-A", "DISCORD_BYPASS", "-d", ip4.String(), "-p", "udp", "--dport", "443", "-j", "DROP").Run()
				}
				_ = exec.Command("iptables", "-t", "mangle", "-A", "DISCORD_BYPASS", "-d", ip4.String(), "-p", "tcp", "--dport", "443", "-j", "NFQUEUE", "--queue-num", strconv.Itoa(m.queueNum), "--queue-bypass").Run()
			}
		}
		if m.manageIPv6 {
			_ = exec.Command("ip6tables", "-t", "mangle", "-F", "DISCORD_BYPASS").Run()
			for _, ip := range v6 {
				if ip.To4() == nil && ip.To16() != nil {
					if m.blockQUIC {
						_ = exec.Command("ip6tables", "-t", "mangle", "-A", "DISCORD_BYPASS", "-d", ip.String(), "-p", "udp", "--dport", "443", "-j", "DROP").Run()
					}
					_ = exec.Command("ip6tables", "-t", "mangle", "-A", "DISCORD_BYPASS", "-d", ip.String(), "-p", "tcp", "--dport", "443", "-j", "NFQUEUE", "--queue-num", strconv.Itoa(m.queueNum), "--queue-bypass").Run()
				}
			}
		}
	} else if m.mode == "redirect" {
		_ = exec.Command("iptables", "-t", "nat", "-F", "DISCORD_BYPASS_NAT").Run()
		for _, ip := range v4 {
			if ip4 := ip.To4(); ip4 != nil {
				_ = exec.Command("iptables", "-t", "nat", "-A", "DISCORD_BYPASS_NAT", "-d", ip4.String(), "-p", "tcp", "--dport", "443", "-j", "REDIRECT", "--to-ports", strconv.Itoa(m.proxyPort)).Run()
			}
		}
		if m.manageIPv6 {
			_ = exec.Command("ip6tables", "-t", "nat", "-F", "DISCORD_BYPASS_NAT").Run()
			for _, ip := range v6 {
				if ip.To4() == nil && ip.To16() != nil {
					_ = exec.Command("ip6tables", "-t", "nat", "-A", "DISCORD_BYPASS_NAT", "-d", ip.String(), "-p", "tcp", "--dport", "443", "-j", "REDIRECT", "--to-ports", strconv.Itoa(m.proxyPort)).Run()
				}
			}
		}
	}

	m.currentV4 = v4
	m.currentV6 = v6
	return nil
}

func (m *IPTablesManager) IsActive() bool {
	cmd := exec.Command("iptables", "-t", "mangle", "-L", "DISCORD_BYPASS", "-n")
	if cmd.Run() == nil {
		return true
	}
	cmdNat := exec.Command("iptables", "-t", "nat", "-L", "DISCORD_BYPASS_NAT", "-n")
	return cmdNat.Run() == nil
}

func (m *IPTablesManager) GetStats() (packets uint64, bytes uint64, err error) {
	out, err := exec.Command("iptables", "-t", "mangle", "-L", "DISCORD_BYPASS", "-v", "-n", "-x").Output()
	if err != nil {
		// Try nat table
		out, err = exec.Command("iptables", "-t", "nat", "-L", "DISCORD_BYPASS_NAT", "-v", "-n", "-x").Output()
		if err != nil {
			return 0, 0, err
		}
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			if p, err := strconv.ParseUint(fields[0], 10, 64); err == nil {
				packets += p
			}
			if b, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
				bytes += b
			}
		}
	}
	return packets, bytes, nil
}
