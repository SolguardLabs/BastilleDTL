package scenario

import (
	"encoding/json"
	"os"
)

func LoadFile(path string) (Definition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Definition{}, err
	}
	var definition Definition
	if err := json.Unmarshal(data, &definition); err != nil {
		return Definition{}, err
	}
	if definition.Name == "" {
		definition.Name = path
	}
	if len(definition.Bootstrap.Institutions) == 0 {
		definition.Bootstrap = DefaultBootstrap()
	}
	return definition, nil
}

func LoadBootstrapFile(path string) (engineBootstrap Definition, err error) {
	return LoadFile(path)
}
