package config

import (
	"os"
	"path/filepath"
)

type Paths struct {
	Dir            string
	RegistryFile   string
	PlanIndexFile  string
	FrontendAssets string
}

func ResolvePaths() (Paths, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return Paths{}, err
	}
	dir := filepath.Join(base, "plan-manager")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Paths{}, err
	}
	return Paths{
		Dir:           dir,
		RegistryFile:  filepath.Join(dir, "repositories.json"),
		PlanIndexFile: filepath.Join(dir, "plan-index.json"),
	}, nil
}
