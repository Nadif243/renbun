package oracle

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/genai"
)

// ---------------------------------------------------------------------------
// Settings to tweak
// ---------------------------------------------------------------------------

const (
	dissectModel = "gemini-3.5-flash-lite"
	presentModel = "gemini-3.5-flash-lite"

	// Thinking level for Gemini 3.x models: "MINIMAL", "LOW", "MEDIUM" or "HIGH".
	// The default for 3.6 Flash is MEDIUM, which is slow for a one-sentence job.
	// Leave empty ("") to not send the setting at all (Lite already defaults to MINIMAL).
	dissectThinking = genai.ThinkingLevel("HIGH")
	presentThinking = genai.ThinkingLevel("")

	// Generous on purpose: a timeout does NOT refund quota, so it must only
	// fire when something is truly stuck.
	dissectTimeout = 60 * time.Second
	presentTimeout = 30 * time.Second

	// Minimum gap between two calls to the same model (free tier: 5 RPM and 15 RPM).
	dissectMinGap = 13 * time.Second
	presentMinGap = 5 * time.Second
)

// ---------------------------------------------------------------------------
// Client setup
// ---------------------------------------------------------------------------

var (
	client       *genai.Client
	apiKeySecret string       // only kept so we can scrub it out of error messages
	httpCalls    atomic.Int64 // counts real HTTP requests that leave this process
)

// countingTransport logs every outgoing request (method + path only, never the key).
// If one !dissect shows up here more than once, something is resending requests.
type countingTransport struct{ base http.RoundTripper }

func (t countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	n := httpCalls.Add(1)
	log.Printf("[oracle] HTTP request #%d -> %s %s", n, req.Method, req.URL.Path)
	return t.base.RoundTrip(req)
}

// InitOracle ignites the connection to the Gemini API.
func InitOracle(apiKey string) error {
	apiKeySecret = apiKey
	ctx := context.Background()

	var err error
	client, err = genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:     apiKey,
		Backend:    genai.BackendGeminiAPI,
		HTTPClient: &http.Client{Transport: countingTransport{base: http.DefaultTransport}},
	})
	if err != nil {
		return fmt.Errorf("failed to ignite the Oracle: %s", redact(err))
	}
	return nil
}

// redact removes the API key from any error text so it never reaches logs or Discord.
func redact(err error) string {
	msg := err.Error()
	if apiKeySecret != "" {
		msg = strings.ReplaceAll(msg, apiKeySecret, "[redacted]")
	}
	return msg
}

// ---------------------------------------------------------------------------
// Gate: one call at a time per model, spaced out, and a breaker for 429s
// ---------------------------------------------------------------------------

type gate struct {
	mu           sync.Mutex
	minGap       time.Duration
	last         time.Time
	blockedUntil time.Time
}

var (
	dissectGate = &gate{minGap: dissectMinGap}
	presentGate = &gate{minGap: presentMinGap}
)

type restingError struct{ remaining time.Duration }

func (e *restingError) Error() string {
	if e.remaining > 10*time.Minute {
		return fmt.Sprintf("the Oracle has used up its daily allowance (back in about %s)",
			e.remaining.Round(time.Minute))
	}
	return fmt.Sprintf("the Oracle is catching its breath, try again in %s",
		e.remaining.Round(time.Second))
}

// wait blocks until the call is allowed, or fails fast (without touching the API)
// if the model is currently rate-limited.
func (g *gate) wait(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if remaining := time.Until(g.blockedUntil); remaining > 0 {
		return &restingError{remaining: remaining}
	}
	if gap := g.minGap - time.Since(g.last); gap > 0 {
		select {
		case <-time.After(gap):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	g.last = time.Now()
	return nil
}

var retryRe = regexp.MustCompile(`retry in ([0-9.hms]+)`)

// noteFailure arms the breaker after a 429 so we stop spending quota on doomed calls.
func (g *gate) noteFailure(err error) {
	msg := err.Error()
	if !strings.Contains(msg, "429") && !strings.Contains(msg, "RESOURCE_EXHAUSTED") {
		return
	}
	d := 60 * time.Second // fallback if the message has no retry hint
	if m := retryRe.FindStringSubmatch(msg); m != nil {
		if parsed, perr := time.ParseDuration(strings.TrimRight(m[1], ".")); perr == nil {
			d = parsed
		}
	}
	g.mu.Lock()
	g.blockedUntil = time.Now().Add(d)
	g.mu.Unlock()
	log.Printf("[oracle] rate limited, breaker armed for %s", d.Round(time.Second))
}

// ---------------------------------------------------------------------------
// One shared call path used by Present and Dissect
// ---------------------------------------------------------------------------

func generate(ctx context.Context, g *gate, model string, timeout time.Duration,
	thinking genai.ThinkingLevel, prompt string) (string, error) {

	if err := g.wait(ctx); err != nil {
		return "", err
	}

	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cfg := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		MaxOutputTokens:  4096,
	}
	// thinking_level only exists on Gemini 3.x; other models (e.g. Gemma) would reject it.
	if thinking != "" && strings.HasPrefix(model, "gemini-3") {
		cfg.ThinkingConfig = &genai.ThinkingConfig{ThinkingLevel: thinking}
	}

	start := time.Now()
	resp, err := client.Models.GenerateContent(callCtx, model, genai.Text(prompt), cfg)
	elapsed := time.Since(start).Round(100 * time.Millisecond)

	if err != nil {
		log.Printf("[oracle] %s failed after %s: %s", model, elapsed, redact(err))
		if callCtx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("the Oracle timed out after %s", timeout)
		}
		g.noteFailure(err)
		return "", fmt.Errorf("%s", redact(err))
	}

	log.Printf("[oracle] %s ok in %s", model, elapsed)
	if u := resp.UsageMetadata; u != nil {
		log.Printf("[oracle] tokens: prompt=%d output=%d thinking=%d",
			u.PromptTokenCount, u.CandidatesTokenCount, u.ThoughtsTokenCount)
	}

	text := resp.Text()
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("the Oracle remained silent")
	}
	return text, nil
}

// ---------------------------------------------------------------------------
// Result types
// ---------------------------------------------------------------------------

// PresentResult holds the lightweight translation data.
type PresentResult struct {
	Reading     string `json:"reading"`
	Translation string `json:"translation"`
}

// DissectResult holds the heavy morphological breakdown.
type DissectResult struct {
	Reading     string      `json:"reading"`
	Translation string      `json:"translation"`
	JLPT        string      `json:"jlpt"`
	Nuance      string      `json:"nuance"`
	Grammar     []string    `json:"grammar"`
	Vocab       []VocabItem `json:"vocab"`
}

type VocabItem struct {
	Word    string `json:"word"`
	Reading string `json:"reading"`
	Meaning string `json:"meaning"`
}

// cleanJSON is a utility to strip markdown backticks often returned by LLMs.
func cleanJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	return strings.TrimSpace(raw)
}

// ---------------------------------------------------------------------------
// Present / Dissect
// ---------------------------------------------------------------------------

// Present requests a lightweight kana + English translation.
func Present(ctx context.Context, japaneseText string) (*PresentResult, error) {
	prompt := fmt.Sprintf(`Analyze this Japanese sentence: "%s"
Output strictly in JSON format with exactly these two keys: "reading" (the full sentence in hiragana/katakana) and "translation" (a natural English translation). No other text.`, japaneseText)

	raw, err := generate(ctx, presentGate, presentModel, presentTimeout, presentThinking, prompt)
	if err != nil {
		return nil, err
	}
	raw = cleanJSON(raw)

	var result PresentResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("failed to decode the Oracle's prophecy: %w\nRaw: %s", err, raw)
	}
	return &result, nil
}

// Dissect requests a deep, structured morphological breakdown.
func Dissect(ctx context.Context, japaneseText string) (*DissectResult, error) {
	prompt := fmt.Sprintf(`Act as an expert Japanese linguist. Dissect this sentence: "%s"
Output strictly in JSON format matching this structure:
{
  "reading": "full sentence reading in kana",
  "translation": "natural english translation",
  "jlpt": "estimated JLPT level (e.g., N3)",
  "nuance": "1-2 sentences explaining any cultural context, slang, or implied meaning",
  "grammar": ["list of key grammar points used"],
  "vocab": [
    {"word": "kanji or word", "reading": "kana", "meaning": "english"}
  ]
}
Do not include markdown or outside text.`, japaneseText)

	raw, err := generate(ctx, dissectGate, dissectModel, dissectTimeout, dissectThinking, prompt)
	if err != nil {
		return nil, err
	}
	raw = cleanJSON(raw)

	var result DissectResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("failed to decode the Oracle's prophecy: %w\nRaw: %s", err, raw)
	}
	return &result, nil
}
