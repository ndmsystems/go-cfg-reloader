package api

import (
	"encoding/json"
	"time"
)

// ErrorLoggerFunc is a function to log error in service
type ErrorLoggerFunc func(error)

// CallbackFunc is a function called by service to get object to store config
type CallbackFunc func(key string, data json.RawMessage)

// CfgReloaderService ...
type CfgReloaderService interface {
	KeyAdd(key string, fnCallback CallbackFunc) error
	Start() error
	Stop()
	// info
	ReloadTime() time.Time
	Events() <-chan Event
}

// Event - the config reloader event
type Event struct {
	Time   time.Time
	Reason string
}
