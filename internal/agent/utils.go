package agent

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"

	"github.com/mikumaycry/akari/internal/config"
	"github.com/pkg/errors"
)

func loadAgentConf(confDir string) ([]config.AgentConf, error) {
	var m []config.AgentConf
	fileInfo, err := ioutil.ReadDir(confDir)
	if err != nil {
		return nil, errors.Wrap(err, "ioutil.ReadDir")
	}
	for _, file := range fileInfo {
		data, err := ioutil.ReadFile(filepath.Join(confDir, file.Name()))
		if err != nil {
			return nil, errors.Wrap(err, "ioutil.ReadFile")
		}
		var item config.AgentConf
		if err := json.Unmarshal(data, &item); err != nil {
			return nil, errors.Wrap(err, "json.Unmarshal")
		}
		m = append(m, item)
	}
	return m, nil
}
