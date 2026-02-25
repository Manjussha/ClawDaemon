// Package limiter detects rate limit signals in CLI output.
package limiter

import "strings"

// Common rate limit patterns per CLI type.
var patterns = map[string][]string{
	"claude": {
		"rate limit",
		"rate_limit",
		"too many requests",
		"429",
		"overloaded",
	},
	"gemini": {
		"quota exceeded",
		"rate limit",
		"429",
		"resource exhausted",
	},
	"browser": {
		"timeout",
		"connection refused",
	},
}

// Detector checks output lines for rate limit signals.
type Detector struct {
	cliType  string
	keywords []string
}

// New creates a Detector for the given CLI type.
func New(cliType string) *Detector {
	kws := patterns[cliType]
	if kws == nil {
		kws = patterns["claude"]
	}
	return &Detector{cliType: cliType, keywords: kws}
}

// DetectLimit returns true if the line contains a rate limit signal.
func (d *Detector) DetectLimit(line string) bool {
	lower := strings.ToLower(line)
	for _, kw := range d.keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// ErrRateLimit is returned by a worker when a rate limit is detected.
type ErrRateLimit struct {
	Line string
}

func (e *ErrRateLimit) Error() string {
	return "rate limit detected: " + e.Line
}
