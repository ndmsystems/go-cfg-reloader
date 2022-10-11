package reloader

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
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

	done chan struct{}
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
	errLogger api.ErrorLoggerFunc) api.CfgReloaderService {

	s := &svc{
		files:       make([]*fileInfo, len(files)),
		errLogger:   errLogger,
		keys:        make([]*keyInfo, 0),
		eDispatcher: newEventDispatcher(),
		hashMap:     make(map[string]string),
		done:        make(chan struct{}),
	}

	for i, filename := range files {
		s.files[i] = &fileInfo{filename: filename}
	}

	return s
}

// KeyAdd - adds a key
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
	if err = s.parse(); err != nil {
		return err
	}
	s.reloadTime = time.Now()

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

				if _, ok := filesMap[event.Name]; ok && event.Op&eventMask > 0 {
					if err := s.parse(); err != nil {
						if !errors.Is(err, errNotModified) {
							s.errLogger(err)
						}
						continue
					}
					s.eDispatcher.push(fmt.Sprintf("%s config file (%s)", event.Op.String(), event.Name))
					s.reloadTime = time.Now()
				}
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

// ReloadTime ...
func (s *svc) ReloadTime() time.Time {
	return s.reloadTime
}

// Events ...
func (s *svc) Events() <-chan api.Event {
	return s.eDispatcher.getEventsChan()
}
