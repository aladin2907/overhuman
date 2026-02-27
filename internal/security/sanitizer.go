// Package security provides defense-in-depth capabilities for the Overhuman
// system: input sanitization, prompt injection detection, rate limiting,
// audit logging, credential encryption, and skill validation.
package security

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// ---------------------------------------------------------------------------
// Input sanitizer — cleans and validates all incoming data
// ---------------------------------------------------------------------------

// SanitizeResult holds the outcome of a sanitization check.
type SanitizeResult struct {
	Clean       string   // The sanitized input
	WasModified bool     // True if the input was changed
	Warnings    []string // Non-blocking concerns
	Blocked     bool     // True if the input should be rejected
	BlockReason string   // Why it was blocked
}

// Sanitizer cleans and validates incoming inputs across all channels.
type Sanitizer struct {
	mu              sync.RWMutex
	maxInputLength  int
	injectionPatterns []*regexp.Regexp
	blocklist       []string
}

// SanitizerConfig holds configuration for the Sanitizer.
type SanitizerConfig struct {
	MaxInputLength int      // Maximum allowed input length (default: 100000)
	ExtraBlocklist []string // Additional blocked phrases
}

// NewSanitizer creates a Sanitizer with injection detection patterns.
func NewSanitizer(cfg SanitizerConfig) *Sanitizer {
	if cfg.MaxInputLength <= 0 {
		cfg.MaxInputLength = 100000
	}

	s := &Sanitizer{
		maxInputLength: cfg.MaxInputLength,
		blocklist:      cfg.ExtraBlocklist,
	}

	// Compile prompt injection detection patterns.
	patterns := []string{
		// Direct instruction override attempts.
		`(?i)ignore\s+(all\s+)?(previous|prior|above)\s+(instructions?|prompts?|rules?)`,
		`(?i)disregard\s+(all\s+)?(previous|prior|above)`,
		`(?i)forget\s+(all\s+)?(previous|prior|above)\s+(instructions?|context)`,
		// Role manipulation.
		`(?i)you\s+are\s+now\s+(a|an|the)\s+`,
		`(?i)act\s+as\s+(a|an|the)\s+(system|admin|root|developer)`,
		`(?i)pretend\s+(you\s+are|to\s+be)\s+(a|an|the)\s+`,
		// System prompt extraction.
		`(?i)(show|reveal|print|output|display)\s+(your\s+)?(system\s+prompt|instructions|rules)`,
		`(?i)what\s+(are|is)\s+your\s+(system\s+prompt|instructions|rules)`,
		// Delimiter injection.
		`(?i)<\/?system>`,
		`(?i)\[INST\]|\[\/INST\]`,
		`(?i)<<SYS>>|<<\/SYS>>`,
		// Code execution attempts via prompt.
		`(?i)(execute|run|eval)\s*\(\s*['"]`,
		`(?i)import\s+os\s*;\s*os\.(system|popen|exec)`,
		`(?i)__import__\s*\(\s*['"]os['"]\s*\)`,
	}

	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err == nil {
			s.injectionPatterns = append(s.injectionPatterns, re)
		}
	}

	return s
}

// Sanitize checks and cleans an input string.
func (s *Sanitizer) Sanitize(input string) SanitizeResult {
	result := SanitizeResult{Clean: input}

	// 1. Check valid UTF-8.
	if !utf8.ValidString(input) {
		result.Clean = strings.ToValidUTF8(input, "")
		result.WasModified = true
		result.Warnings = append(result.Warnings, "invalid UTF-8 sequences removed")
	}

	// 2. Strip null bytes and control characters (except newline, tab).
	cleaned := stripControlChars(result.Clean)
	if cleaned != result.Clean {
		result.Clean = cleaned
		result.WasModified = true
		result.Warnings = append(result.Warnings, "control characters removed")
	}

	// 3. Length check.
	if len(result.Clean) > s.maxInputLength {
		result.Blocked = true
		result.BlockReason = fmt.Sprintf("input exceeds maximum length (%d > %d)", len(result.Clean), s.maxInputLength)
		return result
	}

	// 4. Check blocklist.
	lower := strings.ToLower(result.Clean)
	s.mu.RLock()
	for _, blocked := range s.blocklist {
		if strings.Contains(lower, strings.ToLower(blocked)) {
			result.Blocked = true
			result.BlockReason = fmt.Sprintf("input contains blocked phrase: %q", blocked)
			s.mu.RUnlock()
			return result
		}
	}
	s.mu.RUnlock()

	// 5. Prompt injection detection (warning, not blocking by default).
	for _, re := range s.injectionPatterns {
		if re.MatchString(result.Clean) {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("potential prompt injection detected: %s", re.String()))
		}
	}

	return result
}

// DetectInjection returns true if the input matches prompt injection patterns.
func (s *Sanitizer) DetectInjection(input string) (bool, []string) {
	var matches []string
	for _, re := range s.injectionPatterns {
		if loc := re.FindString(input); loc != "" {
			matches = append(matches, loc)
		}
	}
	return len(matches) > 0, matches
}

// AddBlocklistPhrase adds a phrase to the blocklist at runtime.
func (s *Sanitizer) AddBlocklistPhrase(phrase string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.blocklist = append(s.blocklist, phrase)
}

// stripControlChars removes ASCII control characters except \n (10), \r (13), \t (9).
func stripControlChars(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '\n' || r == '\r' || r == '\t' || r >= 32 {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// Rate limiter — per-source sliding window
// ---------------------------------------------------------------------------

// RateLimiter enforces per-source request rate limits using a sliding window.
type RateLimiter struct {
	mu       sync.Mutex
	windows  map[string]*slidingWindow
	limit    int
	interval time.Duration
}

type slidingWindow struct {
	timestamps []time.Time
}

// NewRateLimiter creates a rate limiter allowing `limit` requests per `interval`
// for each unique source.
func NewRateLimiter(limit int, interval time.Duration) *RateLimiter {
	return &RateLimiter{
		windows:  make(map[string]*slidingWindow),
		limit:    limit,
		interval: interval,
	}
}

// Allow checks if the given source is within rate limits. Returns true if
// the request is allowed, false if it should be throttled.
func (rl *RateLimiter) Allow(source string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.interval)

	w, ok := rl.windows[source]
	if !ok {
		w = &slidingWindow{}
		rl.windows[source] = w
	}

	// Prune old timestamps.
	valid := w.timestamps[:0]
	for _, ts := range w.timestamps {
		if ts.After(cutoff) {
			valid = append(valid, ts)
		}
	}
	w.timestamps = valid

	if len(w.timestamps) >= rl.limit {
		return false
	}

	w.timestamps = append(w.timestamps, now)
	return true
}

// Reset clears the rate limit state for a source.
func (rl *RateLimiter) Reset(source string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.windows, source)
}

// Remaining returns how many requests are left for a source in the current window.
func (rl *RateLimiter) Remaining(source string) int {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.interval)

	w, ok := rl.windows[source]
	if !ok {
		return rl.limit
	}

	count := 0
	for _, ts := range w.timestamps {
		if ts.After(cutoff) {
			count++
		}
	}

	remaining := rl.limit - count
	if remaining < 0 {
		return 0
	}
	return remaining
}

// Cleanup removes stale windows that haven't been used since the interval.
func (rl *RateLimiter) Cleanup() int {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-rl.interval)
	removed := 0
	for source, w := range rl.windows {
		if len(w.timestamps) == 0 {
			delete(rl.windows, source)
			removed++
			continue
		}
		// If the most recent timestamp is older than the interval, remove.
		if w.timestamps[len(w.timestamps)-1].Before(cutoff) {
			delete(rl.windows, source)
			removed++
		}
	}
	return removed
}
