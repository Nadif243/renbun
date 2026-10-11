package oracle

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

var client *genai.Client

// InitOracle ignites the connection to the Gemini API.
func InitOracle(apiKey string) error {
	var err error
	ctx := context.Background()
	client, err = genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return fmt.Errorf("failed to ignite the Oracle: %w", err)
	}
	return nil
}

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

// Present requests a lightweight kana + English translation.
func Present(ctx context.Context, japaneseText string) (*PresentResult, error) {
	model := client.GenerativeModel("gemini-3.5-flash-lite")

	// Force the Gemini API to exclusively return JSON
	model.ResponseMIMEType = "application/json"

	// Create a float32 variable, then pass its memory address pointer
	temp := float32(0.1) // Keep it highly analytical and deterministic
	model.Temperature = &temp

	prompt := fmt.Sprintf(`Analyze this Japanese sentence: "%s"
Output strictly in JSON format with exactly these two keys: "reading" (the full sentence in hiragana/katakana) and "translation" (a natural English translation). No other text.`, japaneseText)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, err
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("the Oracle remained silent")
	}

	rawText := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])
	rawText = cleanJSON(rawText)

	var result PresentResult
	if err := json.Unmarshal([]byte(rawText), &result); err != nil {
		return nil, fmt.Errorf("failed to decode the Oracle's prophecy: %w\nRaw: %s", err, rawText)
	}

	return &result, nil
}

// Dissect requests a deep, structured morphological breakdown.
func Dissect(ctx context.Context, japaneseText string) (*DissectResult, error) {
	model := client.GenerativeModel("gemini-3.6-flash")

	// Force the Gemini API to exclusively return JSON
	model.ResponseMIMEType = "application/json"

	temp := float32(0.2)
	model.Temperature = &temp

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

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, err
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("the Oracle remained silent")
	}

	rawText := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])
	rawText = cleanJSON(rawText)

	var result DissectResult
	if err := json.Unmarshal([]byte(rawText), &result); err != nil {
		return nil, fmt.Errorf("failed to decode the Oracle's prophecy: %w\nRaw: %s", err, rawText)
	}

	return &result, nil
}
