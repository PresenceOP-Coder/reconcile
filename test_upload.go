package main

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	url := "http://localhost:8080/api/reconcile"
	method := "POST"

	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)

	_ = writer.WriteField("amount_tolerance_pct", "1.5")
	_ = writer.WriteField("date_window_days", "3")
	_ = writer.WriteField("min_confidence", "0.75")

	files := []string{"gateway", "bank", "ledger"}
	paths := []string{
		"testdata/gateway_settlement.csv",
		"testdata/bank_statement.csv",
		"testdata/internal_ledger.csv",
	}

	for i, field := range files {
		file, err := os.Open(paths[i])
		if err != nil {
			panic(err)
		}
		part, err := writer.CreateFormFile(field, filepath.Base(paths[i]))
		if err != nil {
			panic(err)
		}
		io.Copy(part, file)
		file.Close()
	}

	err := writer.Close()
	if err != nil {
		panic(err)
	}

	req, err := http.NewRequest(method, url, payload)
	if err != nil {
		panic(err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		panic(err)
	}
	fmt.Println(res.StatusCode)
	fmt.Println(string(body[:100]))
}
