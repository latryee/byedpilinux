package dns

import (
	"context"
	"net"
	"sync"
	"time"

	"discord-bypass/src/pkg/utils"
)

type FirewallIPSetUpdater interface {
	UpdateIPSets(v4 []net.IP, v6 []net.IP) error
}

type IPUpdater struct {
	domains     []string
	provider    string
	interval    time.Duration
	fw          FirewallIPSetUpdater
	syncHosts   bool
	stopChan    chan struct{}
	mu          sync.RWMutex
	cachedV4    []net.IP
	cachedV6    []net.IP
	isRunning   bool
}

func NewIPUpdater(domains []string, provider string, intervalSec int, fw FirewallIPSetUpdater, syncHosts bool) *IPUpdater {
	if intervalSec <= 0 {
		intervalSec = 300
	}
	return &IPUpdater{
		domains:   domains,
		provider:  provider,
		interval:  time.Duration(intervalSec) * time.Second,
		fw:        fw,
		syncHosts: syncHosts,
		stopChan:  make(chan struct{}),
	}
}

func (u *IPUpdater) Start() {
	u.mu.Lock()
	if u.isRunning {
		u.mu.Unlock()
		return
	}
	u.isRunning = true
	u.mu.Unlock()

	// Perform initial immediate resolve
	u.resolveAndSync()

	go func() {
		ticker := time.NewTicker(u.interval)
		defer ticker.Stop()

		for {
			select {
			case <-u.stopChan:
				return
			case <-ticker.C:
				u.resolveAndSync()
			}
		}
	}()
}

func (u *IPUpdater) Stop() {
	u.mu.Lock()
	defer u.mu.Unlock()
	if !u.isRunning {
		return
	}
	u.isRunning = false
	close(u.stopChan)

	if u.syncHosts {
		_ = RemoveDiscordHosts()
	}
}

func (u *IPUpdater) resolveAndSync() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	v4Map := make(map[string]net.IP)
	v6Map := make(map[string]net.IP)
	domainToIP := make(map[string]net.IP)

	for _, domain := range u.domains {
		v4, v6, err := ResolveBoth(ctx, domain, u.provider)
		if err != nil {
			utils.Debug("DoH resolution for %s failed: %v", domain, err)
			continue
		}
		for _, ip := range v4 {
			if !IsSinkholeIP(ip) {
				v4Map[ip.String()] = ip
				if _, exists := domainToIP[domain]; !exists {
					domainToIP[domain] = ip
				}
			}
		}
		for _, ip := range v6 {
			if !IsSinkholeIP(ip) {
				v6Map[ip.String()] = ip
			}
		}
	}

	var allV4 []net.IP
	for _, ip := range v4Map {
		allV4 = append(allV4, ip)
	}

	var allV6 []net.IP
	for _, ip := range v6Map {
		allV6 = append(allV6, ip)
	}

	u.mu.Lock()
	u.cachedV4 = allV4
	u.cachedV6 = allV6
	u.mu.Unlock()

	utils.Debug("Resolved %d IPv4 and %d IPv6 Discord addresses via DoH (sinkholes filtered)", len(allV4), len(allV6))

	if u.fw != nil && (len(allV4) > 0 || len(allV6) > 0) {
		if err := u.fw.UpdateIPSets(allV4, allV6); err != nil {
			utils.Warn("Failed to update firewall IP sets: %v", err)
		}
	}

	if u.syncHosts && len(domainToIP) > 0 {
		if err := ApplyDiscordHosts(domainToIP); err != nil {
			utils.Warn("Failed to update /etc/hosts with secure Discord IPs: %v", err)
		}
	}
}

func (u *IPUpdater) GetCachedIPs() ([]net.IP, []net.IP) {
	u.mu.RLock()
	defer u.mu.RUnlock()
	v4 := make([]net.IP, len(u.cachedV4))
	copy(v4, u.cachedV4)
	v6 := make([]net.IP, len(u.cachedV6))
	copy(v6, u.cachedV6)
	return v4, v6
}
