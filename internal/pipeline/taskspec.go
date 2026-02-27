package pipeline

import "time"

// TaskStatus represents the lifecycle stage of a task.
type TaskStatus string

const (
	TaskStatusDraft     TaskStatus = "draft"
	TaskStatusClarified TaskStatus = "clarified"
	TaskStatusPlanned   TaskStatus = "planned"
	TaskStatusExecuting TaskStatus = "executing"
	TaskStatusReviewing TaskStatus = "reviewing"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
)

// SubtaskSpec describes a decomposed subtask within a DAG.
type SubtaskSpec struct {
	ID           string   `json:"id"`
	Goal         string   `json:"goal"`
	DependsOn    []string `json:"depends_on,omitempty"` // IDs of prerequisite subtasks
	AssignedTo   string   `json:"assigned_to,omitempty"` // agent_id or skill_id
	Status       TaskStatus `json:"status"`
	Result       string   `json:"result,omitempty"`
	QualityScore float64  `json:"quality_score,omitempty"`
}

// TaskSpec is a versioned specification of a task flowing through the pipeline.
// It evolves from draft → clarified → planned → executing → completed/failed.
type TaskSpec struct {
	ID                   string       `json:"id"`
	Version              int          `json:"version"` // Incremented at each stage transition
	Status               TaskStatus   `json:"status"`
	Goal                 string       `json:"goal"`
	Context              string       `json:"context,omitempty"`
	Constraints          []string     `json:"constraints,omitempty"`
	ExpectedOutput       string       `json:"expected_output,omitempty"`
	VerificationCriteria []string     `json:"verification_criteria,omitempty"`
	Subtasks             []SubtaskSpec `json:"subtasks,omitempty"`
	BudgetUSD            float64      `json:"budget_usd,omitempty"`
	Fingerprint          string       `json:"fingerprint,omitempty"` // Pattern fingerprint
	QualityScore         float64      `json:"quality_score,omitempty"`
	ReviewNotes          string       `json:"review_notes,omitempty"`
	CreatedAt            time.Time    `json:"created_at"`
	UpdatedAt            time.Time    `json:"updated_at"`

	// Source tracking.
	SourceChannel string `json:"source_channel,omitempty"` // Which sense channel this came from
	SourceUserID  string `json:"source_user_id,omitempty"`
}

// NewTaskSpec creates a draft TaskSpec from a goal string.
func NewTaskSpec(id, goal string) *TaskSpec {
	now := time.Now().UTC()
	return &TaskSpec{
		ID:        id,
		Version:   1,
		Status:    TaskStatusDraft,
		Goal:      goal,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Advance moves the task to the next status and increments version.
func (ts *TaskSpec) Advance(newStatus TaskStatus) {
	ts.Status = newStatus
	ts.Version++
	ts.UpdatedAt = time.Now().UTC()
}
