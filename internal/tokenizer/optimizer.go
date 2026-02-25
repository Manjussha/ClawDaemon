package tokenizer

import "strings"

// BudgetZone represents the current token usage level.
type BudgetZone int

const (
	ZoneGreen  BudgetZone = iota // 0–60%: full context
	ZoneYellow                   // 60–80%: compress IDENTITY+THINKING, trim memory
	ZoneOrange                   // 80–90%: skip optional context, essentials only
	ZoneRed                      // 90–100%: minimum context + Telegram alert
)

// String returns a human-readable label for the zone.
func (z BudgetZone) String() string {
	switch z {
	case ZoneYellow:
		return "YELLOW"
	case ZoneOrange:
		return "ORANGE"
	case ZoneRed:
		return "RED"
	default:
		return "GREEN"
	}
}

// OptimizeContext compresses a context string based on the current budget zone.
// GREEN: returns as-is. YELLOW+: truncates to reduce token usage.
func OptimizeContext(ctx string, zone BudgetZone) string {
	switch zone {
	case ZoneGreen:
		return ctx
	case ZoneYellow:
		return CompressSummary(ctx, 2000)
	case ZoneOrange:
		return CompressSummary(ctx, 800)
	case ZoneRed:
		return CompressSummary(ctx, 300)
	default:
		return ctx
	}
}

// CompressSummary truncates text to approximately maxTokens tokens.
// Preserves the beginning of the text (most important context).
func CompressSummary(text string, maxTokens int) string {
	maxChars := maxTokens * 4
	if len(text) <= maxChars {
		return text
	}
	truncated := text[:maxChars]
	// Trim to last sentence boundary to avoid cutting mid-sentence.
	if idx := strings.LastIndexAny(truncated, ".!?\n"); idx > maxChars/2 {
		truncated = truncated[:idx+1]
	}
	return truncated + "\n\n[... context compressed for token budget ...]"
}
