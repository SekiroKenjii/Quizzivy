package anthropic

import sdk "github.com/anthropics/anthropic-sdk-go"

// Usage is what the provider reported for one completion. InputTokens counts
// only the tokens that were neither read from nor written to the prompt cache,
// as the provider does, so the four counts never overlap.
type Usage struct {
	InputTokens      int64
	OutputTokens     int64
	CacheReadTokens  int64
	CacheWriteTokens int64
}

func usageOf(u sdk.Usage) Usage {
	return Usage{
		InputTokens:      u.InputTokens,
		OutputTokens:     u.OutputTokens,
		CacheReadTokens:  u.CacheReadInputTokens,
		CacheWriteTokens: u.CacheCreationInputTokens,
	}
}
