package system

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type Status string

const (
	StatusPass    Status = "pass"
	StatusWarning Status = "warning"
	StatusFail    Status = "fail"
)

type Check struct {
	ID          string   `json:"id"`
	Status      Status   `json:"status"`
	Required    bool     `json:"required"`
	Message     string   `json:"message"`
	Remediation []string `json:"remediation,omitempty"`
}

type Summary struct {
	Passed   int `json:"passed"`
	Failed   int `json:"failed"`
	Warnings int `json:"warnings"`
}

type Result struct {
	OK      bool    `json:"ok"`
	Summary Summary `json:"summary"`
	Checks  []Check `json:"checks"`
}

func (r Result) ExitCode(strict bool) int {
	if r.Summary.Failed > 0 {
		return 1
	}
	if strict && r.Summary.Warnings > 0 {
		return 1
	}
	if !strict && r.Summary.Warnings > 0 {
		return 3
	}
	return 0
}

type Options struct {
	Provider string
	Repo     string
	Port     int
	Strict   bool
}

type DoctorService struct {
	runtime runtimeChecker
	git     gitChecker
	getwd   func() (string, error)
}

func NewDoctorService() *DoctorService {
	return &DoctorService{
		runtime: defaultRuntimeChecker{},
		git:     newDefaultGitChecker(),
		getwd:   os.Getwd,
	}
}

func (s *DoctorService) Run(opts Options) Result {
	result := Result{OK: true, Checks: []Check{}}
	add := func(check Check) {
		result.Checks = append(result.Checks, check)
		switch check.Status {
		case StatusPass:
			result.Summary.Passed++
		case StatusWarning:
			result.Summary.Warnings++
			result.OK = false
		case StatusFail:
			result.Summary.Failed++
			result.OK = false
		}
	}

	executable, ok := s.checkRuntimeBinary()
	add(ok)
	_ = executable

	add(s.checkConfigPath())

	version, versionCheck := s.checkGitInstalled()
	add(versionCheck)
	add(s.checkGitVersion(version))
	if versionCheck.Status == StatusFail {
		return result
	}

	ctx := s.resolveRepoContext(opts.Repo)
	add(ctx.repoContext)
	if ctx.repoContext.Status == StatusFail {
		return result
	}

	providerCheck := s.checkRemoteConfig(ctx.remoteURL, opts.Provider)
	add(providerCheck)
	if providerCheck.Status == StatusFail {
		return result
	}

	add(s.checkProviderAuth(ctx.remoteURL))
	add(s.checkRepoReadAccess(ctx.remoteURL))
	if opts.Port > 0 {
		add(s.checkLocalPort(opts.Port))
	}

	return result
}

func (s *DoctorService) checkRuntimeBinary() (string, Check) {
	path, err := s.runtime.CurrentExecutable()
	if err != nil {
		return "", fail("runtime.binary", true, "kode-stream executable is unavailable", "Reinstall kode-stream and ensure PATH includes the binary.")
	}
	if _, err := os.Stat(path); err != nil {
		return "", fail("runtime.binary", true, fmt.Sprintf("executable path is invalid: %s", path), "Reinstall kode-stream and verify the install location exists.")
	}
	return path, pass("runtime.binary", true, fmt.Sprintf("kode-stream executable found at %s", path))
}

func (s *DoctorService) checkConfigPath() Check {
	paths, err := s.runtime.ResolvePaths()
	if err != nil {
		return fail("runtime.config-path", true, "config path could not be resolved", "Set KODE_STREAM_DATA_DIR to a writable path and retry.")
	}
	if err := s.runtime.CanWriteDir(paths.Dir); err != nil {
		return fail("runtime.config-path", true, fmt.Sprintf("config path is not writable: %s", paths.Dir), "Grant write permission to the config directory or choose another data directory.")
	}
	return pass("runtime.config-path", true, fmt.Sprintf("config path is writable: %s", paths.Dir))
}

func (s *DoctorService) checkGitInstalled() (string, Check) {
	version, err := s.git.Version()
	if err != nil {
		return "", fail("git.installed", true, "git is not available", "Install Git and restart your shell.")
	}
	return version, pass("git.installed", true, version)
}

func (s *DoctorService) checkGitVersion(version string) Check {
	major, minor := parseGitVersion(version)
	if major == 0 && minor == 0 {
		return warning("git.version", false, fmt.Sprintf("could not parse git version from %q", version), "Use Git 2.30 or newer.")
	}
	if major < 2 || (major == 2 && minor < 30) {
		return warning("git.version", false, fmt.Sprintf("git version %d.%d is older than recommended baseline 2.30", major, minor), "Upgrade Git for better compatibility.")
	}
	return pass("git.version", false, fmt.Sprintf("git version baseline satisfied (%d.%d)", major, minor))
}

type repoContext struct {
	repoContext Check
	remoteURL   string
}

func (s *DoctorService) resolveRepoContext(repoInput string) repoContext {
	trimmed := strings.TrimSpace(repoInput)
	if looksLikeRemote(trimmed) {
		return repoContext{
			repoContext: pass("repo.context", true, fmt.Sprintf("using explicit remote %s", trimmed)),
			remoteURL:   trimmed,
		}
	}

	path := trimmed
	if path == "" {
		cwd, err := s.getwd()
		if err != nil {
			return repoContext{repoContext: fail("repo.context", true, "could not resolve current working directory", "Run doctor from a repository path or pass --repo <path>.")}
		}
		path = cwd
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return repoContext{repoContext: fail("repo.context", true, fmt.Sprintf("invalid repository path: %s", path), "Pass a valid local repository path or remote URL with --repo.")}
	}
	isRepo, err := s.git.IsRepository(abs)
	if err != nil || !isRepo {
		return repoContext{repoContext: fail("repo.context", true, fmt.Sprintf("%s is not a Git repository", abs), "Run doctor in a Git repository or pass --repo <remote-url>.")}
	}
	remote, err := s.git.RemoteURL(abs, "origin")
	if err != nil || strings.TrimSpace(remote) == "" {
		return repoContext{
			repoContext: fail("repo.context", true, "remote origin is not configured", "Set a remote origin URL with `git remote add origin <url>` and retry."),
		}
	}
	return repoContext{repoContext: pass("repo.context", true, fmt.Sprintf("repository context: %s", abs)), remoteURL: remote}
}

func (s *DoctorService) checkRemoteConfig(remoteURL, provider string) Check {
	if strings.TrimSpace(remoteURL) == "" {
		return fail("git.remote-config", true, "remote URL is empty", "Set a valid Git remote URL and retry.")
	}
	p := providerFromRemote(remoteURL)
	if p == "" {
		return warning("git.remote-config", true, fmt.Sprintf("provider for remote could not be recognized: %s", remoteURL), "Use a GitHub or Bitbucket remote in v1.")
	}
	if strings.TrimSpace(provider) != "" && !strings.EqualFold(strings.TrimSpace(provider), p) {
		return fail("git.remote-config", true, fmt.Sprintf("remote provider is %s but --provider is %s", p, provider), "Use a matching --provider flag or update the remote URL.")
	}
	return pass("git.remote-config", true, fmt.Sprintf("remote URL looks valid (%s)", p))
}

func (s *DoctorService) checkProviderAuth(remoteURL string) Check {
	if err := s.git.LSRemote(remoteURL); err != nil {
		return fail("auth.provider", true, fmt.Sprintf("cannot read remote refs from %s", remoteURL), "Configure SSH key or HTTPS credential manager access for this remote.")
	}
	return pass("auth.provider", true, "provider authentication works")
}

func (s *DoctorService) checkRepoReadAccess(remoteURL string) Check {
	if err := s.git.LSRemote(remoteURL); err != nil {
		return fail("repo.read-access", true, "remote repository read access failed", "Verify repository permissions and remote URL ownership.")
	}
	return pass("repo.read-access", true, "remote repository access confirmed")
}

func (s *DoctorService) checkLocalPort(port int) Check {
	addr := "127.0.0.1:" + strconv.Itoa(port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return warning("local.port", false, fmt.Sprintf("cannot bind %s", addr), "Use a different port for service mode or stop the conflicting local process.")
	}
	_ = ln.Close()
	return pass("local.port", false, fmt.Sprintf("port %d is available for localhost service", port))
}

func pass(id string, required bool, message string) Check {
	return Check{ID: id, Status: StatusPass, Required: required, Message: message}
}

func warning(id string, required bool, message string, remediation ...string) Check {
	return Check{ID: id, Status: StatusWarning, Required: required, Message: message, Remediation: remediation}
}

func fail(id string, required bool, message string, remediation ...string) Check {
	return Check{ID: id, Status: StatusFail, Required: required, Message: message, Remediation: remediation}
}

var gitVersionPattern = regexp.MustCompile(`(\d+)\.(\d+)`)

func parseGitVersion(raw string) (int, int) {
	m := gitVersionPattern.FindStringSubmatch(raw)
	if len(m) != 3 {
		return 0, 0
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	return major, minor
}

func looksLikeRemote(value string) bool {
	v := strings.TrimSpace(value)
	if v == "" {
		return false
	}
	if strings.HasPrefix(v, "git@") || strings.HasPrefix(v, "ssh://") {
		return true
	}
	u, err := url.Parse(v)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

func providerFromRemote(remoteURL string) string {
	raw := strings.ToLower(strings.TrimSpace(remoteURL))
	if strings.Contains(raw, "github.com") {
		return "github"
	}
	if strings.Contains(raw, "bitbucket.org") {
		return "bitbucket"
	}
	return ""
}
