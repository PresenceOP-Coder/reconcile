package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/reconcile/internal/agent"
	"github.com/reconcile/internal/audit"
	"github.com/reconcile/internal/metrics"
	"github.com/reconcile/internal/pattern"
	"github.com/reconcile/internal/pipeline"
	"github.com/reconcile/internal/rules"
	"github.com/reconcile/web"
)

// Server holds the last pipeline run in memory so the AI handlers can
// reference it without re-running the pipeline.
type Server struct {
	mu            sync.RWMutex
	lastResult    *pipeline.PipelineResult
	lastSummary   *metrics.Summary
	lastAlerts    []pattern.SystemicAlert
	lastAuditPath string
	apiKey        string
}

// ─── API payload types ───────────────────────────────────────────────────────

type reconcileResponse struct {
	Success            bool                    `json:"success"`
	Error              string                  `json:"error,omitempty"`
	TotalRecords       int                     `json:"total_records"`
	MatchRatePct       float64                 `json:"match_rate_pct"`
	ExceptionRatePct   float64                 `json:"exception_rate_pct"`
	ExactMatches       int                     `json:"exact_matches"`
	FuzzyMatches       int                     `json:"fuzzy_matches"`
	ExceptionCount     int                     `json:"exception_count"`
	FPRisk             int                     `json:"fp_risk"`
	FPRatePct          float64                 `json:"fp_rate_pct"`
	InvariantValid     bool                    `json:"invariant_valid"`
	ElapsedMs          float64                 `json:"elapsed_ms"`
	ExceptionBreakdown map[string]int          `json:"exception_breakdown"`
	RecordsBySource    map[string]int          `json:"records_by_source"`
	Exceptions         []exceptionRow          `json:"exceptions"`
	Matches            []matchRow              `json:"matches"`
	Alerts             []pattern.SystemicAlert `json:"alerts"`
}

type exceptionRow struct {
	RecordID   string  `json:"record_id"`
	Source     string  `json:"source"`
	RefID      string  `json:"ref_id"`
	Amount     float64 `json:"amount"`
	Currency   string  `json:"currency"`
	ReasonCode string  `json:"reason_code"`
	Detail     string  `json:"detail"`
}

type matchRow struct {
	MatchID     string   `json:"match_id"`
	Pass        string   `json:"pass"`
	Confidence  float64  `json:"confidence"`
	RefID       string   `json:"ref_id"`
	RecordCount int      `json:"record_count"`
	Sources     []string `json:"sources"`
	Rule        string   `json:"rule"`
	Amount      float64  `json:"amount"`
}

type explainRequest struct {
	RecordID string `json:"record_id"`
}

type explainResponse struct {
	Success     bool   `json:"success"`
	Error       string `json:"error,omitempty"`
	RecordID    string `json:"record_id"`
	Explanation string `json:"explanation"`
}

type agentResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Report  string `json:"report"`
}

// ─── Constructor ─────────────────────────────────────────────────────────────

func New(apiKey string) *Server {
	return &Server{apiKey: apiKey}
}

func (s *Server) Start(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/reconcile", s.handleReconcile)
	mux.HandleFunc("/api/explain", s.handleExplain)
	mux.HandleFunc("/api/agent", s.handleAgent)
	mux.HandleFunc("/api/sample", s.handleSample)

	dist, err := web.Dist()
	if err != nil {
		return err
	}
	mux.Handle("/", http.FileServer(http.FS(dist)))

	fmt.Printf("\n  Reconcile UI  →  http://%s\n\n", addr)
	return http.ListenAndServe(addr, mux)
}

// ─── Handlers ────────────────────────────────────────────────────────────────

func (s *Server) handleSample(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}
	cfg, err := rules.Load("rules.yaml")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "rules.yaml not found — run from the project root directory"})
		return
	}
	s.runPipeline(w, cfg)
}

func (s *Server) handleReconcile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "form parse failed: " + err.Error()})
		return
	}

	amtTol := parseFloat(r.FormValue("amount_tolerance_pct"), 1.5)
	dateWin := parseInt(r.FormValue("date_window_days"), 3)
	minConf := parseFloat(r.FormValue("min_confidence"), 0.75)

	tmpDir, err := os.MkdirTemp("", "reconcile-web-*")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "temp dir failed"})
		return
	}

	// Source file name map: form field → filename on disk
	fileMap := map[string]string{
		"gateway": "gateway_settlement.csv",
		"bank":    "bank_statement.csv",
		"ledger":  "internal_ledger.csv",
	}

	saved := map[string]string{}
	for field, fname := range fileMap {
		f, _, err := r.FormFile(field)
		if err != nil {
			continue // missing source is caught by pipeline
		}
		defer f.Close()
		dest := filepath.Join(tmpDir, fname)
		out, err := os.Create(dest)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "cannot write " + field})
			return
		}
		defer out.Close()
		if _, err := io.Copy(out, f); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "cannot save " + field})
			return
		}
		saved[field] = filepath.ToSlash(dest)
	}

	if len(saved) < 2 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "at least 2 source CSV files are required"})
		return
	}

	rulesYAML := fmt.Sprintf(`matching:
  exact:
    date_bucket_days: 0
  fuzzy:
    amount_tolerance_pct: %.2f
    date_window_days: %d
    min_confidence: %.2f
sources:`, amtTol, dateWin, minConf)

	order := []string{"gateway", "bank", "ledger"}
	for _, name := range order {
		if p, ok := saved[name]; ok {
			rulesYAML += fmt.Sprintf("\n  - name: %s\n    file: %s", name, p)
		}
	}

	rulesPath := filepath.Join(tmpDir, "rules.yaml")
	if err := os.WriteFile(rulesPath, []byte(rulesYAML), 0644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "cannot write rules"})
		return
	}

	cfg, err := rules.Load(rulesPath)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "rules error: " + err.Error()})
		return
	}

	cfg.Sources[0].File = saved["gateway"] // already set via YAML, but be explicit
	s.runPipelineWithAuditDir(w, cfg, tmpDir)
}

func (s *Server) runPipeline(w http.ResponseWriter, cfg *rules.Config) {
	s.runPipelineWithAuditDir(w, cfg, ".")
}

func (s *Server) runPipelineWithAuditDir(w http.ResponseWriter, cfg *rules.Config, auditDir string) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	start := time.Now()
	result, err := pipeline.Run(ctx, cfg, pipeline.Options{})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	elapsed := time.Since(start)

	auditPath := filepath.Join(auditDir, "audit.jsonl")
	audit.WriteAuditLog(auditPath, result)

	summary := metrics.Compute(result)
	alerts := pattern.Detect(result.Exceptions, 3)

	s.mu.Lock()
	s.lastResult = result
	s.lastSummary = &summary
	s.lastAlerts = alerts
	s.lastAuditPath = auditPath
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, buildResponse(result, summary, alerts, elapsed))
}

func (s *Server) handleExplain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}

	var req explainRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RecordID == "" {
		writeJSON(w, http.StatusBadRequest, explainResponse{Error: "record_id required"})
		return
	}

	if s.apiKey == "" {
		writeJSON(w, http.StatusOK, explainResponse{
			Success: false,
			Error:   "GEMINI_API_KEY not set — start server with the env var to enable AI features",
		})
		return
	}

	s.mu.RLock()
	auditPath := s.lastAuditPath
	s.mu.RUnlock()

	if auditPath == "" {
		writeJSON(w, http.StatusOK, explainResponse{Error: "no run yet — upload files and run first"})
		return
	}

	explanation, err := agent.ExplainRecord(context.Background(), s.apiKey, auditPath, req.RecordID)
	if err != nil {
		writeJSON(w, http.StatusOK, explainResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, explainResponse{Success: true, RecordID: req.RecordID, Explanation: explanation})
}

func (s *Server) handleAgent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}

	if s.apiKey == "" {
		writeJSON(w, http.StatusOK, agentResponse{
			Success: false,
			Error:   "GEMINI_API_KEY not set — start server with the env var to enable AI features",
		})
		return
	}

	s.mu.RLock()
	result := s.lastResult
	s.mu.RUnlock()

	if result == nil {
		writeJSON(w, http.StatusOK, agentResponse{Error: "no run yet"})
		return
	}

	report, err := agent.GenerateResolutionReport(context.Background(), s.apiKey, result.Exceptions)
	if err != nil {
		writeJSON(w, http.StatusOK, agentResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, agentResponse{Success: true, Report: report})
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func buildResponse(result *pipeline.PipelineResult, summary metrics.Summary, alerts []pattern.SystemicAlert, elapsed time.Duration) reconcileResponse {
	resp := reconcileResponse{
		Success:            true,
		TotalRecords:       summary.TotalRecords,
		MatchRatePct:       r2(summary.MatchRatePct),
		ExceptionRatePct:   r2(summary.ExceptionRatePct),
		ExactMatches:       summary.ExactMatchedRecords,
		FuzzyMatches:       summary.FuzzyMatchedRecords,
		ExceptionCount:     summary.ExceptionRecords,
		FPRisk:             summary.LowConfidenceMatches,
		FPRatePct:          r2(summary.LowConfidenceRatePct),
		InvariantValid:     summary.CountInvariantValid,
		ElapsedMs:          r2(float64(elapsed.Microseconds()) / 1000.0),
		ExceptionBreakdown: summary.ExceptionBreakdown,
		RecordsBySource:    summary.RecordsBySource,
		Alerts:             alerts,
	}

	for _, exc := range result.Exceptions {
		resp.Exceptions = append(resp.Exceptions, exceptionRow{
			RecordID:   exc.Record.ID,
			Source:     exc.Record.Source,
			RefID:      exc.Record.RefID,
			Amount:     exc.Record.Amount,
			Currency:   exc.Record.Currency,
			ReasonCode: exc.ReasonCode,
			Detail:     exc.Detail,
		})
	}

	seen := map[string]bool{}
	for _, m := range result.Matches {
		if seen[m.MatchID] {
			continue
		}
		seen[m.MatchID] = true

		srcSet := map[string]bool{}
		var uniqSrcs []string
		var refID string
		var amt float64
		for _, rec := range m.Records {
			refID = rec.RefID
			amt = rec.Amount
			if !srcSet[rec.Source] {
				srcSet[rec.Source] = true
				uniqSrcs = append(uniqSrcs, rec.Source)
			}
		}
		sort.Strings(uniqSrcs)

		resp.Matches = append(resp.Matches, matchRow{
			MatchID:     m.MatchID,
			Pass:        m.Pass,
			Confidence:  m.Confidence,
			RefID:       refID,
			RecordCount: len(m.Records),
			Sources:     uniqSrcs,
			Rule:        m.RuleApplied,
			Amount:      amt,
		})
	}
	return resp
}

func parseFloat(s string, def float64) float64 {
	if s == "" {
		return def
	}
	var v float64
	fmt.Sscanf(s, "%f", &v)
	if v == 0 {
		return def
	}
	return v
}

func parseInt(s string, def int) int {
	if s == "" {
		return def
	}
	var v int
	fmt.Sscanf(s, "%d", &v)
	if v == 0 {
		return def
	}
	return v
}

func r2(v float64) float64 { return math.Round(v*100) / 100 }
