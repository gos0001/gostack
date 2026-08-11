package scaffold

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ManifestName is the per-project file recording which features the project was
// created with. It is committed to the repo — later `gostack g ...` runs read it
// to know whether pages, postgres, etc. are available.
const ManifestName = "gostack.json"

type Manifest struct {
	Version  int      `json:"version"`
	Name     string   `json:"name"`
	Module   string   `json:"module"`
	Features Features `json:"features"`
}

func WriteManifest(dir string, m Manifest) error {
	if m.Version == 0 {
		m.Version = 1
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, ManifestName), append(data, '\n'), 0644)
}

// ReadManifest loads the manifest. Callers should check errors.Is(err, fs.ErrNotExist)
// and fall back to DetectFeatures for projects created before manifests existed.
func ReadManifest(dir string) (Manifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, ManifestName))
	if err != nil {
		return Manifest{}, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("parse %s: %w", ManifestName, err)
	}
	if m.Version == 0 {
		m.Version = 1
	}
	if m.Version > 1 {
		return Manifest{}, fmt.Errorf("%s has version %d — upgrade gostack", ManifestName, m.Version)
	}
	return m, nil
}

// DetectFeatures infers which features a project has by probing its tree.
// Used only as a fallback when gostack.json is absent.
func DetectFeatures(dir string) Features {
	has := func(parts ...string) bool {
		_, err := os.Stat(filepath.Join(append([]string{dir}, parts...)...))
		return err == nil
	}
	return Features{
		Frontend: has("views") || has("pkg", "views"),
		Postgres: has("sqlc.yaml") || has("internal", "adapter", "postgres"),
		Redis:    has("pkg", "redis") || has("internal", "adapter", "redis"),
		Docker:   has("docker-compose.yml"),
	}
}
