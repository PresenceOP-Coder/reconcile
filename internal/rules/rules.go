package rules

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type SourceConfig struct {
	Name string `yaml:"name"`
	File string `yaml:"file"`
}

type ExactConfig struct {
	DateBucketDays int `yaml:"date_bucket_days"`
}

type FuzzyConfig struct {
	AmountTolerancePct float64  `yaml:"amount_tolerance_pct"`
	DateWindowDays     int      `yaml:"date_window_days"`
	MinConfidence      float64  `yaml:"min_confidence"`
	FXRates            FXRates  `yaml:"-"` // populated at runtime, not from YAML
}

type MatchingConfig struct {
	Exact ExactConfig `yaml:"exact"`
	Fuzzy FuzzyConfig `yaml:"fuzzy"`
}

type Config struct {
	Matching   MatchingConfig `yaml:"matching"`
	Sources    []SourceConfig `yaml:"sources"`
	FXRatesFile string        `yaml:"fx_rates_file"`
}

// Load loads and validates the reconciliation rules from a YAML file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read rules file %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse YAML rules: %w", err)
	}

	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("invalid rules configuration: %w", err)
	}

	fx, err := LoadFXRates(cfg.FXRatesFile)
	if err != nil {
		return nil, fmt.Errorf("fx_rates: %w", err)
	}
	cfg.Matching.Fuzzy.FXRates = fx

	return &cfg, nil
}

func validate(cfg *Config) error {
	if len(cfg.Sources) == 0 {
		return fmt.Errorf("at least one source must be defined")
	}

	sourceNames := make(map[string]bool)
	for idx, src := range cfg.Sources {
		if src.Name == "" {
			return fmt.Errorf("source at index %d has empty name", idx)
		}
		if src.File == "" {
			return fmt.Errorf("source '%s' has empty file path", src.Name)
		}
		if sourceNames[src.Name] {
			return fmt.Errorf("duplicate source name '%s' in rules", src.Name)
		}
		sourceNames[src.Name] = true
	}

	if cfg.Matching.Exact.DateBucketDays < 0 {
		return fmt.Errorf("exact.date_bucket_days must be non-negative, got %d", cfg.Matching.Exact.DateBucketDays)
	}

	if cfg.Matching.Fuzzy.AmountTolerancePct < 0 {
		return fmt.Errorf("fuzzy.amount_tolerance_pct must be non-negative, got %f", cfg.Matching.Fuzzy.AmountTolerancePct)
	}

	if cfg.Matching.Fuzzy.DateWindowDays < 0 {
		return fmt.Errorf("fuzzy.date_window_days must be non-negative, got %d", cfg.Matching.Fuzzy.DateWindowDays)
	}

	if cfg.Matching.Fuzzy.MinConfidence < 0 || cfg.Matching.Fuzzy.MinConfidence > 1.0 {
		return fmt.Errorf("fuzzy.min_confidence must be between 0.0 and 1.0, got %f", cfg.Matching.Fuzzy.MinConfidence)
	}

	return nil
}
