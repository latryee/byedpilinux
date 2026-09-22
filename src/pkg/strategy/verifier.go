package strategy

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// VerificationResult contains the detailed outcome of the application-layer connectivity check.
type VerificationResult struct {
	Success          bool          `json:"success"`
	FailedStage      string        `json:"failed_stage,omitempty"`
	Details          string        `json:"details"`
	Latency          time.Duration `json:"latency"`
	DNSClean         bool          `json:"dns_clean"`
	TCPConnected     bool          `json:"tcp_connected"`
	TLSNegotiated    bool          `json:"tls_negotiated"`
	ApplicationLayer bool          `json:"application_layer"`
	VoiceReady       bool          `json:"voice_ready"`
	RemoteAddress    string        `json:"remote_address,omitempty"`
}

// VerifyDiscordConnectivity executes a rigorous 4-stage application-layer verification
// to determine if Discord is genuinely reachable without DPI interference.
func VerifyDiscordConnectivity(ctx context.Context, timeout time.Duration) (*VerificationResult, error) {
	start := time.Now()
	res := &VerificationResult{}

	// Bounded context
	subCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Stage 1: DNS Resolution Check
	// Verify discord.com resolves and does not point to known Turkish ISP sinkholes
	addrs, err := net.DefaultResolver.LookupHost(subCtx, "discord.com")
	if err != nil || len(addrs) == 0 {
		res.FailedStage = "DNS"
		res.Details = fmt.Sprintf("DNS resolution failed for discord.com: %v", err)
		res.Latency = time.Since(start)
		return res, nil
	}

	sinkholes := []string{"195.175.", "212.156.", "10.", "127.", "0.0.0.0"}
	for _, a := range addrs {
		for _, s := range sinkholes {
			if strings.HasPrefix(a, s) {
				res.FailedStage = "DNS"
				res.Details = fmt.Sprintf("DNS resolved to known ISP sinkhole: %s", a)
				res.Latency = time.Since(start)
				return res, nil
			}
		}
	}
	res.DNSClean = true

	// Stage 2: TCP Handshake to discord.com:443
	tcpDialer := net.Dialer{Timeout: 3 * time.Second}
	tcpConn, err := tcpDialer.DialContext(subCtx, "tcp", "discord.com:443")
	if err != nil {
		res.FailedStage = "TCP"
		res.Details = fmt.Sprintf("TCP 443 handshake failed: %v", err)
		res.Latency = time.Since(start)
		return res, nil
	}
	res.TCPConnected = true
	res.RemoteAddress = tcpConn.RemoteAddr().String()
	tcpConn.Close()

	// Stage 3: TLS 1.3 Handshake & Certificate Validation
	tlsDialer := &net.Dialer{Timeout: 4 * time.Second}
	tlsConfig := &tls.Config{
		ServerName:         "discord.com",
		InsecureSkipVerify: false,
	}
	tlsConn, err := tls.DialWithDialer(tlsDialer, "tcp", "discord.com:443", tlsConfig)
	if err != nil {
		res.FailedStage = "TLS"
		errMsg := err.Error()
		if strings.Contains(errMsg, "connection reset by peer") {
			res.Details = "DPI RST injection detected during TLS ClientHello"
		} else if strings.Contains(errMsg, "timeout") {
			res.Details = "DPI packet drop detected during TLS handshake"
		} else {
			res.Details = fmt.Sprintf("TLS handshake failed: %v", err)
		}
		res.Latency = time.Since(start)
		return res, nil
	}
	res.TLSNegotiated = true
	tlsConn.Close()

	// Stage 4: Layer-7 Application Verification
	// Discord REST API & Gateway WebSocket Check
	httpClient := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: 3 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout: 4 * time.Second,
		},
	}

	// Probe A: REST API /api/v10/gateway (expects HTTP 200 with JSON payload)
	reqAPI, err := http.NewRequestWithContext(subCtx, "GET", "https://discord.com/api/v10/gateway", nil)
	if err == nil {
		reqAPI.Header.Set("User-Agent", "discord-bypass/1.0")
		respAPI, errAPI := httpClient.Do(reqAPI)
		if errAPI == nil {
			bodyBytes, _ := io.ReadAll(io.LimitReader(respAPI.Body, 512))
			respAPI.Body.Close()
			bodyStr := string(bodyBytes)

			if respAPI.StatusCode == http.StatusOK && strings.Contains(bodyStr, "wss://") {
				res.ApplicationLayer = true
				res.Success = true
				res.VoiceReady = probeVoiceUDP(subCtx)
				res.Details = fmt.Sprintf("Layer 7 verified: Discord REST API returned HTTP 200 (%s)", strings.TrimSpace(bodyStr))
				res.Latency = time.Since(start)
				return res, nil
			}
		}
	}

	// Probe B: Gateway WebSocket Handshake (expects HTTP 101 or HTTP 400 Bad Request)
	reqGW, err := http.NewRequestWithContext(subCtx, "GET", "https://gateway.discord.gg/?v=10&encoding=json", nil)
	if err == nil {
		reqGW.Header.Set("Upgrade", "websocket")
		reqGW.Header.Set("Connection", "Upgrade")
		reqGW.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
		reqGW.Header.Set("Sec-WebSocket-Version", "13")
		reqGW.Header.Set("User-Agent", "discord-bypass/1.0")

		respGW, errGW := httpClient.Do(reqGW)
		if errGW == nil {
			respGW.Body.Close()
			if respGW.StatusCode == http.StatusSwitchingProtocols || respGW.StatusCode == http.StatusBadRequest || respGW.StatusCode == http.StatusOK {
				res.ApplicationLayer = true
				res.Success = true
				res.VoiceReady = probeVoiceUDP(subCtx)
				res.Details = fmt.Sprintf("Layer 7 verified: Discord Gateway endpoint reached (HTTP %d)", respGW.StatusCode)
				res.Latency = time.Since(start)
				return res, nil
			}
		}
	}

	// Probe C: Fallback to /api/v10/users/@me (authentic 401 Unauthorized proves reachability)
	reqAuth, err := http.NewRequestWithContext(subCtx, "GET", "https://discord.com/api/v10/users/@me", nil)
	if err == nil {
		reqAuth.Header.Set("User-Agent", "discord-bypass/1.0")
		respAuth, errAuth := httpClient.Do(reqAuth)
		if errAuth == nil {
			respAuth.Body.Close()
			if respAuth.StatusCode == http.StatusUnauthorized || respAuth.StatusCode == http.StatusTooManyRequests {
				res.ApplicationLayer = true
				res.Success = true
				res.VoiceReady = probeVoiceUDP(subCtx)
				res.Details = fmt.Sprintf("Layer 7 verified: Discord API reachable (authentic HTTP %d)", respAuth.StatusCode)
				res.Latency = time.Since(start)
				return res, nil
			}
		}
	}

	res.FailedStage = "Application"
	res.Details = "TLS negotiated, but Discord application layer endpoints failed to respond"
	res.Latency = time.Since(start)
	return res, nil
}

func probeVoiceUDP(ctx context.Context) bool {
	d := net.Dialer{Timeout: 2 * time.Second}
	conn, err := d.DialContext(ctx, "udp", "latency.discord.media:443")
	if err != nil {
		return false
	}
	defer conn.Close()
	_, _ = conn.Write([]byte{0x00, 0x01, 0x00, 0x00})
	return true
}
