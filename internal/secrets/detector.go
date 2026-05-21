package secrets

import (
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-run-task-scaffolding-go/internal/helpers"
	"github.com/praetorian-inc/titus"
	"github.com/sirupsen/logrus"
)

const (
	// RedactedChar is the character used to replace each character of detected secrets
	RedactedChar = '*'
)

// Detector wraps the Titus scanner for secret detection
type Detector struct {
	scanner *titus.Scanner
	logger  *logrus.Logger
}

// NewDetector creates a new secret detector
func NewDetector(logger *logrus.Logger) (*Detector, error) {
	scanner, err := titus.NewScanner()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize secret scanner: %w", err)
	}

	// Use provided logger or get the global logger
	if logger == nil {
		logger = helpers.GetLogger()
	}

	return &Detector{
		scanner: scanner,
		logger:  logger,
	}, nil
}

// ScanAndRedactJSON scans JSON content for secrets and masks them with asterisks
func (d *Detector) ScanAndRedactJSON(jsonContent string) (string, bool, error) {
	if jsonContent == "" {
		return jsonContent, false, nil
	}

	matches, err := d.scanner.ScanString(jsonContent)
	if err != nil {
		return jsonContent, false, fmt.Errorf("failed to scan for secrets: %w", err)
	}

	if len(matches) == 0 {
		return jsonContent, false, nil
	}

	d.logger.Infof("Detected %d secret(s) in JSON content", len(matches))

	// Convert to byte slice for direct manipulation
	contentBytes := []byte(jsonContent)

	for _, match := range matches {
		startOffset := match.Location.Offset.Start
		endOffset := match.Location.Offset.End

		d.logger.Infof("Secret detected - Rule: %s (ID: %s), Line: %d",
			match.RuleName, match.RuleID, match.Location.Source.Start.Line)

		// Validate bounds to prevent runtime panics
		if startOffset >= 0 && endOffset > startOffset && int(endOffset) <= len(contentBytes) {
			// Fill the exact secret range with '*' characters
			for i := startOffset; i < endOffset; i++ {
				contentBytes[i] = RedactedChar
			}
		}
	}

	return string(contentBytes), true, nil
}

// ScanAndRedactMap scans a map for secrets and redacts them
// This is useful for scanning structured data before converting to JSON
func (d *Detector) ScanAndRedactMap(data map[string]interface{}) (map[string]interface{}, bool, error) {
	// Convert map to JSON
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return data, false, fmt.Errorf("failed to marshal map to JSON: %w", err)
	}

	// Scan and redact
	redactedJSON, hasSecrets, err := d.ScanAndRedactJSON(string(jsonBytes))
	if err != nil {
		return data, false, err
	}

	if !hasSecrets {
		return data, false, nil
	}

	// Convert back to map
	var redactedData map[string]interface{}
	if err := json.Unmarshal([]byte(redactedJSON), &redactedData); err != nil {
		return data, true, fmt.Errorf("failed to unmarshal redacted JSON: %w", err)
	}

	return redactedData, true, nil
}

// ScanString scans a string for secrets without redaction
// Returns true if secrets are found
func (d *Detector) ScanString(content string) bool {
	matches, err := d.scanner.ScanString(content)
	if err != nil {
		d.logger.Warnf("Failed to scan string for secrets: %v", err)
		return false
	}
	return len(matches) > 0
}

// Close closes the scanner and releases resources
func (d *Detector) Close() error {
	if d.scanner != nil {
		return d.scanner.Close()
	}
	return nil
}
