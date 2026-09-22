package dns

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"discord-bypass/src/pkg/utils"
)

type DNSProxy struct {
	listenAddr string
	dohURL     string
	preferIPv4 bool
	udpConn    *net.UDPConn
	tcpLn      net.Listener
	mu         sync.Mutex
	isRunning  bool
	cancel     context.CancelFunc
}

func NewDNSProxy(listenAddr string, dohProvider string, preferIPv4 bool) *DNSProxy {
	if listenAddr == "" {
		listenAddr = "127.0.0.1:5354"
	}
	url, ok := ProviderURLs[strings.ToLower(dohProvider)]
	if !ok {
		url = ProviderURLs["cloudflare"]
	}
	return &DNSProxy{
		listenAddr: listenAddr,
		dohURL:     url,
		preferIPv4: preferIPv4,
	}
}

func (p *DNSProxy) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.isRunning {
		p.mu.Unlock()
		return nil
	}

	uAddr, err := net.ResolveUDPAddr("udp", p.listenAddr)
	if err != nil {
		p.mu.Unlock()
		return err
	}

	uConn, err := net.ListenUDP("udp", uAddr)
	if err != nil {
		p.mu.Unlock()
		return fmt.Errorf("failed to bind local DNS proxy UDP on %s: %w", p.listenAddr, err)
	}

	tLn, err := net.Listen("tcp", p.listenAddr)
	if err != nil {
		uConn.Close()
		p.mu.Unlock()
		return fmt.Errorf("failed to bind local DNS proxy TCP on %s: %w", p.listenAddr, err)
	}

	subCtx, cancel := context.WithCancel(ctx)
	p.udpConn = uConn
	p.tcpLn = tLn
	p.cancel = cancel
	p.isRunning = true
	p.mu.Unlock()

	utils.Info("Local Secure DNS Proxy listening on %s (forwarding via %s)", p.listenAddr, p.dohURL)

	go p.udpLoop(subCtx)
	go p.tcpLoop(subCtx)
	return nil
}

func (p *DNSProxy) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.isRunning {
		return
	}
	p.isRunning = false
	if p.cancel != nil {
		p.cancel()
	}
	if p.udpConn != nil {
		_ = p.udpConn.Close()
	}
	if p.tcpLn != nil {
		_ = p.tcpLn.Close()
	}
	utils.Info("Local Secure DNS Proxy stopped")
}

func (p *DNSProxy) udpLoop(ctx context.Context) {
	buf := make([]byte, 4096)
	client := &http.Client{Timeout: 5 * time.Second}

	for {
		n, remoteAddr, err := p.udpConn.ReadFrom(buf)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				return
			}
		}

		queryData := make([]byte, n)
		copy(queryData, buf[:n])

		go func(q []byte, rAddr net.Addr) {
			resp, err := p.forwardDoH(ctx, client, q)
			if err == nil && len(resp) > 0 {
				_, _ = p.udpConn.WriteTo(resp, rAddr)
			}
		}(queryData, remoteAddr)
	}
}

func (p *DNSProxy) tcpLoop(ctx context.Context) {
	client := &http.Client{Timeout: 5 * time.Second}

	for {
		conn, err := p.tcpLn.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				return
			}
		}

		go func(c net.Conn) {
			defer c.Close()
			lenBuf := make([]byte, 2)
			if _, err := io.ReadFull(c, lenBuf); err != nil {
				return
			}
			msgLen := int(lenBuf[0])<<8 | int(lenBuf[1])
			queryData := make([]byte, msgLen)
			if _, err := io.ReadFull(c, queryData); err != nil {
				return
			}

			resp, err := p.forwardDoH(ctx, client, queryData)
			if err == nil && len(resp) > 0 {
				respLenBuf := []byte{byte(len(resp) >> 8), byte(len(resp))}
				_, _ = c.Write(respLenBuf)
				_, _ = c.Write(resp)
			}
		}(conn)
	}
}

func (p *DNSProxy) forwardDoH(ctx context.Context, client *http.Client, queryData []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", p.dohURL, bytes.NewReader(queryData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/dns-message")
	req.Header.Set("Accept", "application/dns-message")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DoH status: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
