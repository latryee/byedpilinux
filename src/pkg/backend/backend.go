package backend

import (
	"context"
	"fmt"

	"discord-bypass/src/pkg/config"
	"discord-bypass/src/pkg/utils"
)

type Backend interface {
	Start(ctx context.Context) error
	Stop() error
	Name() string
	IsRunning() bool
}

func NewBackend(cfg *config.Config, sys *utils.SystemInfo) (Backend, error) {
	switch cfg.General.Backend {
	case "nfqws":
		return NewNFQWSBackend(cfg), nil
	case "native":
		return NewNativeProxy(cfg.Firewall.ProxyPort, cfg.Native.SplitPos, cfg.Native.SplitDelayMs), nil
	case "byedpi":
		return NewByeDPIBackend(cfg), nil
	default:
		return nil, fmt.Errorf("unknown backend: %s", cfg.General.Backend)
	}
}
