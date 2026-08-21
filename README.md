# Multi-Source Financial Reconciliation Engine (`reconcile`)

A high-throughput, fault-resilient CLI reconciliation engine written in Go. Designed for multi-source financial controllers to reconcile messy, independent transaction records across Payment Gateways, Bank Statements, and Internal General Ledgers.

Built for **Razorpay AI Buildathon — Track 04 (AI Finance Controller)**.

---

## 🌟 Core Features

- **Concurrent Multi-Source Ingestion**: Ingests gateway, bank, and ledger streams simultaneously using lightweight Go goroutines and channel fan-in.
- **2-Tier Matching Architecture**:
  1. **Tier 1 (Deterministic Exact Match)**: High-speed join on `(RefID, Amount, DateBucket)` with zero false positives.
  2. **Tier 2 (Tolerance-Derived Fuzzy Match)**: Resolves currency rounding drift, split settlements, and delayed settlement date drift within configurable thresholds.
- **Zero Record Drop Policy & Typed Exceptions**: Every single transaction is accounted for. Unmatched or corrupted records are strictly classified into typed reason codes (`AMOUNT_MISMATCH`, `DATE_DRIFT`, `DUPLICATE_REF`, `NO_COUNTERPART`, `MALFORMED_INPUT`).
- **Traceable JSONL Audit Trail**: Emits an `audit.jsonl` log per record explaining *why* it matched (pass, applied rule, confidence, counterpart IDs) or *why* it failed.
- **Declarative YAML Rules**: Matching thresholds and source mappings are defined entirely in `rules.yaml` — no hardcoded constants.
- **Resilient Fault Isolation**: Catches corrupted records (e.g. malformed dates, invalid numbers) cleanly at ingest, flags them as exceptions, and continues processing without crashing.

---

## 📐 Architecture & Pipeline Flow

```
┌────────────────────────┐   ┌──────────────────────┐   ┌────────────────────────┐
│ Gateway Settlement CSV │   │  Bank Statement CSV  │   │  Internal Ledger CSV   │
└───────────┬────────────┘   └──────────┬───────────┘   └───────────┬────────────┘
            │                           │                           │
            └─────────────────►  Concurrent Ingestion  ◄────────────┘
                                (Goroutines + Fan-in)
                                        │
                                        ▼ (Captures MALFORMED_INPUT)
                               ┌─────────────────┐
                               │ Exact Match Pass│ (RefID, Exact Amount, Date Bucket)
                               └────────┬────────┘
                                        │
                         ┌──────────────┴──────────────┐
                         ▼                             ▼
                  [Exact Matches]             [Unmatched Records]
                  (Confidence 1.0)                     │
                                                       ▼
                                              ┌─────────────────┐
                                              │ Fuzzy Match Pass│ (Amount %, Date Window)
                                              └────────┬────────┘
                                                       │
                                        ┌──────────────┴──────────────┐
                                        ▼                             ▼
                                 [Fuzzy Matches]             [Exception Classifier]
                               (Confidence >= 0.75)          • AMOUNT_MISMATCH
                                                             • DATE_DRIFT
                                                             • DUPLICATE_REF
                                                             • NO_COUNTERPART
                                        │                             │
                                        └──────────────┬──────────────┘
                                                       ▼
                                         ┌───────────────────────────┐
                                         │  Metrics & Audit Trail    │
                                         │  • audit.jsonl per record │
                                         │  • CLI Terminal Summary   │
                                         └───────────────────────────┘
```

---

## ⚙️ Configuration (`rules.yaml`)

```yaml
matching:
  exact:
    date_bucket_days: 0          # Maximum date diff (in days) for exact match
  fuzzy:
    amount_tolerance_pct: 1.5   # Allowed amount variance percentage
    date_window_days: 3          # Maximum days drift for delayed settlements
    min_confidence: 0.75         # Minimum confidence score to accept fuzzy match

sources:
  - name: gateway
    file: testdata/gateway_settlement.csv
  - name: bank
    file: testdata/bank_statement.csv
  - name: ledger
    file: testdata/internal_ledger.csv
```

---

## 🚀 Quick Start & CLI Usage

### 1. Build the Binary
```bash
go build -o reconcile ./cmd/reconcile
```

### 2. Generate Synthetic Fixtures
Produces 3 CSV files covering clean matches, split settlements, rounding drift, date drift, duplicates, orphans, and malformed records:
```bash
./reconcile generate
```

### 3. Run Full Reconciliation Pipeline
Executes concurrent ingestion, exact match, fuzzy match, exception classification, and outputs audit trail:
```bash
./reconcile run --rules rules.yaml --audit audit.jsonl
```

### 4. Run Exact Match Only (Isolated Mode)
```bash
./reconcile run --rules rules.yaml --exact-only
```

---

## 📊 Sample Output & Verification

```text
====================================================================
           MULTI-SOURCE FINANCIAL RECONCILIATION REPORT           
====================================================================
  Total Ingested Records    : 74    
  Successfully Matched      : 59     (79.73%)
    ├── Exact Matches       : 39    
    └── Fuzzy Matches       : 20    
  Uncertainty / FP Risk     : 6      (8.11% of total)
  Total Exceptions          : 15     (20.27%)
--------------------------------------------------------------------
  EXCEPTION BREAKDOWN BY REASON CODE:
    • AMOUNT_MISMATCH        :    3 records  ( 20.0% of exceptions)
    • DATE_DRIFT             :    3 records  ( 20.0% of exceptions)
    • DUPLICATE_REF          :    4 records  ( 26.7% of exceptions)
    • MALFORMED_INPUT        :    1 records  (  6.7% of exceptions)
    • NO_COUNTERPART         :    4 records  ( 26.7% of exceptions)
--------------------------------------------------------------------
  ✓ Integrity Invariant: 59 matched + 15 exceptions = 74 total (VERIFIED)
====================================================================

Execution time: 1.67ms
✓ Audit log successfully written to audit.jsonl (74 records logged)
```

---

## 🧪 Exception Reason Codes

| Reason Code | Definition & Trigger |
| :--- | :--- |
| `AMOUNT_MISMATCH` | Counterpart records found with matching `RefID`, but amount difference exceeds `amount_tolerance_pct`. |
| `DATE_DRIFT` | Counterpart records found with matching `RefID`, but transaction date exceeds `date_window_days`. |
| `DUPLICATE_REF` | Same reference ID appears multiple times within a source, creating ambiguous non-deterministic match candidates. |
| `NO_COUNTERPART` | Genuinely orphaned record with no corresponding entries in one or more expected counterpart sources. |
| `MALFORMED_INPUT` | Upstream corrupt data (e.g. invalid date formats, non-numeric amount strings) caught during streaming ingestion. |

---

## 🔍 Audit Trail (`audit.jsonl`) Sample Entries

**Matched Entry:**
```json
{
  "record_id": "GW-14-A",
  "source": "gateway",
  "outcome": "matched",
  "pass": "fuzzy",
  "rule": "split_settlement_fuzzy(amount_tol=1.50%, date_window=3d, actual_diff=0.00%, date_diff=0.0d)",
  "matched_with": ["bank:BK-14", "ledger:LD-14", "gateway:GW-14-B"],
  "confidence": 1.0
}
```

**Exception Entry:**
```json
{
  "record_id": "GW-MALF",
  "source": "gateway",
  "outcome": "exception",
  "reason_code": "MALFORMED_INPUT",
  "detail": "Row 26 (ID GW-MALF): invalid date format 'INVALID_DATE_FORMAT_2023-99-99'"
}
```

---

## 🧪 Running Unit Tests

```bash
go test -v ./...
```
