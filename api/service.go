package api

import (
	"encoding/json"
	"time"
)

//
// ErrorLogger is a function to log error in service
//
type ErrorLoggerFunc func(error)

//
// KeyFactory is a function called by service to get object to store config
//
type CallbackFunc func(key string, data json.RawMessage)

//
// CfgReloaderService ...
//
type CfgReloaderService interface {
	KeyAdd(key string, fnCallback CallbackFunc) error
	Start() error
	// info
	ReloadTime() time.Time
}
