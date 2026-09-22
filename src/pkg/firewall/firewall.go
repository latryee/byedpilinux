package firewall

import (
	"fmt"
	"net"
	"os/exec"

	"discord-bypass/src/pkg/config"
	"discord-bypass/src/pkg/utils"
)

type FirewallManager interface {
	Setup(mode string, queueNum int, proxyPort int, blockQUIC bool) error
	Teardown() error
	UpdateIPSets(v4 []net.IP, v6 []net.IP) error
	IsActive() bool
	GetStats() (packets uint64, bytes uint64, err error)
	DriverName() string
}

func NewFirewallManager(cfg *config.Config, sys *utils.SystemInfo) (FirewallManager, error) {
	driver := cfg.Firewall.Driver
	if driver == "auto" || driver == "" {
		if sys.HasNftables {
			driver = "nftables"
		} else if sys.HasIptables {
			driver = "iptables"
		} else {
			return nil, fmt.Errorf("neither nftables nor iptables found on system")
		}
	}

	switch driver {
	case "nftables":
		return NewNFTablesManager(cfg.Firewall.TableName, cfg.Firewall.ManageIPv6 && sys.HasIPv6), nil
	case "iptables":
		return NewIPTablesManager(cfg.Firewall.ManageIPv6 && sys.HasIPv6), nil
	default:
		return nil, fmt.Errorf("unsupported firewall driver: %s", driver)
	}
}

// EmergencyDisable immediately purges all rules and tables created by discord-bypass across nftables and iptables
func EmergencyDisable() error {
	var errs []string

	// 1. Purge nftables table inet discord_bypass
	if _, err := exec.LookPath("nft"); err == nil {
		out, err := exec.Command("nft", "delete", "table", "inet", "discord_bypass").CombinedOutput()
		if err != nil && !isNotExistErr(string(out)) {
			errs = append(errs, fmt.Sprintf("nft error: %s (%v)", string(out), err))
		}
	}

	// 2. Purge iptables rules
	if _, err := exec.LookPath("iptables"); err == nil {
		_ = exec.Command("iptables", "-t", "mangle", "-D", "OUTPUT", "-j", "DISCORD_BYPASS").Run()
		_ = exec.Command("iptables", "-t", "mangle", "-F", "DISCORD_BYPASS").Run()
		_ = exec.Command("iptables", "-t", "mangle", "-X", "DISCORD_BYPASS").Run()

		_ = exec.Command("iptables", "-t", "nat", "-D", "OUTPUT", "-j", "DISCORD_BYPASS_NAT").Run()
		_ = exec.Command("iptables", "-t", "nat", "-F", "DISCORD_BYPASS_NAT").Run()
		_ = exec.Command("iptables", "-t", "nat", "-X", "DISCORD_BYPASS_NAT").Run()
	}

	// 3. Purge ip6tables rules
	if _, err := exec.LookPath("ip6tables"); err == nil {
		_ = exec.Command("ip6tables", "-t", "mangle", "-D", "OUTPUT", "-j", "DISCORD_BYPASS").Run()
		_ = exec.Command("ip6tables", "-t", "mangle", "-F", "DISCORD_BYPASS").Run()
		_ = exec.Command("ip6tables", "-t", "mangle", "-X", "DISCORD_BYPASS").Run()

		_ = exec.Command("ip6tables", "-t", "nat", "-D", "OUTPUT", "-j", "DISCORD_BYPASS_NAT").Run()
		_ = exec.Command("ip6tables", "-t", "nat", "-F", "DISCORD_BYPASS_NAT").Run()
		_ = exec.Command("ip6tables", "-t", "nat", "-X", "DISCORD_BYPASS_NAT").Run()
	}

	if len(errs) > 0 {
		return fmt.Errorf("emergency disable encountered errors: %s", errs)
	}
	return nil
}

func isNotExistErr(out string) bool {
	return out == "" ||
		contains(out, "No such file or directory") ||
		contains(out, "does not exist") ||
		contains(out, "No such chain")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > 0 && len(substr) > 0 && indexOf(s, substr) >= 0))
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
