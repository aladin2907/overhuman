package brain

// Tier represents a model cost/capability tier.
type Tier string

const (
	TierCheap    Tier = "cheap"
	TierMid      Tier = "mid"
	TierPowerful Tier = "powerful"
)

// ModelEntry describes a model with its tier and approximate cost per 1K tokens.
type ModelEntry struct {
	ID           string  // e.g. "claude-haiku-3-5-20241022"
	Provider     string  // e.g. "claude", "openai"
	Tier         Tier
	CostPer1K    float64 // approximate blended cost per 1K tokens in USD
}

// ModelRouter selects the best model based on task complexity and remaining budget.
type ModelRouter struct {
	models   []ModelEntry
	provider string // Active provider filter ("claude", "openai", or "" for any)
}

// NewModelRouter creates a router with default model entries.
func NewModelRouter() *ModelRouter {
	return &ModelRouter{
		models: []ModelEntry{
			{ID: "claude-haiku-3-5-20241022", Provider: "claude", Tier: TierCheap, CostPer1K: 0.00075},
			{ID: "gpt-4o-mini", Provider: "openai", Tier: TierCheap, CostPer1K: 0.000375},
			{ID: "claude-sonnet-4-20250514", Provider: "claude", Tier: TierMid, CostPer1K: 0.009},
			{ID: "gpt-4o", Provider: "openai", Tier: TierMid, CostPer1K: 0.00625},
			{ID: "claude-opus-4-20250514", Provider: "claude", Tier: TierPowerful, CostPer1K: 0.045},
			{ID: "gpt-4.1", Provider: "openai", Tier: TierPowerful, CostPer1K: 0.030},
		},
	}
}

// NewModelRouterWithModels creates a router with custom model entries.
func NewModelRouterWithModels(models []ModelEntry) *ModelRouter {
	return &ModelRouter{models: models}
}

// SetProvider sets the active provider filter. Only models from this provider
// will be returned by Select. Pass "" to disable filtering.
func (r *ModelRouter) SetProvider(provider string) {
	r.provider = provider
}

// Provider returns the current provider filter.
func (r *ModelRouter) Provider() string {
	return r.provider
}

// Select picks the best model based on complexity and remaining budget.
// complexity should be one of: "simple", "moderate", "complex".
// budgetRemaining is in USD.
// If a provider filter is set, only models from that provider are considered.
func (r *ModelRouter) Select(complexity string, budgetRemaining float64) string {
	targetTier := complexityToTier(complexity)

	// If budget is low (less than $0.10), force downgrade to cheap tier.
	if budgetRemaining < 0.10 {
		targetTier = TierCheap
	} else if budgetRemaining < 1.0 && targetTier == TierPowerful {
		// If budget is moderate but not generous, downgrade powerful to mid.
		targetTier = TierMid
	}

	// Find the first model matching the target tier and provider.
	for _, m := range r.models {
		if r.matchesProvider(m) && m.Tier == targetTier {
			return m.ID
		}
	}

	// Fallback: if target tier not found, try progressively cheaper tiers.
	fallbackOrder := tierFallback(targetTier)
	for _, tier := range fallbackOrder {
		for _, m := range r.models {
			if r.matchesProvider(m) && m.Tier == tier {
				return m.ID
			}
		}
	}

	// Absolute fallback: return first available model matching provider.
	for _, m := range r.models {
		if r.matchesProvider(m) {
			return m.ID
		}
	}

	// Last resort: return first model regardless of provider.
	if len(r.models) > 0 {
		return r.models[0].ID
	}
	return ""
}

// matchesProvider checks if a model matches the active provider filter.
func (r *ModelRouter) matchesProvider(m ModelEntry) bool {
	return r.provider == "" || m.Provider == r.provider
}

// complexityToTier maps a complexity string to a model tier.
func complexityToTier(complexity string) Tier {
	switch complexity {
	case "simple":
		return TierCheap
	case "moderate":
		return TierMid
	case "complex":
		return TierPowerful
	default:
		return TierMid
	}
}

// tierFallback returns the fallback order for a given tier.
func tierFallback(tier Tier) []Tier {
	switch tier {
	case TierPowerful:
		return []Tier{TierMid, TierCheap}
	case TierMid:
		return []Tier{TierCheap, TierPowerful}
	case TierCheap:
		return []Tier{TierMid, TierPowerful}
	default:
		return []Tier{TierCheap, TierMid, TierPowerful}
	}
}
