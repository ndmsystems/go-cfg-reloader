package reloader

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Logger is the interface service uses for logging
type Logger interface {
	Info(...interface{})
	Error(...interface{})
}

// CallbackFunc is a func called on config changed
type CallbackFunc[T any] func(oldConfig, curConfig T)

// ConfigReloader - the config reloader service
type ConfigReloader[T any] struct {
	files      []*fileInfo
	logger     Logger
	curConfig  T
	callbacks  []CallbackFunc[T]
	reloadTime time.Time
	watcher    *fsnotify.Watcher
	batchTime  time.Duration
	mu         sync.RWMutex
}

// fileInfo - represents config file information
type fileInfo struct {
	filename string
}

var (
	errKeyCallbackIsNil = errors.New("key callback function is nil")
)

// New return service object
func New[T any](
	files []string,
	batchTime time.Duration,
	logger Logger,
) *ConfigReloader[T] {

	s := &ConfigReloader[T]{
		files:     make([]*fileInfo, len(files)),
		logger:    logger,
		batchTime: batchTime,
	}

	for i, filename := range files {
		s.files[i] = &fileInfo{filename: filename}
	}

	return s
}

func (s *ConfigReloader[T]) Subscribe(cb CallbackFunc[T]) error {

	if cb == nil {
		return errKeyCallbackIsNil
	}

	s.callbacks = append(s.callbacks, cb)

	return nil
}

// Start ...
func (s *ConfigReloader[T]) Start(ctx context.Context) error {
	var err error
	// first time parse config
	if err = s.reloadConfig(true); err != nil {
		return err
	}

	filesMap := make(map[string]struct{}, len(s.files))
	dirsMap := make(map[string]struct{}, len(s.files))
	for _, cfg := range s.files {
		filesMap[cfg.filename] = struct{}{}
		dirsMap[filepath.Dir(cfg.filename)] = struct{}{}
	}

	// init file watcher
	s.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	// add config directories to watcher
	for d := range dirsMap {
		if e := s.watcher.Add(d); e != nil {
			return e
		}
	}

	// events that we'll catch
	eventMask := fsnotify.Create | fsnotify.Write | fsnotify.Remove | fsnotify.Rename

	// catch filesystem events, and reload config if any config file was changed
	go func() {
		defer s.stop()

		var timer <-chan time.Time
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			select {
			case <-ctx.Done():
				return
			case event, ok := <-s.watcher.Events:
				if !ok {
					return
				}
				// eventMask contains bits we interested in
				// if we do bitwise "and" of eventMask and one of that bits result will be > 0
				// otherwise 0
				if _, ok := filesMap[event.Name]; !ok || event.Op&eventMask == 0 {
					continue
				}
				s.logger.Info(fmt.Sprintf("%s config file (%s)", event.Op.String(), event.Name))
				if timer == nil {
					timer = time.After(s.batchTime)
				}
			case <-timer:
				if err := s.reloadConfig(false); err != nil {
					s.logger.Error(err)
				}
				timer = nil
			case err, ok := <-s.watcher.Errors:
				if !ok {
					return
				}
				s.logger.Error(err)
			}
		}
	}()

	return nil
}

// stop ...
func (s *ConfigReloader[T]) stop() {
	_ = s.watcher.Close()
}

// ForceReload ...
func (s *ConfigReloader[T]) ForceReload() error {
	if err := s.reloadConfig(true); err != nil {
		return fmt.Errorf("couldn't reload config: %v", err)
	}

	return nil
}

// ReloadTime returns last time config was changed
func (s *ConfigReloader[T]) ReloadTime() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.reloadTime
}

// Config returns current config
// should not be used in callback (deadlock)
func (s *ConfigReloader[T]) Config() T {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.curConfig
}
