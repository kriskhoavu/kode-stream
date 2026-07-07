package system

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type SystemRepository struct{}

type Dialog = SystemRepository

func New() *SystemRepository {
	return &SystemRepository{}
}

func (d *Dialog) SelectDirectory() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("osascript", "-e", `POSIX path of (choose folder with prompt "Select workspace folder")`).Output()
		if err != nil {
			return "", errors.New("directory selection cancelled")
		}
		return cleanSelectedPath(string(out))
	case "windows":
		script := `Add-Type -AssemblyName System.Windows.Forms; $dialog = New-Object System.Windows.Forms.FolderBrowserDialog; $dialog.Description = "Select workspace folder"; if ($dialog.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) { [Console]::Write($dialog.SelectedPath) }`
		out, err := exec.Command("powershell", "-NoProfile", "-STA", "-Command", script).Output()
		if err != nil {
			return "", errors.New("directory selection cancelled")
		}
		return cleanSelectedPath(string(out))
	default:
		if path, err := selectWithCommand("zenity", "--file-selection", "--directory", "--title=Select workspace folder"); err == nil {
			return path, nil
		}
		if path, err := selectWithCommand("kdialog", "--getexistingdirectory", "."); err == nil {
			return path, nil
		}
		return "", errors.New("native directory picker is not available on this platform")
	}
}

func (d *Dialog) SelectYAMLFile() (string, error) {
	var out []byte
	var err error
	switch runtime.GOOS {
	case "darwin":
		out, err = exec.Command("osascript", "-e", `try
POSIX path of (choose file with prompt "Select workspaces.yaml" of type {"public.yaml"})
on error number -128
return ""
end try`).Output()
	case "windows":
		script := `Add-Type -AssemblyName System.Windows.Forms; $dialog = New-Object System.Windows.Forms.OpenFileDialog; $dialog.Title = "Select workspaces.yaml"; $dialog.Filter = "YAML files (*.yaml;*.yml)|*.yaml;*.yml"; if ($dialog.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) { [Console]::Write($dialog.FileName) }`
		out, err = exec.Command("powershell", "-NoProfile", "-STA", "-Command", script).Output()
	default:
		commands := [][]string{
			{"zenity", "--file-selection", "--title=Select workspaces.yaml", "--file-filter=YAML files | *.yaml *.yml"},
			{"kdialog", "--getopenfilename", ".", "YAML files (*.yaml *.yml)"},
		}
		for _, command := range commands {
			if _, lookupErr := exec.LookPath(command[0]); lookupErr != nil {
				continue
			}
			selected, selectErr := exec.Command(command[0], command[1:]...).Output()
			if selectErr != nil {
				var exitErr *exec.ExitError
				if errors.As(selectErr, &exitErr) {
					return "", nil
				}
				return "", selectErr
			}
			if strings.TrimSpace(string(selected)) == "" {
				return "", nil
			}
			return cleanSelectedPath(string(selected))
		}
		return "", errors.New("native file picker is not available on this platform")
	}
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(string(out)) == "" {
		return "", nil
	}
	return cleanSelectedPath(string(out))
}

func (d *Dialog) OpenPath(path string) error {
	clean, err := cleanSelectedPath(path)
	if err != nil {
		return err
	}
	stat, err := os.Stat(clean)
	if err != nil {
		return fmt.Errorf("path does not exist: %w", err)
	}
	if !stat.IsDir() {
		clean = filepath.Dir(clean)
	}

	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", clean).Start()
	case "windows":
		return exec.Command("explorer", clean).Start()
	default:
		return exec.Command("xdg-open", clean).Start()
	}
}

func selectWithCommand(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return "", err
	}
	return cleanSelectedPath(string(out))
}

func cleanSelectedPath(raw string) (string, error) {
	path := strings.TrimSpace(raw)
	path = strings.Trim(path, `"'`)
	if strings.HasPrefix(path, "file://") {
		parsed, err := url.Parse(path)
		if err != nil {
			return "", fmt.Errorf("invalid file URL")
		}
		path = parsed.Path
		if runtime.GOOS == "windows" && strings.HasPrefix(path, "/") {
			path = strings.TrimPrefix(path, "/")
		}
	}
	if path == "" {
		return "", errors.New("path is empty")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("path is invalid")
	}
	return abs, nil
}
