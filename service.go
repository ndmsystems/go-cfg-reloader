package reloader

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/ndmsystems/go-cfg-reloader/api"

	"github.com/fsnotify/fsnotify"
)

// svc - the config reloader service
type svc struct {
	files   []*fileInfo
	hashMap map[string]string
	logger  api.Logger

	keys       []*keyInfo
	reloadTime time.Time
	watcher    *fsnotify.Watcher
	batchTime  time.Duration
	mu         sync.Mutex
}

// keyInfo represents information according to json first level keys (name, parser functions etc.)
type keyInfo struct {
	name       string
	fnCallBack api.CallbackFunc
	orig       json.RawMessage // raw key data
}

// fileInfo - represents config file information
type fileInfo struct {
	filename string
}

const (
	tag = "[CFG-RELOADER]:"
)

var (
	errKeyCallbackIsNil = errors.New("key callback function is nil")
)

// New return service object
func New(
	files []string,
	batchTime time.Duration,
	logger api.Logger,
) api.CfgReloaderService {

	s := &svc{
		files:     make([]*fileInfo, len(files)),
		logger:    logger,
		keys:      make([]*keyInfo, 0),
		hashMap:   make(map[string]string),
		batchTime: batchTime,
	}

	for i, filename := range files {
		s.files[i] = &fileInfo{filename: filename}
	}

	return s
}

// KeyAdd - adds a key
// allows more than one cb on one key
// no way to delete keys
// if key was deleted fnCallback will be called with len(data) == 0
func (s *svc) KeyAdd(key string, fnCallBack api.CallbackFunc) error {

	if fnCallBack == nil {
		return errKeyCallbackIsNil
	}

	s.keys = append(s.keys, &keyInfo{
		name:       key,
		fnCallBack: fnCallBack,
	})

	return nil
}

// Start ...
func (s *svc) Start(ctx context.Context) error {
	var err error
	// first time parse config
	if err = s.parse(true); err != nil {
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
				if err := s.parse(false); err != nil {
					if !errors.Is(err, errNotModified) {
						s.logger.Error(err)
					}
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

// Stop ...
func (s *svc) stop() {
	_ = s.watcher.Close()
}

// ForceReload ...
func (s *svc) ForceReload() error {
	if err := s.parse(true); err != nil {
		return fmt.Errorf("couldn't reload config: %v", err)
	}

	return nil
}

// ReloadTime ...
func (s *svc) ReloadTime() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reloadTime
}
