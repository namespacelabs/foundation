package nodejs

import (
	"encoding/json"
	"io/fs"
	"sync"
)

var (
	v     versions
	vonce sync.Once
)

type versions struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

func builtin() *versions {
	vonce.Do(func() {
		data, err := fs.ReadFile(resources, "versions.json")
		if err != nil {
			panic(err)
		}
		if err := json.Unmarshal(data, &v); err != nil {
			panic(err)
		}
	})
	return &v
}
