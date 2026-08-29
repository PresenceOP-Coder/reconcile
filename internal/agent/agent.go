package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"github.com/reconcile/internal/model"
	"google.golang.org/api/option"
)

// GenerateResolutionReport takes a list of reconciliation exceptions and
// uses the Gemini API to generate actionable finance operations tasks.
func GenerateResolutionReport(ctx context.Context, apiKey string, exceptions []model.Exception) (string, error) {
	if apiKey == "" {
		return "", fmt.Errorf("GEMINI_API_KEY is not set")
	}

	if len(exceptions) == 0 {
		return "No exceptions found! The books are perfectly balanced.", nil
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return "", fmt.Errorf("failed to create genai client: %w", err)
	}
	defer client.Close()

	gemini := client.GenerativeModel("gemini-3.5-flash")

	gemini.SystemInstruction = &genai.Content{
		Parts: []genai.Part{
			genai.Text(`You are an AI Finance Controller. Your job is to review automated reconciliation exceptions and draft a clear, actionable resolution report for the finance operations team.

For each exception type:
1. Explain briefly what went wrong.
2. Provide a specific, actionable next step (e.g., "Email Stripe support", "Verify manual ledger entry with sales").

Format your response as a professional Markdown report. Group similar issues together. Be concise and authoritative.`),
		},
	}

	var promptBuilder strings.Builder
	promptBuilder.WriteString(fmt.Sprintf("Please review the following %d reconciliation exceptions:\n\n", len(exceptions)))

	for i, exc := range exceptions {
		if i >= 50 {
			promptBuilder.WriteString("\n... and more exceptions omitted for brevity.\n")
			break
		}
		promptBuilder.WriteString(fmt.Sprintf("- Record %s (Source: %s) | Reason: %s | Detail: %s\n",
			exc.Record.ID, exc.Record.Source, exc.ReasonCode, exc.Detail))
	}

	resp, err := gemini.GenerateContent(ctx, genai.Text(promptBuilder.String()))
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no response generated from AI")
	}

	var reportBuilder strings.Builder
	for _, part := range resp.Candidates[0].Content.Parts {
		if text, ok := part.(genai.Text); ok {
			reportBuilder.WriteString(string(text))
		}
	}

	return reportBuilder.String(), nil
}

// ExplainRecord reads an audit log file, finds the entry for the given recordID,
// and asks Gemini to explain in plain English what happened and why.
// Strictly grounded in real audit data — the model only sees what's in the log.
func ExplainRecord(ctx context.Context, apiKey, auditPath, recordID string) (string, error) {
	if apiKey == "" {
		return "", fmt.Errorf("GEMINI_API_KEY is not set")
	}

	entry, err := findAuditEntry(auditPath, recordID)
	if err != nil {
		return "", err
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return "", fmt.Errorf("failed to create genai client: %w", err)
	}
	defer client.Close()

	gemini := client.GenerativeModel("gemini-3.5-flash")

	gemini.SystemInstruction = &genai.Content{
		Parts: []genai.Part{
			genai.Text(`You are an AI Finance Controller analyzing a reconciliation audit log.
Your job is to explain, in plain English, exactly what happened to a specific record.

Rules:
- Only use information present in the audit entry provided. Never invent data.
- Explain the outcome (matched / exception) in simple terms a non-technical finance analyst can understand.
- If it was an exception, say what the finance team should do next.
- If it was a match, confirm what sources it was matched with and with what confidence.
- Be concise: 3-5 sentences maximum.`),
		},
	}

	prompt := fmt.Sprintf(
		"Here is the raw audit log entry for record %q:\n\n```json\n%s\n```\n\nPlease explain what happened to this record.",
		recordID, entry,
	)

	resp, err := gemini.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("failed to generate explanation: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no explanation generated")
	}

	var sb strings.Builder
	for _, part := range resp.Candidates[0].Content.Parts {
		if text, ok := part.(genai.Text); ok {
			sb.WriteString(string(text))
		}
	}
	return sb.String(), nil
}

// findAuditEntry scans the JSONL audit log and returns the pretty-printed JSON
// for the record with the given ID.
func findAuditEntry(auditPath, recordID string) (string, error) {
	f, err := os.Open(auditPath)
	if err != nil {
		return "", fmt.Errorf("cannot open audit log %s: %w", auditPath, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if id, ok := entry["record_id"].(string); ok && id == recordID {
			pretty, _ := json.MarshalIndent(entry, "", "  ")
			return string(pretty), nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading audit log: %w", err)
	}

	return "", fmt.Errorf("record %q not found in %s — run 'reconcile run --audit %s' first", recordID, auditPath, auditPath)
}
