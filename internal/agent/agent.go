package agent

import (
	"context"
	"fmt"
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

	// Use gemini-1.5-flash as it is fast and excellent for structured reasoning
	gemini := client.GenerativeModel("gemini-1.5-flash")
	
	// Configure the model behavior
	gemini.SystemInstruction = &genai.Content{
		Parts: []genai.Part{
			genai.Text(`You are an AI Finance Controller. Your job is to review automated reconciliation exceptions and draft a clear, actionable resolution report for the finance operations team.

For each exception type:
1. Explain briefly what went wrong.
2. Provide a specific, actionable next step (e.g., "Email Stripe support", "Verify manual ledger entry with sales").

Format your response as a professional Markdown report. Group similar issues together. Be concise and authoritative.`),
		},
	}

	// Prepare the prompt payload
	var promptBuilder strings.Builder
	promptBuilder.WriteString(fmt.Sprintf("Please review the following %d reconciliation exceptions:\n\n", len(exceptions)))

	for i, exc := range exceptions {
		// Limit to 50 exceptions to avoid overwhelming the context if it's a huge batch,
		// though 1.5-flash can handle millions of tokens.
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

	// Extract the text part
	var reportBuilder strings.Builder
	for _, part := range resp.Candidates[0].Content.Parts {
		if text, ok := part.(genai.Text); ok {
			reportBuilder.WriteString(string(text))
		}
	}

	return reportBuilder.String(), nil
}
