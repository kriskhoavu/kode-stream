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
	paths := Paths{
		Dir:           dir,
		RegistryFile:  filepath.Join(dir, "workspaces.yaml"),
		PlanIndexFile: filepath.Join(dir, "item-index.yaml"),
	}
	copyLegacyFile(filepath.Join(dir, "repositories.yaml"), paths.RegistryFile)
	copyLegacyFile(filepath.Join(dir, "plan-index.yaml"), paths.PlanIndexFile)
	return paths, nil
}

func copyLegacyFile(oldPath, newPath string) {
	if _, err := os.Stat(newPath); err == nil {
		return
	}
	data, err := os.ReadFile(oldPath)
	if err != nil {
		return
	}
	_ = os.WriteFile(newPath, data, 0o600)
}
