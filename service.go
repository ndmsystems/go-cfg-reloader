package reloader

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// CallbackFunc is a func called on config changed
type CallbackFunc[T any] func(oldConfig, curConfig T)

// ConfigReloader - the config reloader service
type ConfigReloader[T any] struct {
	files     []*fileInfo
	batchTime time.Duration
	watcher   *fsnotify.Watcher

	mu         sync.RWMutex // guards following fields
	curConfig  T
	callbacks  []CallbackFunc[T]
	reloadTime time.Time
	onError    func(error)
}

// fileInfo - represents config file information
type fileInfo struct {
	filename string
}

var (
	errCallbackIsNil = errors.New("callback function is nil")
)

// New loads config from files and returns service object
func New[T any](files []string, batchTime time.Duration) (*ConfigReloader[T], error) {

	s := &ConfigReloader[T]{
		files:     make([]*fileInfo, len(files)),
		batchTime: batchTime,
	}

	for i, filename := range files {
		s.files[i] = &fileInfo{filename: filename}
	}

	if err := s.reloadConfig(); err != nil {
		return nil, err
	}

	return s, nil
}

// Subscribe registers a callback that's called with the old and new config whenever the config is reloaded
func (s *ConfigReloader[T]) Subscribe(cb CallbackFunc[T]) error {
	if cb == nil {
		return errCallbackIsNil
	}

	s.callbacks = append(s.callbacks, cb)

	return nil
}

// Start begins watching the config files for changes in the background
func (s *ConfigReloader[T]) Start(ctx context.Context) error {
	var err error

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
			if os.IsNotExist(e) {
				continue
			}
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
				if timer == nil {
					timer = time.After(s.batchTime)
				}
			case <-timer:
				if err := s.reloadConfig(); err != nil {
					s.notifyError(err)
				}
				timer = nil
			case err, ok := <-s.watcher.Errors:
				if !ok {
					return
				}
				s.notifyError(err)
			}
		}
	}()

	return nil
}

// stop closes the file watcher, ending the background goroutine started by Start.
func (s *ConfigReloader[T]) stop() {
	_ = s.watcher.Close()
}

// notifyError calls the OnError handler, if one is set
func (s *ConfigReloader[T]) notifyError(err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.onError != nil {
		s.onError(err)
	}
}

// ForceReload reloads the config from files and calls the callbacks
func (s *ConfigReloader[T]) ForceReload() error {
	if err := s.reloadConfig(); err != nil {
		return fmt.Errorf("couldn't reload config: %w", err)
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

// OnError sets the error handler function
func (s *ConfigReloader[T]) OnError(fn func(error)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.onError = fn
}
