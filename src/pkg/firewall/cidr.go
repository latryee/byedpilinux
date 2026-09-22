package firewall

import (
	"bytes"
	"fmt"
	"net"
	"sort"
	"strings"
)

// DefaultDiscordIPv4CIDRs defines initial Cloudflare and Discord IPv4 netblocks.
var DefaultDiscordIPv4CIDRs = []string{
	"162.159.128.0/20",
	"162.159.135.0/24",
	"162.159.136.0/24",
	"162.159.137.0/24",
	"162.159.138.0/24",
	"104.16.0.0/13",
	"104.24.0.0/14",
	"172.64.0.0/13",
	"188.114.96.0/20",
}

// DefaultDiscordIPv6CIDRs defines initial Cloudflare and Discord IPv6 netblocks.
var DefaultDiscordIPv6CIDRs = []string{
	"2606:4700::/32",
}

// ParseCIDRorIP parses either a CIDR notation (e.g. "192.168.1.0/24") or a single IP address (e.g. "192.168.1.1").
// If a single IP is provided, it is masked as a host route (/32 for IPv4, /128 for IPv6).
func ParseCIDRorIP(s string) (*net.IPNet, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty CIDR/IP string")
	}

	if strings.Contains(s, "/") {
		_, ipNet, err := net.ParseCIDR(s)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR %q: %w", s, err)
		}
		// Ensure IP is canonical network base IP
		ipNet.IP = ipNet.IP.Mask(ipNet.Mask)
		return ipNet, nil
	}

	ip := net.ParseIP(s)
	if ip == nil {
		return nil, fmt.Errorf("invalid IP address %q", s)
	}

	if ip4 := ip.To4(); ip4 != nil {
		mask := net.CIDRMask(32, 32)
		return &net.IPNet{
			IP:   ip4,
			Mask: mask,
		}, nil
	}

	mask := net.CIDRMask(128, 128)
	return &net.IPNet{
		IP:   ip,
		Mask: mask,
	}, nil
}

// Covers checks if parentNet completely covers childNet.
// A covers B if:
// 1. Both are the same IP version (IPv4 or IPv6).
// 2. Parent's prefix length is <= child's prefix length (larger or equal network size).
// 3. Parent's network contains child's network base IP.
func Covers(parent, child *net.IPNet) bool {
	if parent == nil || child == nil {
		return false
	}

	pOnes, pBits := parent.Mask.Size()
	cOnes, cBits := child.Mask.Size()

	if pBits != cBits {
		return false // Different address families
	}

	if pOnes > cOnes {
		return false // Parent has smaller address space (longer prefix)
	}

	// Because CIDRs are power-of-two aligned, if parent's prefix is <= child's prefix,
	// and parent contains the start IP of child, parent covers the entire range of child.
	return parent.Contains(child.IP)
}

// IsCoveredIP checks if a single net.IP is contained in any of the provided networks.
func IsCoveredIP(ip net.IP, nets []*net.IPNet) bool {
	if ip == nil {
		return false
	}
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// NormalizeIPNets takes a slice of *net.IPNet and returns a minimal, pairwise-disjoint slice.
// Any subnet completely contained inside another network is removed.
// Duplicate networks are removed.
func NormalizeIPNets(nets []*net.IPNet) []*net.IPNet {
	if len(nets) == 0 {
		return []*net.IPNet{}
	}

	// 1. Deduplicate identical networks
	var valid []*net.IPNet
	seen := make(map[string]bool)
	for _, n := range nets {
		if n == nil {
			continue
		}
		key := n.String()
		if !seen[key] {
			seen[key] = true
			valid = append(valid, n)
		}
	}

	// 2. Sort by prefix length ascending (shorter prefix = larger network first).
	// Within the same prefix length, sort by IP bytes ascending for deterministic output.
	sort.Slice(valid, func(i, j int) bool {
		onesI, _ := valid[i].Mask.Size()
		onesJ, _ := valid[j].Mask.Size()
		if onesI != onesJ {
			return onesI < onesJ
		}
		return bytes.Compare(valid[i].IP, valid[j].IP) < 0
	})

	// 3. Filter out any network that is covered by a network already accepted.
	var normalized []*net.IPNet
	for _, candidate := range valid {
		covered := false
		for _, existing := range normalized {
			if Covers(existing, candidate) {
				covered = true
				break
			}
		}
		if !covered {
			normalized = append(normalized, candidate)
		}
	}

	// 4. Final sort by IP bytes ascending for clean, readable ordering
	sort.Slice(normalized, func(i, j int) bool {
		cmp := bytes.Compare(normalized[i].IP, normalized[j].IP)
		if cmp != 0 {
			return cmp < 0
		}
		onesI, _ := normalized[i].Mask.Size()
		onesJ, _ := normalized[j].Mask.Size()
		return onesI < onesJ
	})

	return normalized
}

// NormalizeCIDRs parses raw CIDR or IP strings, splits them into IPv4 and IPv6,
// and removes redundant subnets and duplicates. Returns sorted string representations.
func NormalizeCIDRs(rawCIDRs []string) (v4 []string, v6 []string, err error) {
	var v4Nets []*net.IPNet
	var v6Nets []*net.IPNet

	for _, raw := range rawCIDRs {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		ipNet, parseErr := ParseCIDRorIP(raw)
		if parseErr != nil {
			return nil, nil, parseErr
		}
		if ipNet.IP.To4() != nil {
			v4Nets = append(v4Nets, ipNet)
		} else {
			v6Nets = append(v6Nets, ipNet)
		}
	}

	normV4 := NormalizeIPNets(v4Nets)
	normV6 := NormalizeIPNets(v6Nets)

	for _, n := range normV4 {
		v4 = append(v4, n.String())
	}
	for _, n := range normV6 {
		v6 = append(v6, n.String())
	}

	return v4, v6, nil
}
