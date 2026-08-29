package rules

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// FXRates maps currency code (e.g. "USD", "EUR") to its conversion rate
// relative to the base currency (INR). INR itself should be 1.0.
type FXRates map[string]float64

// DefaultFXRates is a built-in mock table used when no fx_rates.yaml is provided.
// All rates are approximate and relative to INR.
var DefaultFXRates = FXRates{
	"INR": 1.0,
	"USD": 83.50,
	"EUR": 91.20,
	"GBP": 106.40,
	"SGD": 62.10,
	"AED": 22.73,
	"JPY": 0.56,
}

// LoadFXRates reads an FX rate YAML file. Falls back to DefaultFXRates if the
// path is empty or the file does not exist.
func LoadFXRates(path string) (FXRates, error) {
	if path == "" {
		return DefaultFXRates, nil
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return DefaultFXRates, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cannot read fx_rates file %s: %w", path, err)
	}

	var rates FXRates
	if err := yaml.Unmarshal(data, &rates); err != nil {
		return nil, fmt.Errorf("invalid fx_rates file %s: %w", path, err)
	}

	// Always ensure INR baseline is present
	if _, ok := rates["INR"]; !ok {
		rates["INR"] = 1.0
	}

	return rates, nil
}

// ToBase converts an amount in the given currency to the base currency (INR).
func (fx FXRates) ToBase(amount float64, currency string) float64 {
	if currency == "" {
		return amount
	}
	rate, ok := fx[currency]
	if !ok || rate == 0 {
		return amount
	}
	return amount * rate
}
