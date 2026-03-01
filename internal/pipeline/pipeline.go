package pipeline

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/overhuman/overhuman/internal/brain"
	"github.com/overhuman/overhuman/internal/budget"
	"github.com/overhuman/overhuman/internal/evolution"
	"github.com/overhuman/overhuman/internal/goals"
	"github.com/overhuman/overhuman/internal/instruments"
	"github.com/overhuman/overhuman/internal/memory"
	"github.com/overhuman/overhuman/internal/observability"
	"github.com/overhuman/overhuman/internal/reflection"
	"github.com/overhuman/overhuman/internal/security"
	"github.com/overhuman/overhuman/internal/senses"
	"github.com/overhuman/overhuman/internal/soul"
	"github.com/overhuman/overhuman/internal/versioning"
)

// RunResult holds the output of a full pipeline run.
type RunResult struct {
	TaskID           string  `json:"task_id"`
	Success          bool    `json:"success"`
	Result           string  `json:"result"`
	QualityScore     float64 `json:"quality_score"`
	CostUSD          float64 `json:"cost_usd"`
	ElapsedMs        int64   `json:"elapsed_ms"`
	Fingerprint      string  `json:"fingerprint,omitempty"`
	AutomationTriggered bool `json:"automation_triggered"`
}

// Dependencies holds all subsystem references the pipeline needs.
type Dependencies struct {
	Soul           *soul.Soul
	LLM            brain.LLMProvider
	Router         *brain.ModelRouter
	Context        *brain.ContextAssembler
	ShortTerm      *memory.ShortTermMemory
	LongTerm       *memory.LongTermMemory
	Patterns       *memory.PatternTracker
	AutoThreshold  int // Default 3: trigger code-skill after K repetitions

	// Phase 2 (optional — nil-safe).
	Skills    *instruments.SkillRegistry
	Goals     *goals.Engine
	Budget    *budget.Tracker
	Generator *instruments.Generator

	// Phase 3 (optional — nil-safe).
	Evolution      *evolution.Engine
	Reflection     *reflection.Engine
	VersionControl *versioning.Controller

	// Phase 4 (optional — nil-safe).
	MicroReflector *reflection.MicroReflector
	SKB            *memory.SharedKnowledgeBase
	Experiments    *evolution.ExperimentManager
	Sandbox        *instruments.DockerSandbox
	Logger         *observability.Logger
	Metrics        *observability.MetricsCollector

	// Fractal agents (optional — nil-safe).
	SubagentMgr *instruments.SubagentManager

	// Security (optional — nil-safe).
	Sanitizer      *security.Sanitizer
	AuditLog       *security.AuditLogger
	PolicyEnforcer *security.PolicyEnforcer
	SecretRegistry *security.SecretRegistry
}

// Pipeline orchestrates the 10-stage execution flow.
type Pipeline struct {
	deps Dependencies
}

// New creates a Pipeline with all dependencies.
func New(deps Dependencies) *Pipeline {
	if deps.AutoThreshold <= 0 {
		deps.AutoThreshold = 3
	}
	return &Pipeline{deps: deps}
}

// Run executes the full 10-stage pipeline for a given input signal.
func (p *Pipeline) Run(ctx context.Context, input senses.UnifiedInput) (*RunResult, error) {
	start := time.Now()
	var totalCost float64

	// --- Pre-stage: Input sanitization ---
	if p.deps.Sanitizer != nil {
		sr := p.deps.Sanitizer.Sanitize(input.Payload)
		if sr.Blocked {
			p.logWarn("input blocked by sanitizer", "reason", sr.BlockReason)
			p.auditLog(security.AuditInputBlocked, security.SeverityWarn,
				"system", "sanitize", input.Payload[:min(50, len(input.Payload))], false,
				map[string]string{"reason": sr.BlockReason})
			return &RunResult{Success: false, Result: "input blocked: " + sr.BlockReason}, nil
		}
		if sr.WasModified {
			input.Payload = sr.Clean
		}
		for _, w := range sr.Warnings {
			p.logWarn("sanitizer warning", "warning", w)
			if len(w) > 10 && w[:10] == "potential " {
				p.auditLog(security.AuditInjectionWarn, security.SeverityWarn,
					"system", "sanitize", w, true, nil)
			}
		}
	}

	// --- Stage 1: Intake ---
	taskSpec := p.intake(input)
	p.logPipeline(1, "intake", "task_id", taskSpec.ID)
	p.incrementMetric("pipeline.runs")

	// --- Stage 2: Clarification ---
	if err := p.clarify(ctx, taskSpec, &totalCost); err != nil {
		p.incrementMetric("pipeline.errors")
		return p.failResult(taskSpec, start, totalCost, err), err
	}
	p.logPipeline(2, "clarified", "version", taskSpec.Version)
	p.microCheck(ctx, taskSpec, reflection.StepClarify, taskSpec.Context)

	// --- Stage 3: Planning ---
	if err := p.plan(ctx, taskSpec, &totalCost); err != nil {
		p.incrementMetric("pipeline.errors")
		return p.failResult(taskSpec, start, totalCost, err), err
	}
	p.logPipeline(3, "planned", "subtasks", len(taskSpec.Subtasks))

	// --- Stage 4: Agent Selection ---
	p.selectAgent(taskSpec)
	p.logPipeline(4, "agent selection done")

	// --- Stage 5: Execution ---
	result, err := p.execute(ctx, taskSpec, &totalCost)
	if err != nil {
		p.incrementMetric("pipeline.errors")
		return p.failResult(taskSpec, start, totalCost, err), err
	}
	p.logPipeline(5, "executed")
	p.microCheck(ctx, taskSpec, reflection.StepExecute, result)

	// --- Stage 6: Review ---
	quality, reviewNotes, err := p.review(ctx, taskSpec, result, &totalCost)
	if err != nil {
		p.incrementMetric("pipeline.errors")
		return p.failResult(taskSpec, start, totalCost, err), err
	}
	taskSpec.QualityScore = quality
	taskSpec.ReviewNotes = reviewNotes
	p.logPipeline(6, "reviewed", "quality", quality)
	p.microCheck(ctx, taskSpec, reflection.StepReview, reviewNotes)

	// --- Stage 7: Memory Update ---
	p.updateMemory(taskSpec, result)
	p.logPipeline(7, "memory updated")

	// --- Stage 8: Pattern Tracking ---
	automatable := p.trackPattern(taskSpec)
	p.logPipeline(8, "pattern tracked", "automatable", automatable)
	p.recordMetric(observability.MetricPatterns, boolToFloat(automatable), observability.Labels{"fingerprint": taskSpec.Fingerprint})

	// --- Stage 9: Reflection (meso-loop) ---
	if err := p.reflect(ctx, taskSpec, quality, &totalCost); err != nil {
		p.logWarn("reflection error (non-fatal)", "error", err.Error())
	} else {
		p.logPipeline(9, "reflected")
	}
	p.recordMetric(observability.MetricReflection, quality, observability.Labels{"task_id": taskSpec.ID})

	// --- Stage 10: Goal Update ---
	p.updateGoals(taskSpec, automatable)
	p.logPipeline(10, "goals updated")

	// --- Phase 3: Post-run hooks ---
	p.evolve(taskSpec, quality)
	p.observeVersion(taskSpec, quality)

	// --- Phase 4: Post-run metrics ---
	p.recordMetric(observability.MetricQuality, quality, observability.Labels{"task_id": taskSpec.ID})
	p.recordMetric(observability.MetricCost, totalCost, observability.Labels{"task_id": taskSpec.ID})
	p.recordMetric(observability.MetricLatency, float64(time.Since(start).Milliseconds()), observability.Labels{"task_id": taskSpec.ID})
	p.propagateSKB(taskSpec, quality)

	taskSpec.Advance(TaskStatusCompleted)

	// --- Post: Sanitize output secrets ---
	if p.deps.SecretRegistry != nil {
		result = p.deps.SecretRegistry.Sanitize(result)
	}

	return &RunResult{
		TaskID:              taskSpec.ID,
		Success:             true,
		Result:              result,
		QualityScore:        quality,
		CostUSD:             totalCost,
		ElapsedMs:           time.Since(start).Milliseconds(),
		Fingerprint:         taskSpec.Fingerprint,
		AutomationTriggered: automatable,
	}, nil
}

// --- Stage implementations ---

// Stage 1: Intake — convert UnifiedInput to TaskSpec.
func (p *Pipeline) intake(input senses.UnifiedInput) *TaskSpec {
	ts := NewTaskSpec(
		fmt.Sprintf("task_%d", time.Now().UnixNano()),
		input.Payload,
	)
	ts.SourceChannel = string(input.SourceType)
	ts.SourceUserID = input.SourceMeta.Sender
	return ts
}

// Stage 2: Clarification — LLM refines the task spec.
func (p *Pipeline) clarify(ctx context.Context, ts *TaskSpec, cost *float64) error {
	soulContent, _ := p.deps.Soul.Read()

	messages := p.deps.Context.Assemble(brain.ContextLayers{
		SystemPrompt: soulContent,
		TaskDescription: fmt.Sprintf(
			"Clarify this task. Extract: goal, constraints, expected output, verification criteria.\n\nTask: %s\n\nRespond in this exact format:\nGOAL: <clarified goal>\nCONSTRAINTS: <comma-separated>\nEXPECTED_OUTPUT: <what to produce>\nVERIFICATION: <how to verify>",
			ts.Goal),
	})

	model := p.deps.Router.Select("simple", ts.BudgetUSD)
	resp, err := p.deps.LLM.Complete(ctx, brain.LLMRequest{
		Messages: messages,
		Model:    model,
	})
	if err != nil {
		return fmt.Errorf("clarify: %w", err)
	}
	*cost += resp.CostUSD

	// Parse response (simplified — in production would use structured output).
	ts.Context = resp.Content
	ts.Advance(TaskStatusClarified)
	return nil
}

// Stage 3: Planning — decompose into subtasks.
func (p *Pipeline) plan(ctx context.Context, ts *TaskSpec, cost *float64) error {
	soulContent, _ := p.deps.Soul.Read()

	messages := p.deps.Context.Assemble(brain.ContextLayers{
		SystemPrompt: soulContent,
		TaskDescription: fmt.Sprintf(
			"Decompose this task into subtasks. For simple tasks, a single subtask is fine.\n\nTask: %s\nContext: %s\n\nRespond with a numbered list of subtasks.",
			ts.Goal, ts.Context),
	})

	model := p.deps.Router.Select("moderate", ts.BudgetUSD)
	resp, err := p.deps.LLM.Complete(ctx, brain.LLMRequest{
		Messages: messages,
		Model:    model,
	})
	if err != nil {
		return fmt.Errorf("plan: %w", err)
	}
	*cost += resp.CostUSD

	// For now, create a single subtask from the planning response.
	ts.Subtasks = []SubtaskSpec{
		{
			ID:     ts.ID + "_sub1",
			Goal:   ts.Goal,
			Status: TaskStatusDraft,
		},
	}
	ts.Advance(TaskStatusPlanned)
	return nil
}

// Stage 4: Agent Selection — select agent/skill for each subtask.
// Priority: 1) existing code/hybrid skill, 2) subagent by role match, 3) self (LLM).
func (p *Pipeline) selectAgent(ts *TaskSpec) {
	for i := range ts.Subtasks {
		// 1. Try to find an existing skill for this pattern.
		if p.deps.Skills != nil && ts.Fingerprint != "" {
			if skill := p.deps.Skills.FindActive(ts.Fingerprint); skill != nil {
				ts.Subtasks[i].AssignedTo = "skill:" + skill.Meta.ID
				continue
			}
		}

		// 2. Try to delegate to a subagent (if SubagentManager is available
		//    and the subtask has an agent: prefix hint from planning).
		if p.deps.SubagentMgr != nil && len(ts.Subtasks[i].AssignedTo) > 6 &&
			ts.Subtasks[i].AssignedTo[:6] == "agent:" {
			// Keep the agent assignment from the planning stage.
			continue
		}

		ts.Subtasks[i].AssignedTo = "self"
	}
}

// Stage 5: Execution — execute subtasks and collect results.
func (p *Pipeline) execute(ctx context.Context, ts *TaskSpec, cost *float64) (string, error) {
	ts.Advance(TaskStatusExecuting)

	// Check budget before execution.
	if p.deps.Budget != nil && !p.deps.Budget.CanSpend(0.01) {
		return "", fmt.Errorf("execute: daily/monthly budget exhausted")
	}

	// Use DAG executor for multi-subtask parallel execution.
	if len(ts.Subtasks) > 1 {
		return p.executeDAG(ctx, ts, cost)
	}

	// Single subtask path (optimized).
	return p.executeSingle(ctx, ts, cost)
}

// executeDAG runs multiple subtasks in parallel using the DAG executor.
func (p *Pipeline) executeDAG(ctx context.Context, ts *TaskSpec, cost *float64) (string, error) {
	dag := NewDAGExecutor(func(ctx context.Context, sub *SubtaskSpec) (string, error) {
		return p.executeSubtask(ctx, ts, sub, cost)
	})

	results, err := dag.Execute(ctx, ts.Subtasks)
	if err != nil {
		return "", fmt.Errorf("execute DAG: %w", err)
	}
	ts.Subtasks = results

	// Combine results.
	var combined string
	for _, r := range results {
		if r.Status == TaskStatusCompleted {
			if combined != "" {
				combined += "\n\n"
			}
			combined += r.Result
		}
	}
	return combined, nil
}

// executeSingle handles the common case of a single subtask.
func (p *Pipeline) executeSingle(ctx context.Context, ts *TaskSpec, cost *float64) (string, error) {
	if len(ts.Subtasks) == 0 {
		return p.executeLLM(ctx, ts, cost)
	}
	result, err := p.executeSubtask(ctx, ts, &ts.Subtasks[0], cost)
	if err != nil {
		return "", err
	}
	ts.Subtasks[0].Result = result
	ts.Subtasks[0].Status = TaskStatusCompleted
	return result, nil
}

// executeSubtask runs a single subtask, trying skill → subagent → LLM fallback.
func (p *Pipeline) executeSubtask(ctx context.Context, ts *TaskSpec, sub *SubtaskSpec, cost *float64) (string, error) {
	// 1. Try skill execution first if assigned.
	if p.deps.Skills != nil {
		assignee := sub.AssignedTo
		if len(assignee) > 6 && assignee[:6] == "skill:" {
			skillID := assignee[6:]
			if skill := p.deps.Skills.Get(skillID); skill != nil {
				out, err := skill.Executor.Execute(ctx, instruments.SkillInput{
					Goal:    sub.Goal,
					Context: ts.Context,
				})
				if err == nil && out.Success {
					*cost += out.CostUSD
					if p.deps.Budget != nil {
						p.deps.Budget.Record(ts.ID, out.CostUSD)
					}
					skill.RecordRun(out)
					p.logInfo("skill executed", "subtask", sub.ID, "skill", skillID, "cost", out.CostUSD)
					p.recordMetric(observability.MetricFitness, 1.0, observability.Labels{"skill_id": skillID})
					return out.Result, nil
				}
				p.logWarn("skill failed, falling back to LLM", "skill", skillID, "subtask", sub.ID)
			}
		}
	}

	// 2. Try subagent delegation if assigned.
	if p.deps.SubagentMgr != nil {
		assignee := sub.AssignedTo
		if len(assignee) > 6 && assignee[:6] == "agent:" {
			agentID := assignee[6:]
			result, err := p.deps.SubagentMgr.Delegate(ctx, "pipeline", agentID, instruments.DelegatedTask{
				Goal:    sub.Goal,
				Context: ts.Context,
			})
			if err == nil && result.Success {
				*cost += result.CostUSD
				if p.deps.Budget != nil {
					p.deps.Budget.Record(ts.ID, result.CostUSD)
				}
				p.logInfo("subagent executed", "subtask", sub.ID, "agent", agentID, "quality", result.Quality)
				return result.Output, nil
			}
			if err != nil {
				p.logWarn("subagent delegation failed, falling back to LLM",
					"agent", agentID, "subtask", sub.ID, "error", err.Error())
			}
		}
	}

	// 3. LLM fallback.
	return p.executeLLM(ctx, ts, cost)
}

// executeLLM executes via LLM provider.
func (p *Pipeline) executeLLM(ctx context.Context, ts *TaskSpec, cost *float64) (string, error) {
	budgetRemaining := ts.BudgetUSD
	if p.deps.Budget != nil {
		budgetRemaining = p.deps.Budget.EffectiveBudget()
	}

	soulContent, _ := p.deps.Soul.Read()

	recentEntries := p.deps.ShortTerm.GetRecent(5)
	var history []brain.Message
	for _, e := range recentEntries {
		history = append(history, brain.Message{Role: e.Role, Content: e.Content})
	}

	messages := p.deps.Context.Assemble(brain.ContextLayers{
		SystemPrompt:    soulContent,
		TaskDescription: ts.Goal,
		RecentHistory:   history,
	})

	model := p.deps.Router.Select("moderate", budgetRemaining)
	resp, err := p.deps.LLM.Complete(ctx, brain.LLMRequest{
		Messages:  messages,
		Model:     model,
		MaxTokens: 4096,
	})
	if err != nil {
		return "", fmt.Errorf("execute: %w", err)
	}
	*cost += resp.CostUSD
	if p.deps.Budget != nil {
		p.deps.Budget.Record(ts.ID, resp.CostUSD)
	}

	return resp.Content, nil
}

// Stage 6: Review — evaluate quality of execution.
func (p *Pipeline) review(ctx context.Context, ts *TaskSpec, result string, cost *float64) (float64, string, error) {
	ts.Advance(TaskStatusReviewing)

	soulContent, _ := p.deps.Soul.Read()

	messages := p.deps.Context.Assemble(brain.ContextLayers{
		SystemPrompt: soulContent,
		TaskDescription: fmt.Sprintf(
			"Review this task result. Rate quality from 0.0 to 1.0.\n\nOriginal task: %s\nResult: %s\n\nRespond in this format:\nSCORE: <0.0-1.0>\nNOTES: <brief assessment>",
			ts.Goal, result),
	})

	model := p.deps.Router.Select("simple", ts.BudgetUSD)
	resp, err := p.deps.LLM.Complete(ctx, brain.LLMRequest{
		Messages: messages,
		Model:    model,
	})
	if err != nil {
		return 0.5, "review failed", fmt.Errorf("review: %w", err)
	}
	*cost += resp.CostUSD

	// Default quality; in production would parse SCORE from response.
	return 0.8, resp.Content, nil
}

// Stage 7: Memory Update — store results in short and long term memory.
func (p *Pipeline) updateMemory(ts *TaskSpec, result string) {
	// Short-term: add the interaction.
	p.deps.ShortTerm.Add("user", ts.Goal, map[string]string{
		"task_id": ts.ID,
		"channel": ts.SourceChannel,
	})
	p.deps.ShortTerm.Add("assistant", result, map[string]string{
		"task_id": ts.ID,
		"quality": fmt.Sprintf("%.2f", ts.QualityScore),
	})

	// Long-term: store a summary.
	p.deps.LongTerm.Store(memory.LongTermEntry{
		ID:          ts.ID,
		Summary:     fmt.Sprintf("Task: %s → Quality: %.2f", ts.Goal, ts.QualityScore),
		Tags:        []string{ts.SourceChannel, ts.Fingerprint},
		SourceRunID: ts.ID,
	})
}

// Stage 8: Pattern Tracking — fingerprint and count.
func (p *Pipeline) trackPattern(ts *TaskSpec) bool {
	fingerprint := p.deps.Patterns.ComputeFingerprint(ts.Goal, ts.SourceChannel)
	ts.Fingerprint = fingerprint

	entry, _ := p.deps.Patterns.Record(fingerprint, ts.Goal, ts.QualityScore)
	if entry != nil && entry.Count >= p.deps.AutoThreshold && entry.SkillID == "" {
		return true // Should trigger code-skill generation
	}
	return false
}

// Stage 9: Meso-reflection — per-run reflection on what went well/poorly.
func (p *Pipeline) reflect(ctx context.Context, ts *TaskSpec, quality float64, cost *float64) error {
	// Use Phase 3 reflection engine if available.
	if p.deps.Reflection != nil {
		return p.reflectPhase3(ctx, ts, quality, cost)
	}

	// Phase 1 fallback: simple LLM-based reflection.
	soulContent, _ := p.deps.Soul.Read()

	messages := p.deps.Context.Assemble(brain.ContextLayers{
		SystemPrompt: soulContent,
		TaskDescription: fmt.Sprintf(
			"Reflect on this completed task. What went well? What could be improved? Suggest one concrete improvement.\n\nTask: %s\nQuality: %.2f\nNotes: %s",
			ts.Goal, quality, ts.ReviewNotes),
	})

	model := p.deps.Router.Select("simple", ts.BudgetUSD)
	resp, err := p.deps.LLM.Complete(ctx, brain.LLMRequest{
		Messages: messages,
		Model:    model,
	})
	if err != nil {
		return err
	}
	*cost += resp.CostUSD

	// Store reflection in long-term memory.
	p.deps.LongTerm.Store(memory.LongTermEntry{
		ID:          ts.ID + "_reflection",
		Summary:     fmt.Sprintf("Reflection on %s: %s", ts.ID, resp.Content),
		Tags:        []string{"reflection", "meso"},
		SourceRunID: ts.ID,
	})

	return nil
}

// reflectPhase3 uses the full reflection engine with meso + macro support.
func (p *Pipeline) reflectPhase3(ctx context.Context, ts *TaskSpec, quality float64, cost *float64) error {
	soulContent, _ := p.deps.Soul.Read()

	// Meso-reflection.
	summary := reflection.RunSummary{
		TaskID:        ts.ID,
		Goal:          ts.Goal,
		QualityScore:  quality,
		ReviewNotes:   ts.ReviewNotes,
		CostUSD:       *cost,
		Fingerprint:   ts.Fingerprint,
		SourceChannel: ts.SourceChannel,
	}

	_, mesoCost, err := p.deps.Reflection.Meso(ctx, soulContent, summary)
	if err != nil {
		return fmt.Errorf("meso reflection: %w", err)
	}
	*cost += mesoCost

	// Check if macro-reflection should run.
	if p.deps.Reflection.ShouldRunMacro() {
		macroSummary := reflection.MacroSummary{
			TotalRuns:  p.deps.Reflection.RunsSinceMacro(),
			AvgQuality: quality, // Simplified — in production would aggregate.
			AvgCostUSD: *cost,
		}
		if p.deps.Skills != nil {
			macroSummary.SkillCount = p.deps.Skills.Count()
		}

		_, macroCost, macroErr := p.deps.Reflection.Macro(ctx, soulContent, macroSummary)
		if macroErr != nil {
			p.logWarn("macro-reflection error (non-fatal)", "error", macroErr.Error())
		} else {
			*cost += macroCost
			p.logInfo("macro-reflection completed")
		}
	}

	return nil
}

// Stage 10: Goal Update — create goals based on patterns and quality.
func (p *Pipeline) updateGoals(ts *TaskSpec, automatable bool) {
	if p.deps.Goals == nil {
		// Phase 1 fallback: just log.
		if automatable {
			p.logInfo("goal: generate code-skill (no GoalEngine)", "fingerprint", ts.Fingerprint)
		}
		return
	}

	if automatable {
		p.deps.Goals.AddWithMeta(
			fmt.Sprintf("Generate code-skill for pattern %s", ts.Fingerprint),
			goals.GoalSourcePattern,
			goals.GoalPriorityHigh,
			map[string]string{
				"fingerprint": ts.Fingerprint,
				"goal":        ts.Goal,
				"channel":     ts.SourceChannel,
			},
		)
		p.logInfo("goal added: generate code-skill", "fingerprint", ts.Fingerprint)
	}

	if ts.QualityScore < 0.5 {
		p.deps.Goals.AddWithMeta(
			fmt.Sprintf("Investigate low quality for task type %s", ts.SourceChannel),
			goals.GoalSourceReflection,
			goals.GoalPriorityNormal,
			map[string]string{
				"task_id": ts.ID,
				"quality": fmt.Sprintf("%.2f", ts.QualityScore),
			},
		)
	}
}

// evolve evaluates skill fitness and triggers deprecation if needed.
func (p *Pipeline) evolve(ts *TaskSpec, quality float64) {
	if p.deps.Evolution == nil || p.deps.Skills == nil {
		return
	}

	// Check all active A/B tests.
	for _, test := range p.deps.Evolution.ActiveTests() {
		winner, loserID, decided := p.deps.Evolution.EvaluateABTest(test.ID, p.deps.Skills)
		if decided {
			p.logInfo("A/B test decided", "test_id", test.ID, "winner", winner, "loser", loserID)
			_ = p.deps.Skills.UpdateStatus(loserID, instruments.SkillStatusDeprecated)
			_ = p.deps.Skills.UpdateStatus(winner, instruments.SkillStatusActive)
		}
	}

	// Evaluate all skills for deprecation.
	deprecated := p.deps.Evolution.EvaluateAll(p.deps.Skills)
	for _, id := range deprecated {
		_ = p.deps.Skills.UpdateStatus(id, instruments.SkillStatusDeprecated)
		p.logInfo("deprecated skill (low fitness)", "skill_id", id)
	}
}

// observeVersion records a run against active observation windows.
func (p *Pipeline) observeVersion(ts *TaskSpec, quality float64) {
	if p.deps.VersionControl == nil {
		return
	}

	// Observe against all entity types that might have pending changes.
	rollbacks := p.deps.VersionControl.ObserveRun(ts.Fingerprint, quality, ts.QualityScore)
	for _, changeID := range rollbacks {
		ch := p.deps.VersionControl.Get(changeID)
		if ch != nil {
			p.logWarn("auto-rollback triggered", "description", ch.Description, "entity", ch.EntityID)
		}
	}
}

// microCheck runs micro-reflection on a pipeline step if available.
func (p *Pipeline) microCheck(ctx context.Context, ts *TaskSpec, step reflection.StepName, stepResult string) {
	if p.deps.MicroReflector == nil {
		return
	}
	sr := reflection.StepResult{
		Step:   step,
		Output: stepResult,
	}
	verdict, confidence, err := p.deps.MicroReflector.Check(ctx, ts.Goal, sr)
	if err != nil {
		p.logWarn("micro-reflection error", "step", string(step), "error", err.Error())
		return
	}
	if !verdict.OK {
		p.logWarn("micro-reflection issue",
			"step", string(step),
			"confidence", confidence,
			"issue", verdict.Issue,
			"suggestion", verdict.Suggestion,
		)
	}
}

// propagateSKB stores high-quality insights in the Shared Knowledge Base.
func (p *Pipeline) propagateSKB(ts *TaskSpec, quality float64) {
	if p.deps.SKB == nil {
		return
	}
	// Only store entries with quality >= 0.7 as insights.
	if quality < 0.7 {
		return
	}
	entry := memory.SKBEntry{
		ID:          ts.ID + "_insight",
		Type:        memory.SKBInsight,
		SourceAgent: "pipeline",
		Content:     fmt.Sprintf("Task: %s → quality %.2f", ts.Goal, quality),
		Tags:        []string{ts.SourceChannel, ts.Fingerprint},
		Fitness:     quality,
	}
	if err := p.deps.SKB.Store(entry); err != nil {
		p.logWarn("SKB store error", "error", err.Error())
	}
}

func boolToFloat(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}

// logInfo logs using structured logger if available, falling back to log.Printf.
func (p *Pipeline) logInfo(msg string, args ...any) {
	if p.deps.Logger != nil {
		p.deps.Logger.Info(msg, args...)
	} else {
		log.Printf("[pipeline] %s%s", msg, formatLogArgs(args))
	}
}

// logWarn logs a warning.
func (p *Pipeline) logWarn(msg string, args ...any) {
	if p.deps.Logger != nil {
		p.deps.Logger.Warn(msg, args...)
	} else {
		log.Printf("[pipeline] WARN: %s%s", msg, formatLogArgs(args))
	}
}

// formatLogArgs formats key-value pairs for fallback log output.
func formatLogArgs(args []any) string {
	if len(args) == 0 {
		return ""
	}
	s := ""
	for i := 0; i+1 < len(args); i += 2 {
		s += fmt.Sprintf(" %v=%v", args[i], args[i+1])
	}
	return s
}

// logPipeline logs a pipeline stage event.
func (p *Pipeline) logPipeline(stage int, msg string, args ...any) {
	if p.deps.Logger != nil {
		p.deps.Logger.Pipeline(stage, 10, msg, args...)
	} else {
		log.Printf("[pipeline] stage %d/10: %s", stage, msg)
	}
}

// recordMetric records a metric data point if the collector is available.
func (p *Pipeline) recordMetric(mt observability.MetricType, value float64, labels observability.Labels) {
	if p.deps.Metrics != nil {
		p.deps.Metrics.Record(mt, value, labels)
	}
}

// incrementMetric increments a named counter if the collector is available.
func (p *Pipeline) incrementMetric(name string) {
	if p.deps.Metrics != nil {
		p.deps.Metrics.Increment(name)
	}
}

// auditLog records a security audit event if the logger is available.
func (p *Pipeline) auditLog(eventType security.AuditEventType, severity security.AuditSeverity, actor, action, resource string, success bool, details map[string]string) {
	if p.deps.AuditLog != nil {
		p.deps.AuditLog.Log(eventType, severity, "pipeline", actor, action, resource, success, details)
	}
}

// min returns the smaller of two ints.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (p *Pipeline) failResult(ts *TaskSpec, start time.Time, cost float64, err error) *RunResult {
	ts.Advance(TaskStatusFailed)
	p.recordMetric(observability.MetricErrors, 1, observability.Labels{"task_id": ts.ID})
	return &RunResult{
		TaskID:    ts.ID,
		Success:   false,
		Result:    err.Error(),
		CostUSD:   cost,
		ElapsedMs: time.Since(start).Milliseconds(),
	}
}
