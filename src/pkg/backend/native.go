package backend

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"discord-bypass/src/pkg/utils"
)

const (
	SO_ORIGINAL_DST = 80
)

type NativeProxy struct {
	port         int
	splitPos     int
	splitDelayMs int
	listener     net.Listener
	mu           sync.Mutex
	isRunning    bool
	cancel       context.CancelFunc
}

func NewNativeProxy(port, splitPos, splitDelayMs int) *NativeProxy {
	if splitPos <= 0 {
		splitPos = 2
	}
	if splitDelayMs <= 0 {
		splitDelayMs = 2
	}
	return &NativeProxy{
		port:         port,
		splitPos:     splitPos,
		splitDelayMs: splitDelayMs,
	}
}

func (p *NativeProxy) Name() string {
	return "native (Go TCP splitter)"
}

func (p *NativeProxy) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.isRunning
}

func (p *NativeProxy) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.isRunning {
		p.mu.Unlock()
		return errors.New("native proxy is already running")
	}

	addr := fmt.Sprintf("127.0.0.1:%d", p.port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		p.mu.Unlock()
		return fmt.Errorf("failed to bind native proxy to %s: %w", addr, err)
	}

	subCtx, cancel := context.WithCancel(ctx)
	p.listener = ln
	p.cancel = cancel
	p.isRunning = true
	p.mu.Unlock()

	utils.Info("Native transparent TCP splitter listening on %s (split_pos=%d, delay=%dms)", addr, p.splitPos, p.splitDelayMs)

	go p.acceptLoop(subCtx)
	return nil
}

func (p *NativeProxy) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.isRunning {
		return nil
	}

	if p.cancel != nil {
		p.cancel()
	}
	if p.listener != nil {
		_ = p.listener.Close()
	}
	p.isRunning = false
	utils.Info("Native transparent TCP splitter stopped")
	return nil
}

func (p *NativeProxy) acceptLoop(ctx context.Context) {
	for {
		conn, err := p.listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				utils.Debug("Accept error: %v", err)
				return
			}
		}

		go p.handleConnection(conn)
	}
}

func (p *NativeProxy) handleConnection(clientConn net.Conn) {
	defer clientConn.Close()

	if tcpConn, ok := clientConn.(*net.TCPConn); ok {
		_ = tcpConn.SetNoDelay(true)
	}

	// 1. Determine original destination address
	targetAddr, err := getOriginalDst(clientConn)
	if err != nil {
		utils.Debug("Failed to get original destination: %v", err)
		return
	}

	// 2. Connect to original destination
	targetConn, err := net.DialTimeout("tcp", targetAddr, 5*time.Second)
	if err != nil {
		utils.Debug("Failed to connect to original destination %s: %v", targetAddr, err)
		return
	}
	defer targetConn.Close()

	if tcpTarget, ok := targetConn.(*net.TCPConn); ok {
		_ = tcpTarget.SetNoDelay(true)
	}

	// 3. Read initial payload from client (TLS ClientHello)
	buf := make([]byte, 4096)
	_ = clientConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err := clientConn.Read(buf)
	if err != nil || n == 0 {
		return
	}
	_ = clientConn.SetReadDeadline(time.Time{})

	initialData := buf[:n]

	// 4. Inspect if TLS Handshake (0x16)
	if len(initialData) > p.splitPos && initialData[0] == 0x16 {
		// Split ClientHello into two TCP segments
		// Part 1: First splitPos bytes (e.g. \x16\x03)
		_, err = targetConn.Write(initialData[:p.splitPos])
		if err != nil {
			return
		}

		// Brief delay to ensure kernel emits packet 1 before packet 2
		time.Sleep(time.Duration(p.splitDelayMs) * time.Millisecond)

		// Part 2: Remainder of ClientHello (contains SNI extension)
		_, err = targetConn.Write(initialData[p.splitPos:])
		if err != nil {
			return
		}
		utils.Debug("Split TLS ClientHello to %s (%d + %d bytes)", targetAddr, p.splitPos, len(initialData)-p.splitPos)
	} else {
		// Non-TLS or too short, send as-is
		_, err = targetConn.Write(initialData)
		if err != nil {
			return
		}
	}

	// 5. Bidirectional copy
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, _ = io.Copy(targetConn, clientConn)
		if tc, ok := targetConn.(*net.TCPConn); ok {
			_ = tc.CloseWrite()
		}
	}()

	go func() {
		defer wg.Done()
		_, _ = io.Copy(clientConn, targetConn)
		if cc, ok := clientConn.(*net.TCPConn); ok {
			_ = cc.CloseWrite()
		}
	}()

	wg.Wait()
}

// getOriginalDst extracts original destination from socket redirected by iptables/nftables
func getOriginalDst(conn net.Conn) (string, error) {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return "", errors.New("not a TCP connection")
	}

	rawConn, err := tcpConn.SyscallConn()
	if err != nil {
		return "", err
	}

	var originalDst string
	var opErr error

	err = rawConn.Control(func(fd uintptr) {
		// Try IPv4 SO_ORIGINAL_DST
		raw := syscall.RawSockaddrInet4{}
		size := uint32(binary.Size(raw))
		_, _, errno := syscall.Syscall6(
			syscall.SYS_GETSOCKOPT,
			fd,
			uintptr(syscall.SOL_IP),
			uintptr(SO_ORIGINAL_DST),
			uintptr(unsafe.Pointer(&raw)),
			uintptr(unsafe.Pointer(&size)),
			0,
		)
		if errno == 0 {
			ip := net.IPv4(raw.Addr[0], raw.Addr[1], raw.Addr[2], raw.Addr[3])
			port := binary.BigEndian.Uint16([]byte{byte(raw.Port), byte(raw.Port >> 8)})
			originalDst = fmt.Sprintf("%s:%d", ip.String(), port)
			return
		}

		opErr = fmt.Errorf("getsockopt SO_ORIGINAL_DST failed: errno %d", errno)
	})

	if err != nil {
		return "", err
	}
	if opErr != nil {
		return "", opErr
	}
	return originalDst, nil
}

