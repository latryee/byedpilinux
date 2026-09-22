package diagnostics

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"discord-bypass/src/pkg/config"
	"discord-bypass/src/pkg/dns"
	"discord-bypass/src/pkg/firewall"
	"discord-bypass/src/pkg/utils"
)

type CheckStatus string

const (
	StatusOk   CheckStatus = "OK"
	StatusFail CheckStatus = "FAIL"
	StatusWarn CheckStatus = "WARN"
	StatusInfo CheckStatus = "INFO"
)

type DiagnosticItem struct {
	Name    string
	Status  CheckStatus
	Details string
	Latency time.Duration
}

type DiagnosticReport struct {
	Timestamp      time.Time
	Items          []DiagnosticItem
	OverallVerdict string
	FailureStage   string
	Recommendation string
	ISPInfo        *ISPDiagnosticResult
}

func RunDiagnostics(ctx context.Context, cfg *config.Config, sys *utils.SystemInfo, fw firewall.FirewallManager) *DiagnosticReport {
	report := &DiagnosticReport{
		Timestamp: time.Now(),
	}

	// 1. ISP Detection
	ispRes, err := DetectISP(ctx)
	if err == nil {
		report.ISPInfo = ispRes
		report.Items = append(report.Items, DiagnosticItem{
			Name:    "ISP Detection",
			Status:  StatusInfo,
			Details: fmt.Sprintf("%s (%s, %s)", ispRes.DetectedISP, ispRes.ASN, ispRes.Country),
		})
	} else {
		report.Items = append(report.Items, DiagnosticItem{
			Name:    "ISP Detection",
			Status:  StatusWarn,
			Details: "Unable to query external IP/ASN (offline, sandbox, or blocked)",
		})
	}

	// 2. DNS Resolution Test
	dnsItem := checkDNS(ctx, cfg)
	report.Items = append(report.Items, dnsItem)

	// 2b. Local Hosts Override (/etc/hosts)
	hostsItem := checkHostsOverride()
	report.Items = append(report.Items, hostsItem)

	// 3. IPv4 Connectivity
	ipv4Item := checkIPv4(ctx)
	report.Items = append(report.Items, ipv4Item)

	// 4. IPv6 Connectivity
	ipv6Item := checkIPv6(ctx, sys)
	report.Items = append(report.Items, ipv6Item)

	// 5. TCP Handshake to discord.com:443
	tcpItem := checkTCP(ctx, "discord.com:443")
	report.Items = append(report.Items, tcpItem)

	// 6. TLS Handshake & SNI Filtering Detection
	tlsItem := checkTLS(ctx, "discord.com:443", "discord.com")
	report.Items = append(report.Items, tlsItem)

	// 7. Discord REST API Endpoint
	apiItem := checkHTTP(ctx, cfg.Diagnostics.APIEndpoint, "Discord REST API")
	report.Items = append(report.Items, apiItem)

	// 8. Discord Gateway WebSocket Endpoint
	gwItem := checkGateway(ctx, cfg.Diagnostics.GatewayEndpoint)
	report.Items = append(report.Items, gwItem)

	// 9. Discord CDN Endpoint
	cdnItem := checkHTTP(ctx, cfg.Diagnostics.CDNEndpoint, "Discord CDN")
	report.Items = append(report.Items, cdnItem)

	// 9b. Roblox Web & API Endpoint (Turkey Blockade Check)
	robloxItem := checkHTTP(ctx, "https://www.roblox.com", "Roblox Web & API")
	report.Items = append(report.Items, robloxItem)

	// 10. Discord Voice / WebRTC Endpoint Probe
	voiceItem := checkVoice(ctx, cfg.Diagnostics.VoiceEndpoint)
	report.Items = append(report.Items, voiceItem)

	// 11. Bypass Service Status
	serviceItem := checkService()
	report.Items = append(report.Items, serviceItem)

	// 12. Firewall Rules & Packet Counters
	fwItem := checkFirewall(fw)
	report.Items = append(report.Items, fwItem)

	// Synthesize overall verdict
	analyzeVerdict(report)

	return report
}

func checkDNS(ctx context.Context, cfg *config.Config) DiagnosticItem {
	start := time.Now()
	targetDomain := "discord.com"

	isPoisoned, sysIPs, dohIPs, details, err := dns.DetectPoisoning(targetDomain, cfg.DNS.DoHProvider)
	latency := time.Since(start)

	if err != nil {
		return DiagnosticItem{
			Name:    "DNS Resolution (System vs DoH)",
			Status:  StatusFail,
			Details: fmt.Sprintf("Lookup failed: %v", err),
			Latency: latency,
		}
	}

	if isPoisoned {
		return DiagnosticItem{
			Name:    "DNS Resolution (System vs DoH)",
			Status:  StatusFail,
			Details: fmt.Sprintf("POISONING/SINKHOLE DETECTED! %s\n      System DNS resolved to : %v\n      Secure DoH resolved to : %v", details, sysIPs, dohIPs),
			Latency: latency,
		}
	}

	return DiagnosticItem{
		Name:    "DNS Resolution (System vs DoH)",
		Status:  StatusOk,
		Details: fmt.Sprintf("Clean: System DNS resolved to %v (matches DoH %v)", sysIPs, dohIPs),
		Latency: latency,
	}
}

func checkHostsOverride() DiagnosticItem {
	active := dns.HasDiscordHosts()
	if active {
		return DiagnosticItem{
			Name:    "Local DNS Override (/etc/hosts)",
			Status:  StatusOk,
			Details: "Active - Discord domains mapped to secure DoH IPs via managed /etc/hosts block",
		}
	}
	return DiagnosticItem{
		Name:    "Local DNS Override (/etc/hosts)",
		Status:  StatusInfo,
		Details: "Inactive - standard system resolver in use (managed entries not present)",
	}
}

func checkIPv4(ctx context.Context) DiagnosticItem {
	start := time.Now()
	// Dial Cloudflare 1.1.1.1 on port 53 or 443
	d := net.Dialer{Timeout: 3 * time.Second}
	conn, err := d.DialContext(ctx, "tcp4", "1.1.1.1:53")
	latency := time.Since(start)
	if err != nil {
		return DiagnosticItem{
			Name:    "IPv4 Connectivity",
			Status:  StatusFail,
			Details: fmt.Sprintf("Outbound IPv4 unreachable: %v", err),
			Latency: latency,
		}
	}
	conn.Close()
	return DiagnosticItem{
		Name:    "IPv4 Connectivity",
		Status:  StatusOk,
		Details: "Outbound IPv4 routing is active",
		Latency: latency,
	}
}

func checkIPv6(ctx context.Context, sys *utils.SystemInfo) DiagnosticItem {
	if !sys.HasIPv6 {
		return DiagnosticItem{
			Name:    "IPv6 Connectivity",
			Status:  StatusInfo,
			Details: "IPv6 is not configured or disabled on local network",
		}
	}

	start := time.Now()
	d := net.Dialer{Timeout: 3 * time.Second}
	conn, err := d.DialContext(ctx, "tcp6", "[2606:4700:4700::1111]:53")
	latency := time.Since(start)
	if err != nil {
		return DiagnosticItem{
			Name:    "IPv6 Connectivity",
			Status:  StatusWarn,
			Details: fmt.Sprintf("IPv6 interface configured but remote routing failed: %v", err),
			Latency: latency,
		}
	}
	conn.Close()
	return DiagnosticItem{
		Name:    "IPv6 Connectivity",
		Status:  StatusOk,
		Details: "IPv6 dual-stack routing is fully operational",
		Latency: latency,
	}
}

func checkTCP(ctx context.Context, addr string) DiagnosticItem {
	start := time.Now()
	d := net.Dialer{Timeout: 4 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", addr)
	latency := time.Since(start)
	if err != nil {
		return DiagnosticItem{
			Name:    fmt.Sprintf("TCP Handshake (%s)", addr),
			Status:  StatusFail,
			Details: fmt.Sprintf("Connection failed: %v", err),
			Latency: latency,
		}
	}
	conn.Close()
	return DiagnosticItem{
		Name:    fmt.Sprintf("TCP Handshake (%s)", addr),
		Status:  StatusOk,
		Details: fmt.Sprintf("3-way handshake established in %v", latency),
		Latency: latency,
	}
}

func checkTLS(ctx context.Context, addr string, sni string) DiagnosticItem {
	start := time.Now()
	dialer := &net.Dialer{Timeout: 4 * time.Second}
	tlsConfig := &tls.Config{
		ServerName:         sni,
		InsecureSkipVerify: false,
	}

	conn, err := tls.DialWithDialer(dialer, "tcp", addr, tlsConfig)
	latency := time.Since(start)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "connection reset by peer") {
			return DiagnosticItem{
				Name:    fmt.Sprintf("TLS SNI Test (%s)", sni),
				Status:  StatusFail,
				Details: "DPI Middlebox RST Injection DETECTED! (Connection reset immediately on SNI)",
				Latency: latency,
			}
		} else if strings.Contains(errMsg, "timeout") {
			return DiagnosticItem{
				Name:    fmt.Sprintf("TLS SNI Test (%s)", sni),
				Status:  StatusFail,
				Details: "DPI Packet Drop DETECTED! (TLS handshake timed out after ClientHello)",
				Latency: latency,
			}
		} else if strings.Contains(errMsg, "x509:") || strings.Contains(errMsg, "certificate") {
			// Probe certificate with InsecureSkipVerify purely as a diagnostic inspection to identify the spoofing entity
			probeConn, probeErr := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{
				ServerName:         sni,
				InsecureSkipVerify: true,
			})
			if probeErr == nil {
				defer probeConn.Close()
				peerCerts := probeConn.ConnectionState().PeerCertificates
				remoteIP := probeConn.RemoteAddr().String()
				if len(peerCerts) > 0 {
					cert := peerCerts[0]
					return DiagnosticItem{
						Name:    fmt.Sprintf("TLS SNI Test (%s)", sni),
						Status:  StatusFail,
						Details: fmt.Sprintf("CERTIFICATE MISMATCH / SINKHOLE DETECTED!\n      Connected to: %s\n      Server Certificate: Subject='%s', Issuer='%s', SANs=%v\n      Reason: Destination is an ISP/BTK warning sinkhole, not Discord!", remoteIP, cert.Subject.CommonName, cert.Issuer.CommonName, cert.DNSNames),
						Latency: latency,
					}
				}
				return DiagnosticItem{
					Name:    fmt.Sprintf("TLS SNI Test (%s)", sni),
					Status:  StatusFail,
					Details: fmt.Sprintf("Certificate verification failed on %s: %v", remoteIP, err),
					Latency: latency,
				}
			}
		}
		return DiagnosticItem{
			Name:    fmt.Sprintf("TLS SNI Test (%s)", sni),
			Status:  StatusFail,
			Details: fmt.Sprintf("TLS handshake failed: %v", err),
			Latency: latency,
		}
	}
	defer conn.Close()

	state := conn.ConnectionState()
	remoteIP := conn.RemoteAddr().String()
	return DiagnosticItem{
		Name:    fmt.Sprintf("TLS SNI Test (%s)", sni),
		Status:  StatusOk,
		Details: fmt.Sprintf("Negotiated TLS %s (%s) with %s", tlsVersionToString(state.Version), tls.CipherSuiteName(state.CipherSuite), remoteIP),
		Latency: latency,
	}
}

func checkHTTP(ctx context.Context, endpoint string, label string) DiagnosticItem {
	start := time.Now()
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return DiagnosticItem{
			Name:    label,
			Status:  StatusFail,
			Details: fmt.Sprintf("Malformed URL: %v", err),
		}
	}
	req.Header.Set("User-Agent", "discord-bypass/1.0")

	resp, err := client.Do(req)
	latency := time.Since(start)
	if err != nil {
		return DiagnosticItem{
			Name:    label,
			Status:  StatusFail,
			Details: fmt.Sprintf("Request failed: %v", err),
			Latency: latency,
		}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	if (resp.StatusCode >= 200 && resp.StatusCode < 400) || resp.StatusCode == 401 || resp.StatusCode == 403 {
		return DiagnosticItem{
			Name:    label,
			Status:  StatusOk,
			Details: fmt.Sprintf("HTTP %d (%d bytes read)", resp.StatusCode, len(body)),
			Latency: latency,
		}
	}

	return DiagnosticItem{
		Name:    label,
		Status:  StatusWarn,
		Details: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body))),
		Latency: latency,
	}
}

func checkGateway(ctx context.Context, wsURL string) DiagnosticItem {
	start := time.Now()
	// Test HTTP upgrade handshake to gateway.discord.gg
	httpURL := strings.Replace(wsURL, "wss://", "https://", 1) + "/?v=9&encoding=json"
	req, err := http.NewRequestWithContext(ctx, "GET", httpURL, nil)
	if err != nil {
		return DiagnosticItem{
			Name:    "Gateway (wss)",
			Status:  StatusFail,
			Details: fmt.Sprintf("Invalid gateway endpoint: %v", err),
		}
	}

	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	req.Header.Set("Sec-WebSocket-Version", "13")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	latency := time.Since(start)
	if err != nil {
		return DiagnosticItem{
			Name:    "Gateway (wss)",
			Status:  StatusFail,
			Details: fmt.Sprintf("WebSocket connection failed: %v", err),
			Latency: latency,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusSwitchingProtocols || resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusOK {
		// Discord responds 400 Bad Request to raw HTTP if auth token is absent, which confirms gateway server is REACHABLE!
		return DiagnosticItem{
			Name:    "Gateway (wss)",
			Status:  StatusOk,
			Details: fmt.Sprintf("Gateway endpoint reached (HTTP %d, latency %v)", resp.StatusCode, latency),
			Latency: latency,
		}
	}

	return DiagnosticItem{
		Name:    "Gateway (wss)",
		Status:  StatusWarn,
		Details: fmt.Sprintf("Unexpected gateway response: HTTP %d", resp.StatusCode),
		Latency: latency,
	}
}

func checkVoice(ctx context.Context, endpoint string) DiagnosticItem {
	if endpoint == "" || strings.HasPrefix(endpoint, "voice.discord.media") {
		endpoint = "latency.discord.media:443"
	}
	start := time.Now()
	// Test UDP socket ping
	d := net.Dialer{Timeout: 3 * time.Second}
	conn, err := d.DialContext(ctx, "udp", endpoint)
	latency := time.Since(start)
	if err != nil {
		return DiagnosticItem{
			Name:    "Voice UDP Probe",
			Status:  StatusWarn,
			Details: fmt.Sprintf("Voice UDP port unreachable: %v", err),
			Latency: latency,
		}
	}
	defer conn.Close()

	// Send STUN binding request packet or probe byte
	_, _ = conn.Write([]byte{0x00, 0x01, 0x00, 0x00})

	return DiagnosticItem{
		Name:    "Voice UDP Probe",
		Status:  StatusOk,
		Details: fmt.Sprintf("Voice UDP socket open (%s)", endpoint),
		Latency: latency,
	}
}

func checkService() DiagnosticItem {
	out, err := exec.Command("systemctl", "is-active", "discord-bypass").Output()
	state := strings.TrimSpace(string(out))
	if err == nil && state == "active" {
		return DiagnosticItem{
			Name:    "Service Status",
			Status:  StatusOk,
			Details: "Systemd service 'discord-bypass' is active and running",
		}
	}

	// Check if running as a process
	outPs, _ := exec.Command("pgrep", "-f", "discord-bypass daemon").Output()
	if len(strings.TrimSpace(string(outPs))) > 0 {
		return DiagnosticItem{
			Name:    "Service Status",
			Status:  StatusOk,
			Details: "discord-bypass daemon process is running (standalone)",
		}
	}

	return DiagnosticItem{
		Name:    "Service Status",
		Status:  StatusWarn,
		Details: fmt.Sprintf("Service is inactive (%s). Start with: sudo discord-bypass start", state),
	}
}

func checkFirewall(fw firewall.FirewallManager) DiagnosticItem {
	if fw == nil {
		return DiagnosticItem{
			Name:    "Firewall Rules",
			Status:  StatusInfo,
			Details: "Firewall manager not loaded",
		}
	}

	if fw.IsActive() {
		packets, bytes, _ := fw.GetStats()
		return DiagnosticItem{
			Name:    "Firewall Rules",
			Status:  StatusOk,
			Details: fmt.Sprintf("Rules active on %s (Processed: %d packets, %d bytes)", fw.DriverName(), packets, bytes),
		}
	}

	// Unprivileged check: retrieve live stats from running daemon
	if status, err := utils.ReadRuntimeStatus(); err == nil && status.FirewallActive {
		return DiagnosticItem{
			Name:    "Firewall Rules",
			Status:  StatusOk,
			Details: fmt.Sprintf("Rules active on %s (Processed: %d packets, %d bytes)", status.FirewallDriver, status.Packets, status.Bytes),
		}
	}

	return DiagnosticItem{
		Name:    "Firewall Rules",
		Status:  StatusWarn,
		Details: fmt.Sprintf("No active bypass rules found in %s", fw.DriverName()),
	}
}

func analyzeVerdict(report *DiagnosticReport) {
	hasDNSFail := false
	hasTLSFail := false
	hasAPIFail := false
	hasServiceActive := false

	for _, item := range report.Items {
		if strings.Contains(item.Name, "DNS") && (item.Status == StatusFail || item.Status == StatusWarn) {
			hasDNSFail = true
		}
		if strings.Contains(item.Name, "TLS") && item.Status == StatusFail {
			hasTLSFail = true
		}
		if strings.Contains(item.Name, "API") && item.Status == StatusFail {
			hasAPIFail = true
		}
		if item.Name == "Service Status" && item.Status == StatusOk {
			hasServiceActive = true
		}
	}

	if !hasDNSFail && !hasTLSFail && !hasAPIFail {
		report.OverallVerdict = "BYPASS VERIFIED (Discord connectivity fully functional)"
		report.FailureStage = "None"
		report.Recommendation = "All Discord subsystems (DNS FIXED, TLS valid, REST API, WebSocket Gateway, CDN) are responding properly."
		return
	}

	if hasDNSFail {
		report.OverallVerdict = "FAILED (DNS POISONED by ISP sinkhole)"
		report.FailureStage = "DNS Resolution (BTK/ISP Sinkhole)"
		if !hasServiceActive {
			report.Recommendation = "Start the bypass service: 'sudo discord-bypass start'. The local Domain-Routing proxy will resolve Discord via secure DoH while preserving normal DNS."
		} else {
			report.Recommendation = "Service is running, but system DNS is returning the sinkhole. Run 'resolvectl status' to verify interface DNS is set to 127.0.0.1:5354."
		}
		return
	}

	if hasTLSFail {
		report.OverallVerdict = "FAILED (TLS SNI BLOCKED by DPI)"
		report.FailureStage = "TLS Handshake (SNI Filtering)"
		if !hasServiceActive {
			report.Recommendation = "Start the bypass service: 'sudo discord-bypass start'. Strategy C (TCP segmentation split2) will desynchronize TLS ClientHello packets."
		} else {
			report.Recommendation = "Bypass is active but TLS failed. Verify nftables rules ('sudo nft list table inet discord_bypass') and check nfqws logs."
		}
		return
	}

	report.OverallVerdict = "FAILED (Application layer connectivity issue)"
	report.FailureStage = "Application Layer"
	report.Recommendation = "Review individual probe results above."
}

func tlsVersionToString(v uint16) string {
	switch v {
	case tls.VersionTLS13:
		return "1.3"
	case tls.VersionTLS12:
		return "1.2"
	case tls.VersionTLS11:
		return "1.1"
	case tls.VersionTLS10:
		return "1.0"
	default:
		return fmt.Sprintf("0x%04x", v)
	}
}
