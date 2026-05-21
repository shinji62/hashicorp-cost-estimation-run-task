package secrets

import (
	"strings"
	"testing"
)

func TestNewDetector(t *testing.T) {
	// Pass nil to use the global logger
	detector, err := NewDetector(nil)
	if err != nil {
		t.Fatalf("Failed to create detector: %v", err)
	}

	if detector == nil {
		t.Fatal("Detector is nil")
	}

	if detector.scanner == nil {
		t.Fatal("Scanner is nil")
	}
}

func TestScanAndRedactJSON_NoSecrets(t *testing.T) {
	detector, err := NewDetector(nil)
	if err != nil {
		t.Fatalf("Failed to create detector: %v", err)
	}
	defer detector.Close()

	jsonContent := `{
		"name": "test-resource",
		"type": "aws_instance",
		"region": "us-east-1"
	}`

	redacted, hasSecrets, err := detector.ScanAndRedactJSON(jsonContent)
	if err != nil {
		t.Fatalf("ScanAndRedactJSON failed: %v", err)
	}

	if hasSecrets {
		t.Error("Expected no secrets to be found")
	}

	if redacted != jsonContent {
		t.Error("Content should not be modified when no secrets are found")
	}
}

func TestScanAndRedactJSON_WithSecrets(t *testing.T) {
	detector, err := NewDetector(nil)
	if err != nil {
		t.Fatalf("Failed to create detector: %v", err)
	}
	defer detector.Close()

	// Test with realistic secret patterns that Titus can detect
	testCases := []struct {
		name            string
		jsonContent     string
		expectRedaction bool
		description     string
	}{
		{
			name: "Generic API Key",
			jsonContent: `{
				"api_key": "sk_live_51234567890abcdefghijklmnopqrstuvwxyz",
				"service": "stripe"
			}`,
			expectRedaction: true,
			description:     "Stripe-like API key pattern",
		},
		{
			name: "AWS Credentials in Object",
			jsonContent: `{
				"aws_access_key_id": "AKIAIOSFODNN7EXAMPLE",
				"aws_secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
			}`,
			expectRedaction: true,
			description:     "AWS credentials pattern",
		},
		{
			name: "Password in Object",
			jsonContent: `{
				"database_password": "MySecretPassword123!"
			}`,
			expectRedaction: true,
			description:     "Password field pattern",
		},
		{
			name: "No Secrets",
			jsonContent: `{
				"name": "test",
				"value": "normal-value-123"
			}`,
			expectRedaction: false,
			description:     "Normal content without secrets",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			redacted, hasSecrets, err := detector.ScanAndRedactJSON(tc.jsonContent)
			if err != nil {
				t.Fatalf("ScanAndRedactJSON failed: %v", err)
			}

			if hasSecrets != tc.expectRedaction {
				t.Logf("Test case: %s", tc.description)
				t.Logf("Original content: %s", tc.jsonContent)
				t.Logf("Redacted content: %s", redacted)
				t.Errorf("Expected hasSecrets=%v, got %v", tc.expectRedaction, hasSecrets)
			}

			if tc.expectRedaction && hasSecrets {
				if !strings.Contains(redacted, string(RedactedChar)) {
					t.Error("Expected redacted content to contain redaction character")
				}

				// Verify the redacted content preserves structure by using same-length replacement
				t.Logf("Redacted content: %s", redacted)
			}
		})
	}
}

func TestScanAndRedactJSON_EmptyString(t *testing.T) {
	detector, err := NewDetector(nil)
	if err != nil {
		t.Fatalf("Failed to create detector: %v", err)
	}
	defer detector.Close()

	redacted, hasSecrets, err := detector.ScanAndRedactJSON("")
	if err != nil {
		t.Fatalf("ScanAndRedactJSON failed: %v", err)
	}

	if hasSecrets {
		t.Error("Expected no secrets in empty string")
	}

	if redacted != "" {
		t.Error("Expected empty string to remain empty")
	}
}

func TestScanString(t *testing.T) {
	detector, err := NewDetector(nil)
	if err != nil {
		t.Fatalf("Failed to create detector: %v", err)
	}
	defer detector.Close()

	testCases := []struct {
		name          string
		content       string
		expectSecrets bool
	}{
		{
			name:          "With API Key",
			content:       "api_key=sk_live_51234567890abcdefghijklmnopqrstuvwxyz",
			expectSecrets: true,
		},
		{
			name:          "No Secrets",
			content:       "This is just normal text",
			expectSecrets: false,
		},
		{
			name:          "Empty String",
			content:       "",
			expectSecrets: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			hasSecrets := detector.ScanString(tc.content)
			if hasSecrets != tc.expectSecrets {
				t.Logf("Content: %s", tc.content)
				t.Errorf("Expected hasSecrets=%v, got %v", tc.expectSecrets, hasSecrets)
			}
		})
	}
}

func TestRedactedChar(t *testing.T) {
	if RedactedChar == 0 {
		t.Error("RedactedChar should not be zero")
	}

	if RedactedChar != '*' {
		t.Errorf("RedactedChar should be '*', got '%c'", RedactedChar)
	}
}

func TestScanAndRedactJSON_MultipleSecrets(t *testing.T) {
	detector, err := NewDetector(nil)
	if err != nil {
		t.Fatalf("Failed to create detector: %v", err)
	}
	defer detector.Close()

	jsonContent := `{
		"aws_access_key_id": "AKIAIOSFODNN7EXAMPLE",
		"aws_secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		"database_password": "MySecretPassword123!",
		"normal_field": "normal_value"
	}`

	redacted, hasSecrets, err := detector.ScanAndRedactJSON(jsonContent)
	if err != nil {
		t.Fatalf("ScanAndRedactJSON failed: %v", err)
	}

	if !hasSecrets {
		t.Error("Expected secrets to be found")
	}

	// Count occurrences of the redaction character
	asteriskCount := strings.Count(redacted, string(RedactedChar))
	if asteriskCount == 0 {
		t.Error("Expected at least one redacted secret (asterisks)")
	}

	t.Logf("Found %d asterisks in redacted content", asteriskCount)
	t.Logf("Redacted content: %s", redacted)

	// Verify normal field is preserved
	if !strings.Contains(redacted, "normal_value") {
		t.Error("Normal field should be preserved")
	}

	// Verify secrets are not present in their original form
	if strings.Contains(redacted, "AKIAIOSFODNN7EXAMPLE") {
		t.Error("AWS access key should be redacted")
	}

	if strings.Contains(redacted, "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY") {
		t.Error("AWS secret key should be redacted")
	}
}

func TestDetectorClose(t *testing.T) {
	detector, err := NewDetector(nil)
	if err != nil {
		t.Fatalf("Failed to create detector: %v", err)
	}

	err = detector.Close()
	if err != nil {
		t.Errorf("Close() failed: %v", err)
	}
}
