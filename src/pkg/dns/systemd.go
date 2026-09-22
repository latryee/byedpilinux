package dns

import (
	"fmt"
	"net"
	"os/exec"
	"strings"

	"discord-bypass/src/pkg/utils"
)

// ConfigureSystemdResolved sets up split DNS for Discord domains on the active interface
func ConfigureSystemdResolved(iface string, proxyAddr string) error {
	if _, err := exec.LookPath("resolvectl"); err != nil {
		return nil
	}
	if iface == "" {
		iface = getDefaultInterface()
	}
	if iface == "" {
		return nil
	}

	// 1. Assign DNS server to interface
	dnsCmd := exec.Command("resolvectl", "dns", iface, proxyAddr)
	if out, err := dnsCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("resolvectl dns failed: %s (%w)", strings.TrimSpace(string(out)), err)
	}

	// 2. Assign routing domains with '~' prefix for split DNS
	domains := []string{"~discord.com", "~discordapp.com", "~discord.gg", "~discordapp.net", "~discord.media"}
	args := append([]string{"domain", iface}, domains...)
	domainCmd := exec.Command("resolvectl", args...)
	if out, err := domainCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("resolvectl domain failed: %s (%w)", strings.TrimSpace(string(out)), err)
	}

	utils.Info("Configured systemd-resolved split DNS on interface %s for %v", iface, domains)
	return nil
}

// RevertSystemdResolved reverts interface DNS configuration to default
func RevertSystemdResolved(iface string) error {
	if _, err := exec.LookPath("resolvectl"); err != nil {
		return nil
	}
	if iface == "" {
		iface = getDefaultInterface()
	}
	if iface == "" {
		return nil
	}

	cmd := exec.Command("resolvectl", "revert", iface)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("resolvectl revert failed: %s (%w)", strings.TrimSpace(string(out)), err)
	}

	utils.Info("Reverted systemd-resolved DNS settings on interface %s", iface)
	return nil
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
