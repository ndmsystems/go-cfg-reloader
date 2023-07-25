package reloader

import (
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
	files     []*fileInfo
	hashMap   map[string]string
	errLogger api.ErrorLoggerFunc

	keys        []*keyInfo
	reloadTime  time.Time
	eDispatcher *eventDispatcher
	watcher     *fsnotify.Watcher
	batchTime   time.Duration
	done        chan struct{}
	mu          sync.Mutex
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
	errLogger api.ErrorLoggerFunc,
) api.CfgReloaderService {

	s := &svc{
		files:       make([]*fileInfo, len(files)),
		errLogger:   errLogger,
		keys:        make([]*keyInfo, 0),
		eDispatcher: newEventDispatcher(),
		hashMap:     make(map[string]string),
		done:        make(chan struct{}),
		batchTime:   batchTime,
	}

	for i, filename := range files {
		s.files[i] = &fileInfo{filename: filename}
	}

	return s
}

// KeyAdd - adds a key
// allows more than one cb on one key
// no way to delete keys
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
func (s *svc) Start() error {
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
		var eventBatch []fsnotify.Event
		var timer <-chan time.Time
		for {
			select {
			case <-s.done:
				return
			default:
			}

			select {
			case <-s.done:
				return
			case event, ok := <-s.watcher.Events:
				if !ok {
					return
				}
				if len(eventBatch) == 0 {
					timer = time.After(s.batchTime)
				}
				eventBatch = append(eventBatch, event)
			case <-timer:
				needReload := false
				for _, event := range eventBatch {
					if _, ok := filesMap[event.Name]; ok && event.Op&eventMask > 0 {
						needReload = true
						s.eDispatcher.push(fmt.Sprintf("%s config file (%s)", event.Op.String(), event.Name))
					}
				}
				if needReload {
					if err := s.parse(false); err != nil {
						if !errors.Is(err, errNotModified) {
							s.errLogger(err)
						}
					}
				}
				timer = nil
				eventBatch = eventBatch[:0]
			case err, ok := <-s.watcher.Errors:
				if !ok {
					return
				}
				s.errLogger(err)
			}
		}
	}()

	return nil
}

// Stop ...
func (s *svc) Stop() {
	close(s.done)
	s.eDispatcher.stop()
	_ = s.watcher.Close()
}

// ForceReload ...
func (s *svc) ForceReload(reason string) error {
	if err := s.parse(true); err != nil {
		return fmt.Errorf("couldn't reload config: %v", err)
	}
	s.eDispatcher.push(reason)

	return nil
}

// ReloadTime ...
func (s *svc) ReloadTime() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reloadTime
}

// Events ...
func (s *svc) Events() <-chan api.Event {
	return s.eDispatcher.getEventsChan()
}
