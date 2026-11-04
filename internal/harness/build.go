package harness

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"kode-stream/internal/harness/adapters/engine/process"
	"kode-stream/internal/harness/application/brief"
	"kode-stream/internal/harness/domain"
)

// protectedPaths are never an agent's to change, whatever the scope says.
var protectedPaths = []string{".git/", ".harness/", ".claude/", ".github/workflows/"}

// planScope is where the planner may write.
var planScope = []string{"plans/"}

// scopeFor is the scope a producer's brief names and the harness enforces.
func scopeFor(role domain.Role, settings domain.Settings, workspaceID string) []string {
	if role == domain.RolePlanner {
		return planScope
	}
	if workspace, ok := settings.Workspace(workspaceID); ok {
		return workspace.Scope
	}
	return nil
}

func inScope(path string, scope []string) bool {
	for _, protected := range protectedPaths {
		if strings.HasPrefix(path, protected) || path+"/" == protected {
			return false
		}
	}
	if len(scope) == 0 {
		return true
	}
	for _, prefix := range scope {
		prefix = strings.TrimSpace(prefix)
		if path == strings.TrimSuffix(prefix, "/") || (strings.HasSuffix(prefix, "/") && strings.HasPrefix(path, prefix)) || strings.HasPrefix(path, strings.TrimSuffix(prefix, "/")+"/") {
			return true
		}
	}
	return false
}

// afterProducer runs the harness's own checks on a producer's work: paths it
// changed outside its scope are reverted, then the verification commands
// run in the worktree. Reviewers run read-only and are not checked here.
func (s *Service) afterProducer(ctx context.Context, run domain.Run, agentRun domain.AgentRun, settings domain.Settings, lease time.Duration) ([]string, *domain.GateRun, error) {
	if s.worktrees == nil {
		return nil, nil, nil
	}
	files, _, err := s.worktrees.Diff(ctx, run.WorktreePath, "HEAD")
	if err != nil {
		return nil, nil, err
	}
	scope := scopeFor(agentRun.Role, settings, run.WorkspaceID)
	if len(scope) > 0 && run.PlanPath != "" && agentRun.Role != domain.RolePlanner {
		scope = append(append([]string{}, scope...), strings.TrimSuffix(run.PlanPath, "/")+"/")
	}
	var outside []string
	for _, file := range files {
		if !inScope(file.Path, scope) {
			outside = append(outside, file.Path)
		}
	}
	if len(outside) > 0 {
		if err := s.worktrees.RestorePaths(ctx, run.WorktreePath, "HEAD", outside); err != nil {
			return outside, nil, err
		}
	}
	if agentRun.Role == domain.RolePlanner {
		return outside, nil, nil
	}
	workspace, _ := settings.Workspace(run.WorkspaceID)
	gate := runGate(ctx, run.WorktreePath, workspace.VerificationCommands, lease)
	return outside, gate, nil
}

// runGate runs each verification command in the worktree with sh -c and
// stops at the first failure. No commands means nothing to fail.
func runGate(ctx context.Context, dir string, commands []string, timeout time.Duration) *domain.GateRun {
	started := time.Now()
	gate := &domain.GateRun{Passed: true}
	for _, command := range commands {
		gate.Command = command
		exitCode, output := runCommand(ctx, dir, command, timeout)
		gate.ExitCode = exitCode
		if exitCode != 0 {
			gate.Passed = false
			gate.LogTail = tail(output, domain.MaxLogTailBytes)
			break
		}
	}
	if len(commands) == 0 {
		gate.Command = ""
	}
	gate.DurationMs = time.Since(started).Milliseconds()
	return gate
}

// gateEnvironment is the server's environment without the harness's own
// secrets: verification commands need the developer's toolchains.
func gateEnvironment() []string {
	var env []string
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "KODE_STREAM_HARNESS_") {
			continue
		}
		env = append(env, entry)
	}
	return env
}

func runCommand(ctx context.Context, dir, command string, timeout time.Duration) (int, string) {
	if timeout <= 0 {
		timeout = 30 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = dir
	cmd.Env = gateEnvironment()
	output := process.NewRing(domain.MaxLogTailBytes * 4)
	cmd.Stdout, cmd.Stderr = output, output
	process.Prepare(cmd)
	if err := cmd.Start(); err != nil {
		return -1, err.Error()
	}
	done := make(chan struct{})
	stop := process.NewStop(cmd, done)
	go func() {
		select {
		case <-ctx.Done():
			stop.Terminate()
		case <-done:
		}
	}()
	err := cmd.Wait()
	close(done)
	if ctx.Err() != nil {
		return -1, output.String() + "\n[stopped: " + ctx.Err().Error() + "]"
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), output.String()
	}
	if err != nil {
		return -1, output.String() + "\n" + err.Error()
	}
	return 0, output.String()
}

func tail(text string, limit int) string {
	data := []byte(text)
	if len(data) <= limit {
		return text
	}
	data = data[len(data)-limit:]
	if index := bytes.IndexByte(data, '\n'); index >= 0 && index < 200 {
		data = data[index+1:]
	}
	return string(bytes.ToValidUTF8(data, nil))
}

// commitMessage is the harness's commit subject: the task key, then a short
// imperative phrase.
func commitMessage(run domain.Run, what string) string {
	title := strings.TrimSpace(run.Title)
	if title == "" {
		title = "Apply the harness run"
	}
	subject := fmt.Sprintf("%s: %s%s", run.SourceKey, what, title)
	if len([]rune(subject)) > 100 {
		subject = string([]rune(subject)[:100])
	}
	return subject + "\n\nCommitted by the Kode Stream harness for run " + run.ID + "."
}

// commitRun commits everything in the run's worktree on its harness branch.
func (s *Service) commitRun(ctx context.Context, run domain.Run, what string) (string, error) {
	if s.worktrees == nil || run.WorktreePath == "" {
		return "", nil
	}
	return s.worktrees.Commit(ctx, run.WorktreePath, commitMessage(run, what))
}

// advanceBuild applies a builder's or tester's successful run: reverted
// paths become an advisory finding, a failed gate a blocking one that starts
// another round, and a passing gate goes to review.
func (st *step) advanceBuild(agentRun domain.AgentRun) {
	if agentRun.Role == domain.RoleQAFixer {
		st.advanceFix(agentRun)
		return
	}
	round := st.round(st.stage.Round)
	round.ProducerAgentRunID = agentRun.ID
	if len(agentRun.OutOfScope) > 0 {
		st.stage.Findings = append(st.stage.Findings, domain.Finding{Severity: "advisory", Title: "Changes outside the scope were reverted", Detail: boundedText(strings.Join(agentRun.OutOfScope, ", "), domain.MaxDetailChars), Round: st.stage.Round})
		st.stage.Advisory++
	}
	if gate := agentRun.HarnessGate; gate != nil && !gate.Passed {
		st.stage.Findings = append(st.stage.Findings, domain.Finding{Severity: "blocking", Title: "Verification failed: " + boundedText(gate.Command, 160), Detail: boundedText(tail(gate.LogTail, domain.MaxDetailChars), domain.MaxDetailChars), Round: st.stage.Round})
		round.Blocking++
		st.stage.Blocking = round.Blocking
		st.event(domain.EventStageRound, st.stage.ID, map[string]any{"kind": st.stage.Kind, "round": st.stage.Round, "gate": "failed"})
		if st.stage.Round >= st.stage.MaxRounds {
			st.block("verification still failed after " + fmt.Sprint(st.stage.MaxRounds) + " build rounds.")
			return
		}
		st.stage.Round++
		st.enqueue(domain.RoleCoder, false)
		return
	}
	st.enqueue(domain.RoleCodeReviewer, false)
}

// reviewApproved passes a reviewed stage. An approved build was committed by
// the harness before this step ran.
func (st *step) reviewApproved(agentRun domain.AgentRun) {
	if agentRun.Commit != "" {
		st.event(domain.EventStageRound, st.stage.ID, map[string]any{"kind": st.stage.Kind, "round": st.stage.Round, "commit": agentRun.Commit})
	}
	st.pass()
}

// decorateBrief adds the scope a producer must keep to and, after a failed
// gate, the end of its log.
func (s *Service) decorateBrief(ctx context.Context, input brief.Input, run domain.Run, stage domain.Stage, agentRun domain.AgentRun) brief.Input {
	settings := s.settingsNow(ctx)
	if producer(agentRun.Role) {
		input.Scope = scopeFor(agentRun.Role, settings, run.WorkspaceID)
	}
	if agentRun.Role == domain.RoleCoder && stage.Round > 1 {
		previous, err := s.repository.ListAgentRuns(ctx, domain.AgentRunFilter{StageID: stage.ID, States: []domain.AgentRunState{domain.AgentSucceeded}})
		if err == nil {
			for _, candidate := range previous {
				if candidate.Role == domain.RoleCoder && candidate.Round == stage.Round-1 && candidate.HarnessGate != nil && !candidate.HarnessGate.Passed {
					input.LogTail = candidate.HarnessGate.LogTail
				}
			}
		}
	}
	return s.decorateFix(ctx, input, stage, agentRun)
}
