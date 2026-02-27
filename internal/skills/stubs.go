package skills

import (
	"context"
	"fmt"

	"github.com/overhuman/overhuman/internal/instruments"
)

// StubSkill is a placeholder for skills that require external services.
// It returns a descriptive error explaining what is needed to enable the skill.
type StubSkill struct {
	name   string
	reason string
}

// NewStubSkill creates a stub skill with a descriptive reason.
func NewStubSkill(name, reason string) *StubSkill {
	return &StubSkill{name: name, reason: reason}
}

func (s *StubSkill) Execute(_ context.Context, input instruments.SkillInput) (*instruments.SkillOutput, error) {
	return &instruments.SkillOutput{
		Result:  fmt.Sprintf("[%s] Skill not yet configured: %s\nGoal: %s", s.name, s.reason, input.Goal),
		Success: false,
		Error:   s.reason,
	}, nil
}
