package reloader

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

type jcfg map[string]json.RawMessage
type cfgBuf map[string]interface{}

var (
	errInvalidConfigJSON = errors.New("parseData: invalid json")
	errNotModified       = errors.New("config not modified")
)

func (s *svc) parse(forceReload bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reloadTime = time.Now()
	filesChanged := false
	fileDataMap := make(map[string][]byte, len(s.files))
	hashMap := make(map[string]string, len(s.files))
	for _, cfg := range s.files {
		data, hash, err := s.getDataAndHash(cfg.filename)
		if err != nil {
			return err
		}
		oldHash, ok := s.hashMap[cfg.filename]

		if ok && hash != oldHash || (!ok || oldHash == "") && hash != "" {
			filesChanged = true
		}

		fileDataMap[cfg.filename] = data
		hashMap[cfg.filename] = hash
	}

	if !filesChanged && !forceReload {
		// if files not changed and no need to force reload
		return errNotModified
	}

	buf := make(cfgBuf)

	// merge all files into buf
	for _, cfg := range s.files {
		data := fileDataMap[cfg.filename]
		if len(data) == 0 {
			continue
		}
		err := s.mergeCfgFromBuf(buf, data)
		if err != nil {
			return fmt.Errorf(
				"%s failed to process config: %s: %v", tag, cfg.filename, err)
		}
	}

	fullCfg, err := json.MarshalIndent(buf, "", "    ")
	if err != nil {
		return fmt.Errorf("%s json.Marshal failed: %v", tag, err)
	}

	if err := s.parseData(fullCfg); err != nil {
		return fmt.Errorf("%s parse failed: %v", tag, err)
	}

	s.hashMap = hashMap

	return nil
}

func (s *svc) mergeCfgFromBuf(buf cfgBuf, data []byte) error {

	jc := new(jcfg)
	err := json.Unmarshal(data, jc)
	if err != nil {
		return err
	}

	for k, v := range *jc {
		var data interface{}
		err = json.Unmarshal(v, &data)
		if err != nil {
			return err
		}

		s.mergeData(buf, k, data)
	}

	return nil
}

func (s *svc) mergeData(buf cfgBuf, key string, data interface{}) {

	switch data := data.(type) {
	case map[string]interface{}:
		if _, ok := buf[key]; !ok {
			buf[key] = make(cfgBuf)
		}
		for k, v := range data {
			s.mergeData((buf[key]).(cfgBuf), k, v)
		}

	case []interface{}:
		v, ok := buf[key]
		if !ok {
			v = make([]interface{}, 0)
		}
		for _, v2 := range data {
			v = append((v).([]interface{}), v2)
		}
		buf[key] = v

	default:
		buf[key] = data
	}
}

func (s *svc) parseData(data []byte) error {

	if !json.Valid(data) {
		return errInvalidConfigJSON
	}

	var jc jcfg
	if err := json.Unmarshal(data, &jc); err != nil {
		return err
	}

	for i := range s.keys {
		key := s.keys[i]
		if raw, ok := jc[key.name]; ok {
			if !bytes.Equal(key.orig, raw) {
				key.fnCallBack(key.name, raw)
				key.orig = raw
			}
		}
	}

	return nil
}

func (s *svc) getDataAndHash(fName string) ([]byte, string, error) {
	if _, err := os.Stat(fName); errors.Is(err, os.ErrNotExist) {
		return []byte{}, "", nil
	}

	data, err := os.ReadFile(fName)
	if err != nil {
		return nil, "", err
	}
	hash := fmt.Sprintf("%x", md5.Sum(data))
	return data, hash, nil
}
