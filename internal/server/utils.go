package server

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"

	"github.com/mikumaycry/akari/internal/config"
	"github.com/pkg/errors"
)

func loadServerConf(confDir string) (map[string]config.ServerConf, error) {
	m := make(map[string]config.ServerConf)
	fileInfo, err := ioutil.ReadDir(confDir)
	if err != nil {
		return nil, errors.Wrap(err, "ioutil.ReadDir")
	}
	for _, file := range fileInfo {
		data, err := ioutil.ReadFile(filepath.Join(confDir, file.Name()))
		if err != nil {
			return nil, errors.Wrap(err, "ioutil.ReadFile")
		}
		var item config.ServerConf
		if err := json.Unmarshal(data, &item); err != nil {
			return nil, errors.Wrap(err, "json.Unmarshal")
		}
		m[item.SNI] = item
	}
	return m, nil
}
