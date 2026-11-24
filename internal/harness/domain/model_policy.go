package domain

import (
	"strings"
)

type RolePolicy struct {
	Engine              string   `json:"engine"`
	Model               string   `json:"model"`
	HighDifficultyModel string   `json:"highDifficultyModel,omitempty"`
	MaxTurns            int      `json:"maxTurns"`
	PermissionMode      string   `json:"permissionMode"`
	AllowedTools        []string `json:"allowedTools"`
}

type PolicyMax struct {
	MaxTurns       int     `json:"maxTurns"`
	PerAgentRunUSD float64 `json:"perAgentRunUsd"`
}

type ModelPolicy struct {
	Roles           map[Role]RolePolicy `json:"roles"`
	DisallowedTools []string            `json:"disallowedTools"`
	Max             PolicyMax           `json:"max"`
	PerAgentRunUSD  float64             `json:"perAgentRunUsd"`
}

var readOnlyTools = []string{"Read", "Grep", "Glob"}

var AlwaysDisallowedTools = []string{"Bash(git commit:*)", "Bash(git push:*)", "Bash(git checkout:*)", "Bash(git switch:*)", "Bash(git reset:*)", "Bash(rm -rf:*)"}

func DefaultModelPolicy() ModelPolicy {
	editTools := append(append([]string{}, readOnlyTools...), "Write", "Edit")
	gitRead := []string{"Bash(git diff:*)", "Bash(git log:*)", "Bash(git status:*)"}
	tests := []string{"Bash(go test:*)", "Bash(go vet:*)", "Bash(npm test:*)", "Bash(npm run typecheck:*)", "Bash(make verify:*)"}
	join := func(parts ...[]string) []string {
		var out []string
		for _, part := range parts {
			out = append(out, part...)
		}
		return out
	}
	return ModelPolicy{
		Roles: map[Role]RolePolicy{
			RolePlanner:      {Engine: "claudecode", Model: "opus", MaxTurns: 60, PermissionMode: "acceptEdits", AllowedTools: join(editTools, gitRead[:2])},
			RolePlanReviewer: {Engine: "claudecode", Model: "opus", MaxTurns: 25, PermissionMode: "plan", AllowedTools: join(readOnlyTools, gitRead[:1])},
			RoleCoder:        {Engine: "claudecode", Model: "sonnet", HighDifficultyModel: "opus", MaxTurns: 120, PermissionMode: "acceptEdits", AllowedTools: join(editTools, gitRead, tests)},
			RoleCodeReviewer: {Engine: "claudecode", Model: "opus", MaxTurns: 40, PermissionMode: "plan", AllowedTools: join(readOnlyTools, gitRead[:1], tests)},
			RoleQAFixer:      {Engine: "claudecode", Model: "sonnet", MaxTurns: 60, PermissionMode: "acceptEdits", AllowedTools: join(editTools, gitRead[:1], tests)},
		},
		DisallowedTools: append([]string{}, AlwaysDisallowedTools...),
		Max:             PolicyMax{MaxTurns: DefaultPolicyMaxTurn, PerAgentRunUSD: DefaultPolicyMaxUSD},
		PerAgentRunUSD:  5,
	}
}

// A person cannot remove the always-disallowed tools.
func (p ModelPolicy) Normalize() (ModelPolicy, error) {
	defaults := DefaultModelPolicy()
	if p.Max.MaxTurns == 0 {
		p.Max.MaxTurns = defaults.Max.MaxTurns
	}
	if p.Max.PerAgentRunUSD == 0 {
		p.Max.PerAgentRunUSD = defaults.Max.PerAgentRunUSD
	}
	if p.Max.MaxTurns < 1 || p.Max.MaxTurns > DefaultPolicyMaxTurn {
		return p, Invalid("modelPolicy.max.maxTurns must be between 1 and %d", DefaultPolicyMaxTurn)
	}
	if p.Max.PerAgentRunUSD <= 0 || p.Max.PerAgentRunUSD > 1000 {
		return p, Invalid("modelPolicy.max.perAgentRunUsd must be above 0 and at most 1000")
	}
	if p.PerAgentRunUSD == 0 {
		p.PerAgentRunUSD = defaults.PerAgentRunUSD
	}
	if p.PerAgentRunUSD < 0 || p.PerAgentRunUSD > p.Max.PerAgentRunUSD {
		return p, Invalid("modelPolicy.perAgentRunUsd must be above 0 and at most %.2f", p.Max.PerAgentRunUSD)
	}
	if p.Roles == nil {
		p.Roles = map[Role]RolePolicy{}
	}
	for role := range p.Roles {
		if !validRole(role) {
			return p, Invalid("unknown role %q in modelPolicy.roles", role)
		}
	}
	for _, role := range Roles {
		policy, ok := p.Roles[role]
		if !ok {
			p.Roles[role] = defaults.Roles[role]
			continue
		}
		fallback := defaults.Roles[role]
		if strings.TrimSpace(policy.Engine) == "" {
			policy.Engine = fallback.Engine
		}
		if strings.TrimSpace(policy.Model) == "" {
			policy.Model = fallback.Model
		}
		if policy.MaxTurns == 0 {
			policy.MaxTurns = fallback.MaxTurns
		}
		if policy.MaxTurns < 1 || policy.MaxTurns > p.Max.MaxTurns {
			return p, Invalid("modelPolicy.roles.%s.maxTurns must be between 1 and %d", role, p.Max.MaxTurns)
		}
		switch policy.PermissionMode {
		case "":
			policy.PermissionMode = fallback.PermissionMode
		case "plan", "acceptEdits", "default":
		default:
			return p, Invalid("modelPolicy.roles.%s.permissionMode must be plan, acceptEdits or default", role)
		}
		if (role == RolePlanReviewer || role == RoleCodeReviewer) && policy.PermissionMode != "plan" {
			return p, Invalid("modelPolicy.roles.%s must stay read-only (permissionMode plan)", role)
		}
		if policy.AllowedTools == nil {
			policy.AllowedTools = fallback.AllowedTools
		}
		for _, tool := range policy.AllowedTools {
			if strings.TrimSpace(tool) == "" || strings.ContainsAny(tool, "\n\r") {
				return p, Invalid("modelPolicy.roles.%s.allowedTools has an empty or multi-line entry", role)
			}
			if isAlwaysDisallowed(tool) {
				return p, Invalid("modelPolicy.roles.%s may not allow %s", role, tool)
			}
		}
		p.Roles[role] = policy
	}
	p.DisallowedTools = mergeTools(p.DisallowedTools, AlwaysDisallowedTools)
	return p, nil
}

func validRole(role Role) bool {
	for _, candidate := range Roles {
		if candidate == role {
			return true
		}
	}
	return false
}

func isAlwaysDisallowed(tool string) bool {
	normalized := strings.ReplaceAll(strings.TrimSpace(tool), " ", "")
	for _, refused := range AlwaysDisallowedTools {
		if strings.ReplaceAll(refused, " ", "") == normalized {
			return true
		}
	}
	return strings.HasPrefix(normalized, "Bash(gitcommit") || strings.HasPrefix(normalized, "Bash(gitpush") || normalized == "Bash" || normalized == "Bash(*)"
}

func mergeTools(base, required []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, tool := range append(append([]string{}, base...), required...) {
		tool = strings.TrimSpace(tool)
		if tool == "" || seen[tool] {
			continue
		}
		seen[tool] = true
		out = append(out, tool)
	}
	return out
}

func (p ModelPolicy) ModelFor(role Role, difficulty string) string {
	policy := p.Roles[role]
	if difficulty == "high" && policy.HighDifficultyModel != "" {
		return policy.HighDifficultyModel
	}
	return policy.Model
}

func (p ModelPolicy) BudgetFor() float64 {
	if p.PerAgentRunUSD > 0 && p.PerAgentRunUSD <= p.Max.PerAgentRunUSD {
		return p.PerAgentRunUSD
	}
	return p.Max.PerAgentRunUSD
}
