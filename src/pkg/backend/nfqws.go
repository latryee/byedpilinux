package backend

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"syscall"
	"time"

	"discord-bypass/src/pkg/config"
	"discord-bypass/src/pkg/utils"
)

type NFQWSBackend struct {
	cfg       *config.Config
	cmd       *exec.Cmd
	done      chan struct{}
	mu        sync.Mutex
	isRunning bool
	cancel    context.CancelFunc
}

func NewNFQWSBackend(cfg *config.Config) *NFQWSBackend {
	return &NFQWSBackend{
		cfg: cfg,
	}
}

func (b *NFQWSBackend) Name() string {
	return "nfqws (Netfilter queue)"
}

func (b *NFQWSBackend) IsRunning() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.isRunning
}

func (b *NFQWSBackend) Start(ctx context.Context) error {
	b.mu.Lock()
	if b.isRunning {
		b.mu.Unlock()
		return errors.New("nfqws backend is already running")
	}

	binPath := b.cfg.NFQWS.BinaryPath
	if _, err := exec.LookPath(binPath); err != nil {
		// Fallback check in local build dir
		localBin := "./bin/discord-bypass-nfqws"
		if _, err2 := os.Stat(localBin); err2 == nil {
			binPath = localBin
		} else {
			b.mu.Unlock()
			return fmt.Errorf("nfqws binary not found at %s or %s (run 'make build' or 'discord-bypass install')", binPath, localBin)
		}
	}

	subCtx, cancel := context.WithCancel(ctx)
	b.cancel = cancel
	b.done = make(chan struct{})
	b.isRunning = true
	b.mu.Unlock()

	go b.supervisorLoop(subCtx, binPath)
	return nil
}

func (b *NFQWSBackend) Stop() error {
	b.mu.Lock()
	if !b.isRunning {
		b.mu.Unlock()
		return nil
	}

	b.isRunning = false
	cmd, done, cancel := b.cmd, b.done, b.cancel
	b.mu.Unlock()
	if cancel != nil {
		cancel()
	}

	// nfqws handles SIGTERM/SIGINT. Give it a bounded graceful-stop window,
	// then force termination so systemd never leaves an orphan behind.
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Signal(syscall.SIGTERM)
	}
	if done != nil {
		select {
		case <-done:
		case <-time.After(time.Second):
			if cmd != nil && cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			<-done
		}
	}
	utils.Info("nfqws backend stopped")
	return nil
}

func (b *NFQWSBackend) buildArgs() []string {
	args := []string{
		"--qnum=" + strconv.Itoa(b.cfg.Firewall.QueueNum),
		// Confirmed in zapret v72.13 (Linux default is 0x40000000). This
		// mark is paired with the nftables bypass rule before NFQUEUE.
		"--dpi-desync-fwmark=0x40000000",
	}

	// Hostlist: only desync traffic for targeted domains!
	if b.cfg.General.DomainsFile != "" {
		if _, err := os.Stat(b.cfg.General.DomainsFile); err == nil {
			args = append(args, "--hostlist="+b.cfg.General.DomainsFile)
		}
	}

	// Strategy specific args
	switch b.cfg.General.Strategy {
	case "strategy_d":
		args = append(args, b.cfg.NFQWS.StrategyDArgs...)
	case "strategy_c":
		fallthrough
	default:
		args = append(args, b.cfg.NFQWS.StrategyCArgs...)
	}

	return args
}

// BuildArgs returns the slice of command-line arguments that will be passed to nfqws.
func (b *NFQWSBackend) BuildArgs() []string {
	return b.buildArgs()
}

func (b *NFQWSBackend) supervisorLoop(ctx context.Context, binPath string) {
	defer close(b.done)
	consecutiveFailures := 0
	maxFailures := 5

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		b.mu.Lock()
		if !b.isRunning {
			b.mu.Unlock()
			return
		}

		args := b.buildArgs()
		cmd := exec.Command(binPath, args...)
		b.cmd = cmd
		b.mu.Unlock()

		stdoutPipe, _ := cmd.StdoutPipe()
		stderrPipe, _ := cmd.StderrPipe()

		utils.Info("Starting nfqws engine: %s %s", binPath, args)
		startTime := time.Now()

		if err := cmd.Start(); err != nil {
			utils.Error("Failed to launch nfqws: %v", err)
			consecutiveFailures++
			if consecutiveFailures >= maxFailures {
				utils.Error("nfqws exceeded maximum crash limit (%d times). Stopping supervisor.", maxFailures)
				b.mu.Lock()
				b.isRunning = false
				b.mu.Unlock()
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
			continue
		}

		// Read output streams
		go func() {
			if stdoutPipe != nil {
				scanner := bufio.NewScanner(stdoutPipe)
				for scanner.Scan() {
					utils.Debug("[nfqws] %s", scanner.Text())
				}
			}
		}()

		go func() {
			if stderrPipe != nil {
				scanner := bufio.NewScanner(stderrPipe)
				for scanner.Scan() {
					utils.Debug("[nfqws err] %s", scanner.Text())
				}
			}
		}()

		err := cmd.Wait()

		// If process ran for more than 10 seconds, reset failure counter
		if time.Since(startTime) > 10*time.Second {
			consecutiveFailures = 0
		}

		b.mu.Lock()
		running := b.isRunning
		b.mu.Unlock()

		if !running {
			return
		}

		utils.Warn("nfqws process exited unexpectedly: %v. Restarting...", err)
		consecutiveFailures++
		if consecutiveFailures >= maxFailures {
			utils.Error("nfqws crashed %d times in rapid succession. Halting to avoid infinite loop.", maxFailures)
			b.mu.Lock()
			b.isRunning = false
			b.mu.Unlock()
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}
