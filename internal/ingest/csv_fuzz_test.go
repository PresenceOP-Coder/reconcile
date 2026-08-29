package ingest_test

import (
	"os"
	"testing"

	"github.com/reconcile/internal/ingest"
)

// FuzzParseCSV feeds arbitrary byte sequences into the CSV parser and asserts
// it never panics. All errors must be handled gracefully — the function must
// return cleanly on any input, even garbage.
func FuzzParseCSV(f *testing.F) {
	f.Add([]byte("ID,RefID,Amount,Currency,Date,Description\nGW-1,REF-001,100.00,INR,2023-10-01T10:00:00Z,Test\n"))
	f.Add([]byte(""))
	f.Add([]byte("ID,RefID,Amount,Currency,Date,Description\n"))
	f.Add([]byte("not,a,valid,csv\nbad_amount,REF-X,NOTANUMBER,INR,BADDATE,desc\n"))
	f.Add([]byte("\xff\xfe invalid utf8"))
	f.Add([]byte("ID,RefID,Amount,Currency,Date,Description\nGW-1,REF-001,-500.00,USD,2023-10-01T10:00:00Z,Negative\n"))

	f.Fuzz(func(t *testing.T, data []byte) {
		dir := t.TempDir()
		path := dir + "/fuzz_input.csv"
		if err := os.WriteFile(path, data, 0644); err != nil {
			return
		}

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("ParseCSV panicked on fuzz input: %v", r)
			}
		}()

		ingest.ParseCSV("fuzz_source", path)
	})
}
