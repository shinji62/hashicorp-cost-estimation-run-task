package helpers

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestInitialize(t *testing.T) {
	tests := []struct {
		name          string
		level         string
		format        string
		expectedLevel logrus.Level
	}{
		{"Debug level", "debug", "text", logrus.DebugLevel},
		{"Info level", "info", "text", logrus.InfoLevel},
		{"Warn level", "warn", "text", logrus.WarnLevel},
		{"Warning level", "warning", "text", logrus.WarnLevel},
		{"Error level", "error", "text", logrus.ErrorLevel},
		{"Invalid level defaults to info", "invalid", "text", logrus.InfoLevel},
		{"JSON format", "info", "json", logrus.InfoLevel},
		{"Default format", "info", "unknown", logrus.InfoLevel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Initialize(tt.level, tt.format)
			if err != nil {
				t.Errorf("Initialize() error = %v", err)
			}

			if Logger == nil {
				t.Fatal("Logger should not be nil after initialization")
			}

			if Logger.GetLevel() != tt.expectedLevel {
				t.Errorf("Expected log level %v, got %v", tt.expectedLevel, Logger.GetLevel())
			}

			// Check formatter type
			if tt.format == "json" {
				if _, ok := Logger.Formatter.(*logrus.JSONFormatter); !ok {
					t.Error("Expected JSONFormatter")
				}
			} else {
				if _, ok := Logger.Formatter.(*logrus.TextFormatter); !ok {
					t.Error("Expected TextFormatter")
				}
			}
		})
	}
}

func TestGetLogger(t *testing.T) {
	// Reset logger
	Logger = nil

	logger := GetLogger()
	if logger == nil {
		t.Fatal("GetLogger() should not return nil")
	}

	// Should return the same instance
	logger2 := GetLogger()
	if logger != logger2 {
		t.Error("GetLogger() should return the same instance")
	}
}

func TestSetOutput(t *testing.T) {
	var buf bytes.Buffer
	Initialize("info", "text")

	SetOutput(&buf)

	Info("test message")

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("Expected 'test message' in output, got: %s", output)
	}
}

func TestLogLevelFiltering(t *testing.T) {
	tests := []struct {
		name      string
		setLevel  string
		logFunc   func(...interface{})
		message   string
		shouldLog bool
	}{
		{"Debug at Debug level", "debug", Debug, "debug message", true},
		{"Info at Debug level", "debug", Info, "info message", true},
		{"Debug at Info level", "info", Debug, "debug message", false},
		{"Info at Info level", "info", Info, "info message", true},
		{"Warn at Info level", "info", Warn, "warn message", true},
		{"Debug at Error level", "error", Debug, "debug message", false},
		{"Error at Error level", "error", Error, "error message", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			Initialize(tt.setLevel, "text")
			SetOutput(&buf)

			tt.logFunc(tt.message)

			output := buf.String()
			containsMessage := strings.Contains(output, tt.message)

			if tt.shouldLog && !containsMessage {
				t.Errorf("Expected message to be logged but it wasn't: %s", tt.message)
			}
			if !tt.shouldLog && containsMessage {
				t.Errorf("Expected message not to be logged but it was: %s", tt.message)
			}
		})
	}
}

func TestLogPrefixes(t *testing.T) {
	tests := []struct {
		name       string
		logFunc    func(...interface{})
		message    string
		wantPrefix string
	}{
		{"Debug prefix", Debug, "test", "level=debug"},
		{"Info prefix", Info, "test", "level=info"},
		{"Warn prefix", Warn, "test", "level=warning"},
		{"Error prefix", Error, "test", "level=error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			Initialize("debug", "text")
			SetOutput(&buf)

			tt.logFunc(tt.message)

			output := buf.String()
			if !strings.Contains(output, tt.wantPrefix) {
				t.Errorf("Expected prefix %s in output, got: %s", tt.wantPrefix, output)
			}
			if !strings.Contains(output, tt.message) {
				t.Errorf("Expected message %s in output, got: %s", tt.message, output)
			}
		})
	}
}

func TestLogFormatting(t *testing.T) {
	var buf bytes.Buffer
	Initialize("debug", "text")
	SetOutput(&buf)

	Debugf("Test %s with %d values", "formatting", 42)
	output := buf.String()

	if !strings.Contains(output, "Test formatting with 42 values") {
		t.Errorf("Expected formatted message in output, got: %s", output)
	}
}

func TestWithField(t *testing.T) {
	var buf bytes.Buffer
	Initialize("info", "text")
	SetOutput(&buf)

	WithField("key", "value").Info("test message")

	output := buf.String()
	if !strings.Contains(output, "key=value") {
		t.Errorf("Expected 'key=value' in output, got: %s", output)
	}
	if !strings.Contains(output, "test message") {
		t.Errorf("Expected 'test message' in output, got: %s", output)
	}
}

func TestWithFields(t *testing.T) {
	var buf bytes.Buffer
	Initialize("info", "text")
	SetOutput(&buf)

	WithFields(logrus.Fields{
		"key1": "value1",
		"key2": "value2",
	}).Info("test message")

	output := buf.String()
	if !strings.Contains(output, "key1=value1") {
		t.Errorf("Expected 'key1=value1' in output, got: %s", output)
	}
	if !strings.Contains(output, "key2=value2") {
		t.Errorf("Expected 'key2=value2' in output, got: %s", output)
	}
	if !strings.Contains(output, "test message") {
		t.Errorf("Expected 'test message' in output, got: %s", output)
	}
}

func TestWithError(t *testing.T) {
	var buf bytes.Buffer
	Initialize("info", "text")
	SetOutput(&buf)

	testErr := errors.New("test error")
	WithError(testErr).Error("error occurred")

	output := buf.String()
	if !strings.Contains(output, "error occurred") {
		t.Errorf("Expected 'error occurred' in output, got: %s", output)
	}
	if !strings.Contains(output, "test error") {
		t.Errorf("Expected 'test error' in output, got: %s", output)
	}
}

func TestJSONFormat(t *testing.T) {
	var buf bytes.Buffer
	Initialize("info", "json")
	SetOutput(&buf)

	Info("test message")

	output := buf.String()
	// JSON output should contain these fields
	if !strings.Contains(output, `"msg":"test message"`) && !strings.Contains(output, `"message":"test message"`) {
		t.Errorf("Expected JSON formatted message in output, got: %s", output)
	}
	if !strings.Contains(output, `"level":"info"`) {
		t.Errorf("Expected level field in JSON output, got: %s", output)
	}
}

func TestConvenienceFunctions(t *testing.T) {
	var buf bytes.Buffer
	Initialize("debug", "text")
	SetOutput(&buf)

	tests := []struct {
		name    string
		logFunc func(string, ...interface{})
		message string
	}{
		{"Debugf", Debugf, "debug %s"},
		{"Infof", Infof, "info %s"},
		{"Warnf", Warnf, "warn %s"},
		{"Errorf", Errorf, "error %s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			tt.logFunc(tt.message, "test")

			output := buf.String()
			expectedMsg := strings.Replace(tt.message, "%s", "test", 1)
			if !strings.Contains(output, expectedMsg) {
				t.Errorf("Expected '%s' in output, got: %s", expectedMsg, output)
			}
		})
	}
}
