// Package skills provides the 20 starter skills for the Overhuman system.
//
// Each skill implements instruments.SkillExecutor and can be registered
// into the SkillRegistry. Skills are organized into 5 categories:
//
//   - Development & Code: CodeExec, Git, Testing, Browser, Database
//   - Communication: Email, Calendar, Messaging, Documents
//   - Research & Information: WebSearch, PDFAnalysis, DataAggregation, Monitoring
//   - File & Data: FileOps, DataAnalysis, KnowledgeSearch
//   - Automation & Security: APIIntegration, Scheduler, Audit, Credentials
//
// Skills that require external services (APIs, binaries) are implemented
// as stubs that return descriptive errors when their backend is not configured.
package skills

import (
	"github.com/overhuman/overhuman/internal/instruments"
	"github.com/overhuman/overhuman/internal/storage"
)

// Config holds configuration for all starter skills.
type Config struct {
	DataDir     string           // Base directory for file operations
	Store       storage.Store    // Persistent storage for credentials, audit, etc.
	Sandbox     *instruments.DockerSandbox // Docker sandbox for code execution
}

// SkillDef describes a starter skill for registration.
type SkillDef struct {
	ID          string
	Name        string
	Category    string
	Description string
	Type        instruments.SkillType
	Executor    instruments.SkillExecutor
}

// RegisterAll creates and registers all 20 starter skills.
// Returns the number of skills registered.
func RegisterAll(registry *instruments.SkillRegistry, cfg Config) int {
	defs := AllSkills(cfg)
	count := 0
	for _, d := range defs {
		if registry.Get(d.ID) != nil {
			continue // Skip already registered.
		}
		skill := &instruments.Skill{
			Executor: d.Executor,
			Meta: instruments.SkillMeta{
				ID:     d.ID,
				Name:   d.Name,
				Type:   d.Type,
				Status: instruments.SkillStatusActive,
			},
		}
		registry.Register(skill)
		count++
	}
	return count
}

// AllSkills returns definitions for all 20 starter skills.
func AllSkills(cfg Config) []SkillDef {
	return []SkillDef{
		// --- Development & Code (5) ---
		{ID: "skill_code_exec", Name: "Code Execution", Category: "dev", Description: "Run Python/JS/Bash in Docker sandbox", Type: instruments.SkillTypeCode, Executor: NewCodeExecSkill(cfg.Sandbox)},
		{ID: "skill_git", Name: "Git Management", Category: "dev", Description: "Clone, branch, commit, push, PR", Type: instruments.SkillTypeCode, Executor: NewGitSkill(cfg.DataDir)},
		{ID: "skill_testing", Name: "Testing & QA", Category: "dev", Description: "Generate and run tests, coverage", Type: instruments.SkillTypeHybrid, Executor: NewTestingSkill(cfg.Sandbox)},
		{ID: "skill_browser", Name: "Browser Automation", Category: "dev", Description: "Playwright UI tests, screenshots", Type: instruments.SkillTypeCode, Executor: NewStubSkill("browser", "Browser automation requires Playwright")},
		{ID: "skill_database", Name: "Database Query", Category: "dev", Description: "SQL queries, migrations, schema analysis", Type: instruments.SkillTypeCode, Executor: NewStubSkill("database", "Database requires connection config")},

		// --- Communication (4) ---
		{ID: "skill_email", Name: "Email Management", Category: "comm", Description: "Read/draft/send via IMAP/SMTP", Type: instruments.SkillTypeCode, Executor: NewStubSkill("email", "Email requires IMAP/SMTP config")},
		{ID: "skill_calendar", Name: "Calendar Integration", Category: "comm", Description: "Schedule, check slots, invitations", Type: instruments.SkillTypeCode, Executor: NewStubSkill("calendar", "Calendar requires CalDAV/API config")},
		{ID: "skill_messaging", Name: "Messaging", Category: "comm", Description: "Slack/Discord/Telegram messaging", Type: instruments.SkillTypeCode, Executor: NewStubSkill("messaging", "Messaging requires platform tokens")},
		{ID: "skill_docs", Name: "Document Collaboration", Category: "comm", Description: "Google Docs, Notion read/edit", Type: instruments.SkillTypeCode, Executor: NewStubSkill("docs", "Document collaboration requires API tokens")},

		// --- Research & Information (4) ---
		{ID: "skill_websearch", Name: "Web Search", Category: "research", Description: "Search + extract data from web", Type: instruments.SkillTypeCode, Executor: NewWebSearchSkill()},
		{ID: "skill_pdf", Name: "PDF & Document Analysis", Category: "research", Description: "Extract text, tables, analyze content", Type: instruments.SkillTypeCode, Executor: NewStubSkill("pdf", "PDF analysis requires poppler or similar")},
		{ID: "skill_aggregation", Name: "Data Aggregation", Category: "research", Description: "Collect data from sources, normalize", Type: instruments.SkillTypeCode, Executor: NewAPIIntegrationSkill()},
		{ID: "skill_monitoring", Name: "Real-time Monitoring", Category: "research", Description: "Track website changes, RSS, prices", Type: instruments.SkillTypeCode, Executor: NewStubSkill("monitoring", "Monitoring requires scheduler + targets config")},

		// --- File & Data Management (3) ---
		{ID: "skill_fileops", Name: "File Operations", Category: "data", Description: "Read/write, organize, pattern search", Type: instruments.SkillTypeCode, Executor: NewFileOpsSkill(cfg.DataDir)},
		{ID: "skill_data_analysis", Name: "Data Analysis", Category: "data", Description: "CSV/JSON processing, statistics", Type: instruments.SkillTypeCode, Executor: NewDataAnalysisSkill()},
		{ID: "skill_knowledge", Name: "Knowledge Base Search", Category: "data", Description: "RAG over documents with semantic search", Type: instruments.SkillTypeCode, Executor: NewKnowledgeSearchSkill(cfg.Store)},

		// --- Automation & Security (4) ---
		{ID: "skill_api", Name: "API Integration", Category: "auto", Description: "REST calls, webhook handling", Type: instruments.SkillTypeCode, Executor: NewAPIIntegrationSkill()},
		{ID: "skill_scheduler", Name: "Scheduled Tasks", Category: "auto", Description: "Cron tasks, reminders, triggers", Type: instruments.SkillTypeCode, Executor: NewSchedulerSkill(cfg.Store)},
		{ID: "skill_audit", Name: "Audit & Logging", Category: "auto", Description: "Action logging, audit trail", Type: instruments.SkillTypeCode, Executor: NewAuditSkill(cfg.Store)},
		{ID: "skill_credentials", Name: "Credential Management", Category: "auto", Description: "Secure API key and token storage", Type: instruments.SkillTypeCode, Executor: NewCredentialSkill(cfg.Store)},
	}
}
