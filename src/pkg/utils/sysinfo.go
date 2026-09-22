package utils

import (
	"bufio"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
)

type SystemInfo struct {
	OSName             string
	OSVersion          string
	OSID               string
	KernelVersion      string
	IsRoot             bool
	HasNftables        bool
	HasIptables        bool
	HasIPv6            bool
	HasNetworkManager  bool
	HasSystemdResolved bool
	DefaultInterface   string
}

func DetectSystem() (*SystemInfo, error) {
	info := &SystemInfo{
		IsRoot: os.Geteuid() == 0,
	}

	// Read /etc/os-release
	if file, err := os.Open("/etc/os-release"); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				info.OSName = strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
			} else if strings.HasPrefix(line, "ID=") {
				info.OSID = strings.Trim(strings.TrimPrefix(line, "ID="), "\"")
			} else if strings.HasPrefix(line, "VERSION_ID=") {
				info.OSVersion = strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), "\"")
			}
		}
	}

	// Kernel version via uname -r
	if out, err := exec.Command("uname", "-r").Output(); err == nil {
		info.KernelVersion = strings.TrimSpace(string(out))
	}

	// Check nftables command
	if _, err := exec.LookPath("nft"); err == nil {
		info.HasNftables = true
	}

	// Check iptables command
	if _, err := exec.LookPath("iptables"); err == nil {
		info.HasIptables = true
	}

	// Check if IPv6 is actually routable on the system (must be global unicast, not link-local fe80::)
	if _, err := os.Stat("/proc/net/if_inet6"); err == nil {
		hasGlobalV6 := false
		if addrs, err := net.InterfaceAddrs(); err == nil {
			for _, addr := range addrs {
				if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
					if ipNet.IP.To4() == nil && ipNet.IP.To16() != nil && ipNet.IP.IsGlobalUnicast() {
						hasGlobalV6 = true
						break
					}
				}
			}
		}
		if hasGlobalV6 {
			// Test if outbound IPv6 route actually works
			d := net.Dialer{Timeout: 1 * time.Second}
			if conn, err := d.Dial("udp6", "[2606:4700:4700::1111]:53"); err == nil {
				conn.Close()
				info.HasIPv6 = true
			}
		}
	}

	// Check if NetworkManager is present
	if _, err := exec.LookPath("nmcli"); err == nil {
		info.HasNetworkManager = true
	}

	// Check if systemd-resolved is active
	if _, err := exec.LookPath("resolvectl"); err == nil {
		info.HasSystemdResolved = true
	}

	// Detect default outbound network interface
	if conn, err := net.Dial("udp", "8.8.8.8:80"); err == nil {
		defer conn.Close()
		if localAddr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
			if ifaces, err := net.Interfaces(); err == nil {
				for _, iface := range ifaces {
					if addrs, err := iface.Addrs(); err == nil {
						for _, a := range addrs {
							if ipNet, ok := a.(*net.IPNet); ok && ipNet.IP.Equal(localAddr.IP) {
								info.DefaultInterface = iface.Name
								break
							}
						}
					}
					if info.DefaultInterface != "" {
						break
					}
				}
			}
		}
	}

	return info, nil
}
