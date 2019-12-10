package cfg

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
)

type jcfg map[string]json.RawMessage
type cfgBuf map[string]interface{}

var (
	errInvalidConfigJSON = errors.New("parseData: invalid json")
)

func (s *svc) parse() error {

	filesChanged := false
	for _, cfg := range s.files {
		fi, err := os.Lstat(cfg.filename)
		if err != nil {
			if os.IsNotExist(err) {
				cfg.exists = false
				continue
			} else {
				return fmt.Errorf(
					"%s lstat(%s) failed: %v", tag, cfg.filename, err)
			}
		}
		cfg.exists = true

		if cfg.modTime == nil {
			filesChanged = true
		} else if fi.ModTime() != *cfg.modTime {
			filesChanged = true
		}
	}

	if !filesChanged {
		return nil
	}

	buf := make(cfgBuf)

	// merge all files into buf
	for _, cfg := range s.files {
		if !cfg.exists {
			continue
		}
		err := s.mergeCfgFromFile(buf, cfg)
		if err != nil {
			return fmt.Errorf(
				"%s failed to process config: %s: %v", tag, cfg.filename, err)
		}

		fi, err := os.Lstat(cfg.filename)
		if err != nil {
			return fmt.Errorf("%s lstat(%s) failed: %v", tag, cfg.filename, err)
		}
		modTime := fi.ModTime()
		cfg.modTime = &modTime
	}

	fullCfg, err := json.MarshalIndent(buf, "", "    ")
	if err != nil {
		return fmt.Errorf("%s json.Marshal failed: %v", tag, err)
	}

	if err := s.parseData(fullCfg); err != nil {
		return fmt.Errorf("%s parse failed: %v", tag, err)
	}

	return nil
}

//
// Locals
//
func (s *svc) mergeCfgFromFile(buf cfgBuf, cfg *fileInfo) error {

	file, err := os.Open(cfg.filename)
	if err != nil {
		return err
	}

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}

	return s.mergeCfgFromBuf(buf, data)
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
		for k, v := range data {
			if _, ok := buf[key]; !ok {
				buf[key] = make(cfgBuf)
			}
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

	for k, v := range s.keys {
		if raw, ok := jc[k]; ok {
			if !bytes.Equal(v.orig, raw) {
				v.fnCallBack(k, raw)
				v.orig = raw
			}
		}
	}

	return nil
}
