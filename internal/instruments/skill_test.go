package instruments

import (
	"context"
	"errors"
	"testing"
	"time"
)

// --- SkillExecutor Tests ---

func TestLLMSkill_Execute(t *testing.T) {
	fn := func(ctx context.Context, input SkillInput) (*SkillOutput, error) {
		return &SkillOutput{
			Result:    "LLM response for: " + input.Goal,
			Success:   true,
			CostUSD:   0.003,
			ElapsedMs: 500,
		}, nil
	}

	skill := NewLLMSkill(fn)
	out, err := skill.Execute(context.Background(), SkillInput{Goal: "test task"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !out.Success {
		t.Error("expected success")
	}
	if out.CostUSD != 0.003 {
		t.Errorf("cost = %f", out.CostUSD)
	}
	if out.Result != "LLM response for: test task" {
		t.Errorf("result = %q", out.Result)
	}
}

func TestCodeSkill_Execute(t *testing.T) {
	fn := func(ctx context.Context, input SkillInput) (*SkillOutput, error) {
		return &SkillOutput{
			Result:  "code result for: " + input.Goal,
			Success: true,
		}, nil
	}

	skill := NewCodeSkill(fn, "go", "func() { return 42 }")
	out, err := skill.Execute(context.Background(), SkillInput{Goal: "compute"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !out.Success {
		t.Error("expected success")
	}
	if out.CostUSD != 0 {
		t.Errorf("code skill cost should be 0, got %f", out.CostUSD)
	}
	if out.ElapsedMs < 0 {
		t.Error("elapsed should be >= 0")
	}
	if skill.Language != "go" {
		t.Errorf("language = %q", skill.Language)
	}
}

func TestCodeSkill_Error(t *testing.T) {
	fn := func(ctx context.Context, input SkillInput) (*SkillOutput, error) {
		return nil, errors.New("division by zero")
	}

	skill := NewCodeSkill(fn, "python", "1/0")
	out, err := skill.Execute(context.Background(), SkillInput{Goal: "fail"})
	if err == nil {
		t.Fatal("expected error")
	}
	if out.Success {
		t.Error("should not be success")
	}
	if out.Error != "division by zero" {
		t.Errorf("error = %q", out.Error)
	}
}

func TestHybridSkill_CodeSuccess(t *testing.T) {
	codeFn := func(ctx context.Context, input SkillInput) (*SkillOutput, error) {
		return &SkillOutput{Result: "code result", Success: true}, nil
	}
	llmFn := func(ctx context.Context, input SkillInput) (*SkillOutput, error) {
		t.Error("LLM should not be called when code succeeds")
		return nil, nil
	}

	hybrid := NewHybridSkill(
		NewCodeSkill(codeFn, "go", ""),
		NewLLMSkill(llmFn),
	)

	out, err := hybrid.Execute(context.Background(), SkillInput{Goal: "test"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.Result != "code result" {
		t.Errorf("result = %q, expected code result", out.Result)
	}
}

func TestHybridSkill_FallbackToLLM(t *testing.T) {
	codeFn := func(ctx context.Context, input SkillInput) (*SkillOutput, error) {
		return nil, errors.New("code failed")
	}
	llmFn := func(ctx context.Context, input SkillInput) (*SkillOutput, error) {
		return &SkillOutput{Result: "llm fallback", Success: true, CostUSD: 0.01}, nil
	}

	hybrid := NewHybridSkill(
		NewCodeSkill(codeFn, "go", ""),
		NewLLMSkill(llmFn),
	)

	out, err := hybrid.Execute(context.Background(), SkillInput{Goal: "test"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.Result != "llm fallback" {
		t.Errorf("result = %q, expected llm fallback", out.Result)
	}
	if out.CostUSD != 0.01 {
		t.Errorf("cost = %f, expected LLM cost", out.CostUSD)
	}
}

// --- Skill RecordRun Tests ---

func TestSkill_RecordRun(t *testing.T) {
	s := &Skill{
		Meta: SkillMeta{
			ID:   "skill_1",
			Type: SkillTypeLLM,
		},
	}

	s.RecordRun(&SkillOutput{Success: true, CostUSD: 0.01, ElapsedMs: 100})
	s.RecordRun(&SkillOutput{Success: true, CostUSD: 0.02, ElapsedMs: 200})
	s.RecordRun(&SkillOutput{Success: false, CostUSD: 0.03, ElapsedMs: 300})

	if s.Meta.TotalRuns != 3 {
		t.Errorf("TotalRuns = %d, want 3", s.Meta.TotalRuns)
	}

	// 2 out of 3 succeeded.
	expectedRate := 2.0 / 3.0
	if diff := s.Meta.SuccessRate - expectedRate; diff > 0.01 || diff < -0.01 {
		t.Errorf("SuccessRate = %f, want ~%f", s.Meta.SuccessRate, expectedRate)
	}

	// Average cost: (0.01 + 0.02 + 0.03) / 3 = 0.02
	if diff := s.Meta.AvgCostUSD - 0.02; diff > 0.001 || diff < -0.001 {
		t.Errorf("AvgCostUSD = %f, want ~0.02", s.Meta.AvgCostUSD)
	}

	// Average elapsed: (100 + 200 + 300) / 3 = 200
	if diff := s.Meta.AvgElapsedMs - 200; diff > 1 || diff < -1 {
		t.Errorf("AvgElapsedMs = %f, want ~200", s.Meta.AvgElapsedMs)
	}
}

// --- SkillRegistry Tests ---

func TestSkillRegistry_RegisterAndGet(t *testing.T) {
	reg := NewSkillRegistry()

	skill := &Skill{
		Meta: SkillMeta{
			ID:   "skill_1",
			Name: "Summarizer",
			Type: SkillTypeLLM,
		},
	}
	reg.Register(skill)

	got := reg.Get("skill_1")
	if got == nil {
		t.Fatal("expected to find skill")
	}
	if got.Meta.Name != "Summarizer" {
		t.Errorf("Name = %q", got.Meta.Name)
	}

	if reg.Get("nonexistent") != nil {
		t.Error("should return nil for unknown skill")
	}
}

func TestSkillRegistry_Count(t *testing.T) {
	reg := NewSkillRegistry()
	if reg.Count() != 0 {
		t.Error("empty registry should have 0 skills")
	}

	reg.Register(&Skill{Meta: SkillMeta{ID: "a"}})
	reg.Register(&Skill{Meta: SkillMeta{ID: "b"}})
	if reg.Count() != 2 {
		t.Errorf("Count = %d, want 2", reg.Count())
	}
}

func TestSkillRegistry_FindByFingerprint(t *testing.T) {
	reg := NewSkillRegistry()

	reg.Register(&Skill{Meta: SkillMeta{ID: "s1", Fingerprint: "fp_abc", Type: SkillTypeLLM, Status: SkillStatusActive}})
	reg.Register(&Skill{Meta: SkillMeta{ID: "s2", Fingerprint: "fp_abc", Type: SkillTypeCode, Status: SkillStatusActive}})
	reg.Register(&Skill{Meta: SkillMeta{ID: "s3", Fingerprint: "fp_xyz", Type: SkillTypeLLM, Status: SkillStatusActive}})

	found := reg.FindByFingerprint("fp_abc")
	if len(found) != 2 {
		t.Fatalf("expected 2 skills for fp_abc, got %d", len(found))
	}

	found2 := reg.FindByFingerprint("fp_xyz")
	if len(found2) != 1 {
		t.Fatalf("expected 1 skill for fp_xyz, got %d", len(found2))
	}

	found3 := reg.FindByFingerprint("nonexistent")
	if len(found3) != 0 {
		t.Errorf("expected 0 skills for nonexistent, got %d", len(found3))
	}
}

func TestSkillRegistry_FindActive(t *testing.T) {
	reg := NewSkillRegistry()

	// LLM skill.
	reg.Register(&Skill{Meta: SkillMeta{
		ID: "llm_1", Fingerprint: "fp_1", Type: SkillTypeLLM,
		Status: SkillStatusActive, SuccessRate: 0.9,
	}})
	// Code skill (should be preferred).
	reg.Register(&Skill{Meta: SkillMeta{
		ID: "code_1", Fingerprint: "fp_1", Type: SkillTypeCode,
		Status: SkillStatusActive, SuccessRate: 0.85,
	}})
	// Deprecated skill (should be ignored).
	reg.Register(&Skill{Meta: SkillMeta{
		ID: "dep_1", Fingerprint: "fp_1", Type: SkillTypeCode,
		Status: SkillStatusDeprecated, SuccessRate: 0.99,
	}})

	best := reg.FindActive("fp_1")
	if best == nil {
		t.Fatal("expected to find active skill")
	}
	if best.Meta.ID != "code_1" {
		t.Errorf("expected code_1 (CODE preferred), got %s", best.Meta.ID)
	}
}

func TestSkillRegistry_FindActive_SameTypePrefersHigherSuccess(t *testing.T) {
	reg := NewSkillRegistry()

	reg.Register(&Skill{Meta: SkillMeta{
		ID: "code_a", Fingerprint: "fp_2", Type: SkillTypeCode,
		Status: SkillStatusActive, SuccessRate: 0.7,
	}})
	reg.Register(&Skill{Meta: SkillMeta{
		ID: "code_b", Fingerprint: "fp_2", Type: SkillTypeCode,
		Status: SkillStatusActive, SuccessRate: 0.95,
	}})

	best := reg.FindActive("fp_2")
	if best.Meta.ID != "code_b" {
		t.Errorf("expected code_b (higher success rate), got %s", best.Meta.ID)
	}
}

func TestSkillRegistry_FindActive_None(t *testing.T) {
	reg := NewSkillRegistry()
	if reg.FindActive("unknown") != nil {
		t.Error("should return nil when no skills match")
	}
}

func TestSkillRegistry_UpdateStatus(t *testing.T) {
	reg := NewSkillRegistry()
	reg.Register(&Skill{Meta: SkillMeta{ID: "s1", Status: SkillStatusActive}})

	if err := reg.UpdateStatus("s1", SkillStatusDeprecated); err != nil {
		t.Fatal(err)
	}
	if reg.Get("s1").Meta.Status != SkillStatusDeprecated {
		t.Error("status should be DEPRECATED")
	}

	if err := reg.UpdateStatus("nonexistent", SkillStatusActive); err == nil {
		t.Error("expected error for unknown skill")
	}
}

func TestSkillRegistry_Remove(t *testing.T) {
	reg := NewSkillRegistry()
	reg.Register(&Skill{Meta: SkillMeta{ID: "s1", Fingerprint: "fp_1"}})

	reg.Remove("s1")
	if reg.Get("s1") != nil {
		t.Error("skill should be removed")
	}
	if reg.Count() != 0 {
		t.Errorf("Count = %d, want 0", reg.Count())
	}
	if len(reg.FindByFingerprint("fp_1")) != 0 {
		t.Error("fingerprint index should be cleaned up")
	}

	// Remove nonexistent — should not panic.
	reg.Remove("nonexistent")
}

func TestSkillRegistry_List(t *testing.T) {
	reg := NewSkillRegistry()
	reg.Register(&Skill{Meta: SkillMeta{ID: "a"}})
	reg.Register(&Skill{Meta: SkillMeta{ID: "b"}})

	list := reg.List()
	if len(list) != 2 {
		t.Errorf("List length = %d, want 2", len(list))
	}
}

func TestSkillRegistry_MarshalMeta(t *testing.T) {
	reg := NewSkillRegistry()
	reg.Register(&Skill{Meta: SkillMeta{ID: "s1", Name: "Test", Type: SkillTypeLLM}})

	data, err := reg.MarshalMeta()
	if err != nil {
		t.Fatalf("MarshalMeta: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty JSON")
	}
}

// --- Interface compliance ---

func TestLLMSkill_ImplementsExecutor(t *testing.T) {
	var _ SkillExecutor = (*LLMSkill)(nil)
}

func TestCodeSkill_ImplementsExecutor(t *testing.T) {
	var _ SkillExecutor = (*CodeSkill)(nil)
}

func TestHybridSkill_ImplementsExecutor(t *testing.T) {
	var _ SkillExecutor = (*HybridSkill)(nil)
}

// --- SkillMeta timestamps ---

func TestSkillMeta_Timestamps(t *testing.T) {
	now := time.Now()
	meta := SkillMeta{
		ID:        "test",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if meta.CreatedAt.IsZero() {
		t.Error("created_at should not be zero")
	}
}
