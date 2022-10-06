package reloader

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/ndmsystems/go-cfg-reloader/api"
)

type svc struct {
	files     []*fileInfo
	errLogger api.ErrorLoggerFunc

	// keys       map[string]*keyInfo
	keys       []*keyInfo
	reloadTime time.Time
}

type keyInfo struct {
	name       string
	fnCallBack api.CallbackFunc
	orig       json.RawMessage // raw key data
}

type fileInfo struct {
	filename string
	exists   bool
	modTime  *time.Time
}

const (
	tag = "[CFG-RELOADER]:"
	sep = "----------------------------------------------------------------"
)

var (
	errKeyCallbackIsNil = errors.New("key callback function is nil")
)

// New return service object
func New(
	files []string,
	errLogger api.ErrorLoggerFunc) api.CfgReloaderService {

	s := &svc{
		files:     make([]*fileInfo, len(files)),
		errLogger: errLogger,
		keys:      make([]*keyInfo, 0),
		// keys:      make(map[string]*keyInfo),
	}

	for i, filename := range files {
		s.files[i] = &fileInfo{filename: filename}
	}

	return s
}

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

func (s *svc) Start() error {

	if err := s.parse(); err != nil {
		return err
	}
	s.reloadTime = time.Now()

	go func() {
		for {
			time.Sleep(30 * time.Second)

			if err := s.parse(); err != nil {
				s.errLogger(err)
				continue
			}
			s.reloadTime = time.Now()
		}
	}()

	return nil
}

func (s *svc) ReloadTime() time.Time {
	return s.reloadTime
}
