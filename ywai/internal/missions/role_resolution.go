package missions

import (
	"context"
	"errors"
	"net"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// MaxFallbackChainLen caps how many model attempts a single feature execution can make.
// Each attempt is a real LLM call, so this is a runaway-cost guard.
const MaxFallbackChainLen = 3

// RoleToSkillName maps a role identifier to its canonical SkillName used by
// the mission worker pipeline. Roles that don't map to a worker (e.g. planning)
// fall back to the generic implementation skill.
func RoleToSkillName(role string) string {
	switch role {
	case config.RoleDev:
		return "implementation"
	case config.RoleFrontend:
		return "frontend-worker"
	case config.RoleBackend:
		return "backend-worker"
	case config.RoleQA:
		return "qa-worker"
	case config.RoleReviewer:
		return "reviewer-worker"
	case config.RoleDevops:
		return "devops-worker"
	case config.RolePlanning:
		return "planner"
	default:
		return "implementation"
	}
}

// SkillNameToRole infers a role identifier from a legacy SkillName so missions
// persisted before the Role field existed still resolve to a sensible role.
func SkillNameToRole(skillName string) string {
	switch skillName {
	case "frontend-worker":
		return config.RoleFrontend
	case "backend-worker":
		return config.RoleBackend
	case "qa-worker":
		return config.RoleQA
	case "reviewer-worker":
		return config.RoleReviewer
	case "devops-worker":
		return config.RoleDevops
	case "planner":
		return config.RolePlanning
	case "implementation", "":
		return config.RoleDev
	default:
		return config.RoleDev
	}
}

// ResolveExecution returns the effective model, agent, and fallback chain for
// executing a feature. Precedence per field:
//
//	feature override → mission-wide setting → role default
//
// The chain is the resolved primary followed by the role default's fallbacks
// (deduplicated, with the primary removed from the tail), capped at
// MaxFallbackChainLen entries.
func ResolveExecution(feat *Feature, mission *Mission, cfg *config.UserConfig) (model, agent string, chain []string) {
	role := ""
	if feat != nil {
		role = feat.Role
		if role == "" {
			role = SkillNameToRole(feat.SkillName)
		}
	}
	rd := cfg.GetRoleDefault(role)

	switch {
	case feat != nil && feat.Model != "":
		model = feat.Model
	case mission != nil && mission.Model != "":
		model = mission.Model
	default:
		model = rd.Model
	}

	switch {
	case feat != nil && feat.Agent != "":
		agent = feat.Agent
	case mission != nil && mission.ExecutionAgent != "":
		agent = mission.ExecutionAgent
	case mission != nil && mission.Agent != "":
		agent = mission.Agent
	default:
		agent = rd.Agent
	}

	// Build chain: primary, then feature fallbacks (if any), then role fallbacks.
	chain = []string{}
	if model != "" {
		chain = append(chain, model)
	}
	if feat != nil {
		for _, m := range feat.Fallbacks {
			if m != "" && m != model {
				chain = append(chain, m)
			}
		}
	}
	for _, m := range rd.Fallbacks {
		if m == "" {
			continue
		}
		dup := false
		for _, existing := range chain {
			if existing == m {
				dup = true
				break
			}
		}
		if !dup {
			chain = append(chain, m)
		}
	}
	if len(chain) > MaxFallbackChainLen {
		chain = chain[:MaxFallbackChainLen]
	}
	return model, agent, chain
}

// retriableModelKeywords matches stderr / error text that indicates the model
// or its provider was transiently unavailable. Validation, auth, and prompt
// errors are NOT retried because a different model would fail the same way.
var retriableModelKeywords = []string{
	"timeout",
	"timed out",
	"rate limit",
	"too many requests",
	"model not found",
	"model_not_found",
	"unavailable",
	"overloaded",
	"server error",
	"bad gateway",
	"service unavailable",
	"gateway timeout",
	"connection refused",
	"connection reset",
	"no such host",
}

// logTail returns the last logTailMax bytes of s, used to classify worker
// stdout/stderr for retriable errors without scanning the whole log.
const logTailMax = 2048

func logTail(s string) string {
	if len(s) <= logTailMax {
		return s
	}
	return s[len(s)-logTailMax:]
}

// isRetriableModelError returns true when err indicates a transient model or
// provider failure that warrants trying the next model in the fallback chain.
func isRetriableModelError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	// Network errors are always retriable — different model may live on a
	// different endpoint.
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	msg := strings.ToLower(err.Error())
	for _, kw := range retriableModelKeywords {
		if strings.Contains(msg, kw) {
			return true
		}
	}
	return false
}
