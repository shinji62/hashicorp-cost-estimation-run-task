package config

import (
	"flag"
	"os"
	"testing"
)

func TestConfigPriority(t *testing.T) {
	// Save original args and restore after test
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Reset flag.CommandLine for testing
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	// Set environment variables
	os.Setenv("LOG_LEVEL", "DEBUG")
	os.Setenv("SERVER_ADDR", "8080")
	defer os.Unsetenv("LOG_LEVEL")
	defer os.Unsetenv("SERVER_ADDR")

	// Simulate CLI args (should override env vars)
	os.Args = []string{"cmd", "--log-level", "ERROR", "--addr", "9090"}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// CLI args should take precedence
	if cfg.LogLevel != "ERROR" {
		t.Errorf("Expected log level ERROR from CLI, got %s", cfg.LogLevel)
	}

	if cfg.ServerAddr != "9090" {
		t.Errorf("Expected server addr 9090 from CLI, got %s", cfg.ServerAddr)
	}
}

func TestConfigDefaults(t *testing.T) {
	// Reset flag.CommandLine for testing
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	// Save original args and restore after test
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"cmd"}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Check defaults
	if cfg.ServerAddr != "22180" {
		t.Errorf("Expected default server addr 22180, got %s", cfg.ServerAddr)
	}

	if cfg.ServerPath != "/runtask" {
		t.Errorf("Expected default server path /runtask, got %s", cfg.ServerPath)
	}

	if cfg.LogLevel != "info" {
		t.Errorf("Expected default log level info, got %s", cfg.LogLevel)
	}

	if cfg.LogFormat != "text" {
		t.Errorf("Expected default log format text, got %s", cfg.LogFormat)
	}

	if cfg.GeminiModel != "gemini-3.1-flash-lite" {
		t.Errorf("Expected default gemini model gemini-3.1-flash-lite, got %s", cfg.GeminiModel)
	}
}

func TestConfigEnvVars(t *testing.T) {
	// Reset flag.CommandLine for testing
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	// Save original args and restore after test
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"cmd"}

	// Set environment variables
	os.Setenv("LOG_LEVEL", "WARN")
	os.Setenv("LOG_FORMAT", "json")
	os.Setenv("C3X_BUDGET_LIMIT", "1000.50")
	os.Setenv("AGENT_SECURITY_ENABLED", "true")
	defer os.Unsetenv("LOG_LEVEL")
	defer os.Unsetenv("LOG_FORMAT")
	defer os.Unsetenv("C3X_BUDGET_LIMIT")
	defer os.Unsetenv("AGENT_SECURITY_ENABLED")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.LogLevel != "WARN" {
		t.Errorf("Expected log level WARN from env, got %s", cfg.LogLevel)
	}

	if cfg.LogFormat != "json" {
		t.Errorf("Expected log format json from env, got %s", cfg.LogFormat)
	}

	if cfg.C3XBudgetLimit != 1000.50 {
		t.Errorf("Expected budget limit 1000.50 from env, got %f", cfg.C3XBudgetLimit)
	}

	if !cfg.AgentSecurityEnabled {
		t.Errorf("Expected agent security enabled from env")
	}
}

func TestGetAPIKey(t *testing.T) {
	tests := []struct {
		name           string
		googleAPIKey   string
		geminiAPIKey   string
		expectedResult string
	}{
		{
			name:           "Google API key takes precedence",
			googleAPIKey:   "google-key",
			geminiAPIKey:   "gemini-key",
			expectedResult: "google-key",
		},
		{
			name:           "Gemini API key as fallback",
			googleAPIKey:   "",
			geminiAPIKey:   "gemini-key",
			expectedResult: "gemini-key",
		},
		{
			name:           "No API key",
			googleAPIKey:   "",
			geminiAPIKey:   "",
			expectedResult: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				GoogleAPIKey: tt.googleAPIKey,
				GeminiAPIKey: tt.geminiAPIKey,
			}

			result := cfg.GetAPIKey()
			if result != tt.expectedResult {
				t.Errorf("Expected %s, got %s", tt.expectedResult, result)
			}
		})
	}
}

func TestHasAIEnabled(t *testing.T) {
	tests := []struct {
		name           string
		googleAPIKey   string
		geminiAPIKey   string
		expectedResult bool
	}{
		{
			name:           "AI enabled with Google key",
			googleAPIKey:   "google-key",
			geminiAPIKey:   "",
			expectedResult: true,
		},
		{
			name:           "AI enabled with Gemini key",
			googleAPIKey:   "",
			geminiAPIKey:   "gemini-key",
			expectedResult: true,
		},
		{
			name:           "AI disabled without keys",
			googleAPIKey:   "",
			geminiAPIKey:   "",
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				GoogleAPIKey: tt.googleAPIKey,
				GeminiAPIKey: tt.geminiAPIKey,
			}

			result := cfg.HasAIEnabled()
			if result != tt.expectedResult {
				t.Errorf("Expected %v, got %v", tt.expectedResult, result)
			}
		})
	}
}
