package guardedwrite

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestReplaceExistingContract(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "file.md")
	original := []byte("original")
	if err := os.WriteFile(path, original, 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := ReplaceExisting(root, "file.md", []byte("changed"), "", 1024); !errors.Is(err, ErrHashRequired) {
		t.Fatalf("missing hash error=%v", err)
	}
	if _, err := ReplaceExisting(root, "file.md", []byte("changed"), "stale", 1024); !errors.Is(err, ErrStale) {
		t.Fatalf("stale error=%v", err)
	}
	result, err := ReplaceExisting(root, "file.md", []byte("changed"), hash(original), 1024)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o640 || result.Hash != hash([]byte("changed")) {
		t.Fatalf("result=%+v mode=%v err=%v", result, info.Mode().Perm(), err)
	}
}

func TestReplaceExistingRestoresOriginalAtEveryPublicationFailure(t *testing.T) {
	failure := errors.New("injected failure")
	cases := map[string]failureHooks{
		"write":          {afterWrite: func() error { return failure }},
		"sync":           {afterSync: func() error { return failure }},
		"close":          {afterClose: func() error { return failure }},
		"rename":         {afterRename: func() error { return failure }},
		"directory sync": {afterDirectory: func() error { return failure }},
	}
	for name, hooks := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "file.md")
			original := []byte("original")
			if err := os.WriteFile(path, original, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := replaceExisting(root, "file.md", []byte("changed"), hash(original), 1024, hooks); !errors.Is(err, failure) {
				t.Fatalf("error=%v", err)
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != string(original) {
				t.Fatalf("data=%q err=%v", data, err)
			}
		})
	}
}

func TestReplaceExistingAllowsOnlyOneConcurrentExpectedHash(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "file.md")
	original := []byte("original")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	errorsChannel := make(chan error, 2)
	var wait sync.WaitGroup
	for _, content := range []string{"first", "second"} {
		wait.Add(1)
		go func(content string) {
			defer wait.Done()
			<-start
			_, err := ReplaceExisting(root, "file.md", []byte(content), hash(original), 1024)
			errorsChannel <- err
		}(content)
	}
	close(start)
	wait.Wait()
	close(errorsChannel)
	succeeded := 0
	for err := range errorsChannel {
		if err == nil {
			succeeded++
		}
	}
	if succeeded != 1 {
		t.Fatalf("successful writes=%d", succeeded)
	}
}
