package security

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Skill validator — pre-execution validation framework
// ---------------------------------------------------------------------------

// SkillManifest declares what a skill needs to run (resources, permissions,
// dependencies). Used for pre-execution validation and sandboxing decisions.
type SkillManifest struct {
	SkillID      string          `json:"skill_id"`
	Name         string          `json:"name"`
	Version      int             `json:"version"`
	Author       string          `json:"author"`
	Signature    string          `json:"signature"`    // SHA-256 of code content
	Permissions  SkillPermissions `json:"permissions"`
	Dependencies []string        `json:"dependencies"` // Required packages/tools
	MaxMemoryMB  int             `json:"max_memory_mb"`
	MaxCPU       float64         `json:"max_cpu"`
	MaxTimeoutS  int             `json:"max_timeout_s"`
	NetworkAllow bool            `json:"network_allow"` // Requires outbound network
	CreatedAt    time.Time       `json:"created_at"`
}

// SkillPermissions describes what a skill is allowed to do.
type SkillPermissions struct {
	FileRead     bool     `json:"file_read"`
	FileWrite    bool     `json:"file_write"`
	NetworkHTTP  bool     `json:"network_http"`
	NetworkRaw   bool     `json:"network_raw"`
	ProcessExec  bool     `json:"process_exec"`
	EnvVars      []string `json:"env_vars,omitempty"`      // Allowed env vars
	AllowedPaths []string `json:"allowed_paths,omitempty"` // Filesystem paths
}

// ValidationResult holds the outcome of skill validation.
type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Warnings []string `json:"warnings,omitempty"`
	Errors   []string `json:"errors,omitempty"`
}

// SkillValidator validates skills before execution or registration.
type SkillValidator struct {
	mu              sync.RWMutex
	trustedAuthors  map[string]bool
	blockedSkills   map[string]bool
	maxMemoryMB     int
	maxTimeoutS     int
	allowNetwork    bool
}

// ValidatorConfig holds configuration for the SkillValidator.
type ValidatorConfig struct {
	TrustedAuthors []string // Authors whose skills are auto-trusted
	MaxMemoryMB    int      // Maximum memory a skill can request (default: 512)
	MaxTimeoutS    int      // Maximum timeout (default: 300)
	AllowNetwork   bool     // Whether skills can request network access
}

// NewSkillValidator creates a SkillValidator with the given config.
func NewSkillValidator(cfg ValidatorConfig) *SkillValidator {
	if cfg.MaxMemoryMB <= 0 {
		cfg.MaxMemoryMB = 512
	}
	if cfg.MaxTimeoutS <= 0 {
		cfg.MaxTimeoutS = 300
	}

	trusted := make(map[string]bool)
	for _, a := range cfg.TrustedAuthors {
		trusted[a] = true
	}

	return &SkillValidator{
		trustedAuthors: trusted,
		blockedSkills:  make(map[string]bool),
		maxMemoryMB:    cfg.MaxMemoryMB,
		maxTimeoutS:    cfg.MaxTimeoutS,
		allowNetwork:   cfg.AllowNetwork,
	}
}

// Validate checks a skill manifest against security policies.
func (v *SkillValidator) Validate(manifest SkillManifest) ValidationResult {
	result := ValidationResult{Valid: true}

	// 1. Check if skill is blocked.
	v.mu.RLock()
	if v.blockedSkills[manifest.SkillID] {
		result.Valid = false
		result.Errors = append(result.Errors, "skill is blocked")
		v.mu.RUnlock()
		return result
	}
	v.mu.RUnlock()

	// 2. Check signature present.
	if manifest.Signature == "" {
		result.Warnings = append(result.Warnings, "no signature — skill integrity unverified")
	}

	// 3. Check resource limits.
	if manifest.MaxMemoryMB > v.maxMemoryMB {
		result.Valid = false
		result.Errors = append(result.Errors,
			fmt.Sprintf("requested memory %dMB exceeds limit %dMB", manifest.MaxMemoryMB, v.maxMemoryMB))
	}

	if manifest.MaxTimeoutS > v.maxTimeoutS {
		result.Valid = false
		result.Errors = append(result.Errors,
			fmt.Sprintf("requested timeout %ds exceeds limit %ds", manifest.MaxTimeoutS, v.maxTimeoutS))
	}

	// 4. Check network permissions.
	if manifest.NetworkAllow && !v.allowNetwork {
		result.Valid = false
		result.Errors = append(result.Errors, "network access denied by policy")
	}

	// 5. Check dangerous permissions.
	if manifest.Permissions.ProcessExec {
		result.Warnings = append(result.Warnings, "skill requests process execution — high risk")
	}
	if manifest.Permissions.NetworkRaw {
		result.Warnings = append(result.Warnings, "skill requests raw network access — high risk")
	}
	if manifest.Permissions.FileWrite && len(manifest.Permissions.AllowedPaths) == 0 {
		result.Warnings = append(result.Warnings, "skill requests write access without path restrictions")
	}

	// 6. Check for suspicious dependencies.
	for _, dep := range manifest.Dependencies {
		if isSuspiciousDep(dep) {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("suspicious dependency: %s", dep))
		}
	}

	// 7. Trusted author check.
	v.mu.RLock()
	if !v.trustedAuthors[manifest.Author] {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("author %q is not in trusted list", manifest.Author))
	}
	v.mu.RUnlock()

	return result
}

// VerifySignature checks that the given code content matches the manifest signature.
func (v *SkillValidator) VerifySignature(manifest SkillManifest, codeContent string) bool {
	if manifest.Signature == "" {
		return false
	}
	hash := sha256.Sum256([]byte(codeContent))
	computed := hex.EncodeToString(hash[:])
	return computed == manifest.Signature
}

// ComputeSignature computes SHA-256 signature for code content.
func ComputeSignature(codeContent string) string {
	hash := sha256.Sum256([]byte(codeContent))
	return hex.EncodeToString(hash[:])
}

// BlockSkill adds a skill to the blocklist.
func (v *SkillValidator) BlockSkill(skillID string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.blockedSkills[skillID] = true
}

// UnblockSkill removes a skill from the blocklist.
func (v *SkillValidator) UnblockSkill(skillID string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.blockedSkills, skillID)
}

// AddTrustedAuthor adds an author to the trusted list.
func (v *SkillValidator) AddTrustedAuthor(author string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.trustedAuthors[author] = true
}

// IsBlocked checks if a skill is blocked.
func (v *SkillValidator) IsBlocked(skillID string) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.blockedSkills[skillID]
}

// ---------------------------------------------------------------------------
// Safety policy enforcer
// ---------------------------------------------------------------------------

// PolicyEnforcer checks agent safety policies before execution.
type PolicyEnforcer struct {
	mu          sync.RWMutex
	runCounts   map[string]int // agentID → current concurrent runs
}

// NewPolicyEnforcer creates a PolicyEnforcer.
func NewPolicyEnforcer() *PolicyEnforcer {
	return &PolicyEnforcer{
		runCounts: make(map[string]int),
	}
}

// PolicyViolation describes why an action was denied.
type PolicyViolation struct {
	Agent   string `json:"agent"`
	Rule    string `json:"rule"`
	Details string `json:"details"`
}

// CheckExecution validates whether an agent can execute a task with the
// given tool. Returns nil if allowed, or a violation if denied.
func (pe *PolicyEnforcer) CheckExecution(agentID string, maxConcurrent int, forbiddenTools []string, requireApproval bool, toolName string) *PolicyViolation {
	// Check concurrent runs.
	pe.mu.RLock()
	current := pe.runCounts[agentID]
	pe.mu.RUnlock()

	if maxConcurrent > 0 && current >= maxConcurrent {
		return &PolicyViolation{
			Agent:   agentID,
			Rule:    "max_concurrent_runs",
			Details: fmt.Sprintf("agent has %d/%d concurrent runs", current, maxConcurrent),
		}
	}

	// Check forbidden tools.
	for _, ft := range forbiddenTools {
		if strings.EqualFold(ft, toolName) {
			return &PolicyViolation{
				Agent:   agentID,
				Rule:    "forbidden_tool",
				Details: fmt.Sprintf("tool %q is forbidden for agent %s", toolName, agentID),
			}
		}
	}

	// Check approval requirement.
	if requireApproval {
		return &PolicyViolation{
			Agent:   agentID,
			Rule:    "require_approval",
			Details: "agent requires human approval before execution",
		}
	}

	return nil
}

// AcquireRun marks the start of a run for concurrency tracking.
func (pe *PolicyEnforcer) AcquireRun(agentID string) {
	pe.mu.Lock()
	defer pe.mu.Unlock()
	pe.runCounts[agentID]++
}

// ReleaseRun marks the end of a run.
func (pe *PolicyEnforcer) ReleaseRun(agentID string) {
	pe.mu.Lock()
	defer pe.mu.Unlock()
	pe.runCounts[agentID]--
	if pe.runCounts[agentID] <= 0 {
		delete(pe.runCounts, agentID)
	}
}

// ActiveRuns returns the number of concurrent runs for an agent.
func (pe *PolicyEnforcer) ActiveRuns(agentID string) int {
	pe.mu.RLock()
	defer pe.mu.RUnlock()
	return pe.runCounts[agentID]
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// isSuspiciousDep checks if a dependency name looks suspicious.
func isSuspiciousDep(dep string) bool {
	suspicious := []string{
		"eval", "exec", "subprocess", "shell",
		"pickle", "marshal", "deserialize",
		"ctypes", "cffi",
		"socket", "rawsocket",
	}
	lower := strings.ToLower(dep)
	for _, s := range suspicious {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}
