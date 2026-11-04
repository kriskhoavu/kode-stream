package credential

import (
	"os"
	"testing"
)

func TestResolvePrefersEnvironmentThenExportedFileValues(t *testing.T) {
	files := map[string]string{"/home/.creds.zsh": "GITHUB_TOKEN=bare\nexport GITHUB_TOKEN=\"from-file\" \nexport OTHER=x # note\n"}
	resolver := Resolver{
		Getenv: func(name string) string { return map[string]string{"SET": "env"}[name] },
		ReadFile: func(path string) ([]byte, error) {
			if data, ok := files[path]; ok {
				return []byte(data), nil
			}
			return nil, os.ErrNotExist
		},
		HomeDir: func() (string, error) { return "/home", nil },
		Files:   []string{".creds.zsh", ".creds.sh"},
	}
	for name, want := range map[string]string{"SET": "env", "GITHUB_TOKEN": "from-file", "OTHER": "x", "MISSING": "", "": ""} {
		if got, err := resolver.Resolve(name); err != nil || got != want {
			t.Fatalf("%s = %q err=%v, want %q", name, got, err, want)
		}
	}
}
