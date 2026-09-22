package dns

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"

	"discord-bypass/src/pkg/utils"
)

const (
	DNSStateFile = "/run/discord-bypass-dns.state"
	// Legacy dummy interface name for cleanup
	LegacyDummyInterface = "discord-dns"
)

// ConfigureSystemdResolved configures systemd-resolved to use the local Domain-Routing DNS proxy
// for the specified network interface. Unrelated domains are forwarded by the proxy to the original
// router/ISP DNS, while Discord domains are securely resolved via DoH.
func ConfigureSystemdResolved(iface string, proxyAddr string) error {
	if _, err := exec.LookPath("resolvectl"); err != nil {
		utils.Debug("resolvectl not found, skipping systemd-resolved integration")
		return nil
	}

	if iface == "" {
		iface = getDefaultInterface()
	}
	if iface == "" {
		return fmt.Errorf("no active default network interface found")
	}

	if proxyAddr == "" {
		proxyAddr = "127.0.0.1:5354"
	}

	// 1. Clean any legacy dummy interfaces left by previous versions
	cleanLegacyInterface()

	// 2. Capture and persist original DNS configuration for iface
	origDNS := GetInterfaceDNS(iface)
	if origDNS != "" && !strings.Contains(origDNS, "5354") {
		_ = os.WriteFile(DNSStateFile, []byte(fmt.Sprintf("%s:%s", iface, origDNS)), 0644)
	}

	// 3. Assign the local domain-routing proxy to the interface
	cmd := exec.Command("resolvectl", "dns", iface, proxyAddr)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("resolvectl dns on %s failed: %s (%w)", iface, strings.TrimSpace(string(out)), err)
	}

	// 4. Route only Discord domains through the proxy (split-DNS, never hijack system-wide ~.)
	_ = exec.Command("resolvectl", "domain", iface,
		"~discord.com", "~discordapp.com", "~discord.gg", "~discordapp.net", "~discord.media", "~discordstatus.com").Run()

	// 5. Flush caches to eliminate poisoned records immediately
	_ = exec.Command("resolvectl", "flush-caches").Run()

	utils.Info("Configured systemd-resolved on interface %s -> %s for Discord domains (Upstream preserved: %s)",
		iface, proxyAddr, origDNS)
	return nil
}

// RevertSystemdResolved cleanly restores the interface's original DNS settings
// and removes any temporary state or interfaces.
func RevertSystemdResolved(iface string) error {
	if _, err := exec.LookPath("resolvectl"); err != nil {
		return nil
	}

	// CRITICAL SAFETY GUARD:
	// If DNSStateFile does not exist, discord-bypass NEVER modified systemd-resolved!
	// Calling 'resolvectl revert <link>' blindly on Ubuntu/Debian/Zorin wipes the interface's
	// DHCP DNS server list and breaks system-wide internet completely!
	if _, err := os.Stat(DNSStateFile); os.IsNotExist(err) {
		cleanLegacyInterface()
		return nil
	}

	if iface == "" {
		iface = getDefaultInterface()
	}

	// 1. Read original DNS from state file if present
	var origDNS string
	if data, err := os.ReadFile(DNSStateFile); err == nil {
		parts := strings.SplitN(strings.TrimSpace(string(data)), ":", 2)
		if len(parts) == 2 {
			if iface == "" {
				iface = parts[0]
			}
			origDNS = parts[1]
		}
	}

	// Fallback to router default gateway DNS if origDNS is somehow empty
	if origDNS == "" {
		origDNS = getRouterDNS()
	}

	// 2. Revert resolvectl on the interface and explicitly assign origDNS
	if iface != "" {
		_ = exec.Command("resolvectl", "revert", iface).Run()
		if origDNS != "" && !strings.Contains(origDNS, "5354") {
			_ = exec.Command("resolvectl", "dns", iface, origDNS).Run()
			_ = exec.Command("resolvectl", "default-route", iface, "true").Run()
			_ = exec.Command("resolvectl", "domain", iface, "").Run()
		}
	}

	// 3. Clean any global overrides
	_ = exec.Command("resolvectl", "revert", "").Run()
	_ = exec.Command("resolvectl", "dns", "", "").Run()

	// 4. Clean up legacy dummy interface if present
	cleanLegacyInterface()

	// 5. Remove state file
	_ = os.Remove(DNSStateFile)

	// 6. Flush caches
	_ = exec.Command("resolvectl", "flush-caches").Run()

	utils.Info("Safely restored systemd-resolved DNS configuration on %s (DNS: %s)", iface, origDNS)
	return nil
}

func getRouterDNS() string {
	out, err := exec.Command("ip", "route", "show", "default").Output()
	if err == nil {
		fields := strings.Fields(string(out))
		for i, f := range fields {
			if f == "via" && i+1 < len(fields) {
				return fields[i+1]
			}
		}
	}
	return "192.168.1.1"
}

// GetInterfaceDNS queries resolvectl for the current DNS server of an interface
func GetInterfaceDNS(iface string) string {
	if iface == "" {
		return ""
	}
	out, err := exec.Command("resolvectl", "dns", iface).Output()
	if err != nil {
		return ""
	}
	// Output format: "Link 2 (enp5s0): 192.168.1.1"
	line := strings.TrimSpace(string(out))
	if idx := strings.Index(line, ":"); idx != -1 {
		servers := strings.Fields(line[idx+1:])
		if len(servers) > 0 {
			return servers[0]
		}
	}
	return ""
}

func cleanLegacyInterface() {
	if _, err := net.InterfaceByName(LegacyDummyInterface); err == nil {
		_ = exec.Command("resolvectl", "revert", LegacyDummyInterface).Run()
		_ = exec.Command("ip", "link", "set", "dev", LegacyDummyInterface, "down").Run()
		_ = exec.Command("ip", "link", "del", "dev", LegacyDummyInterface).Run()
		utils.Debug("Cleaned legacy dummy interface %s", LegacyDummyInterface)
	}
}

func getDefaultInterface() string {
	if conn, err := net.Dial("udp", "8.8.8.8:80"); err == nil {
		defer conn.Close()
		if localAddr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
			if ifaces, err := net.Interfaces(); err == nil {
				for _, iface := range ifaces {
					if addrs, err := iface.Addrs(); err == nil {
						for _, a := range addrs {
							if ipNet, ok := a.(*net.IPNet); ok && ipNet.IP.Equal(localAddr.IP) {
								return iface.Name
							}
						}
					}
				}
			}
		}
	}
	return ""
}
