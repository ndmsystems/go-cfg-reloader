package reloader

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type jcfg map[string]json.RawMessage
type cfgBuf map[string]interface{}

func (s *ConfigReloader[T]) reloadConfig(forceReload bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reloadTime = time.Now()
	s.logger.Info("reloading config at", s.reloadTime, "forced:", forceReload)

	buf := make(cfgBuf)

	// merge all files into buf
	for _, cfg := range s.files {
		data, err := os.ReadFile(cfg.filename)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}

		if err := s.mergeCfgFromBuf(buf, data); err != nil {
			return fmt.Errorf(
				"failed to process config: %s: %v", cfg.filename, err)
		}
	}

	fullCfg, err := json.MarshalIndent(buf, "", "    ")
	if err != nil {
		return fmt.Errorf("json.Marshal failed: %v", err)
	}

	var newCfg T
	if err := json.Unmarshal(fullCfg, &newCfg); err != nil {
		return err
	}

	for _, cb := range s.callbacks {
		cb(s.curConfig, newCfg)
	}

	s.curConfig = newCfg

	return nil
}

// существует только из-за того что (не)нужно аппендить массивы
func (s *ConfigReloader[T]) mergeCfgFromBuf(buf cfgBuf, data []byte) error {

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

// существует только из-за того что (не)нужно аппендить массивы
func (s *ConfigReloader[T]) mergeData(buf cfgBuf, key string, data interface{}) {

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
