package api

import (
	"context"
	"encoding/json"
	"time"
)

// Logger is the interface service uses for logging
type Logger interface {
	Info(...interface{})
	Error(...interface{})
}

// CallbackFunc is a function called by service to get object to store config
type CallbackFunc func(key string, data json.RawMessage)

// CfgReloaderService ...
type CfgReloaderService interface {
	KeyAdd(key string, fnCallback CallbackFunc) error
	Start(ctx context.Context) error
	ForceReload() error
	ReloadTime() time.Time
}
