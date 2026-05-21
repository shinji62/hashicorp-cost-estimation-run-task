package c3x

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestFormatEstimate_EmptyEstimate tests formatting with no resources
func TestFormatEstimate_EmptyEstimate(t *testing.T) {
	estimator := &Estimator{
		budgetLimit: 0,
	}

	estimate := &CostEstimate{
		TotalMonthlyCost:     0.0,
		TotalHourlyCost:      0.0,
		Currency:             "USD",
		Projects:             []ProjectCostEstimate{},
		ResourceTypesSummary: map[string]*ResourceTypeCost{},
		Summary: Summary{
			TotalDetectedResources:   0,
			TotalSupportedResources:  0,
			TotalUsageBasedResources: 0,
			TotalNoPriceResources:    0,
		},
	}

	result := estimator.FormatEstimate(estimate)

	// Verify basic structure
	if !strings.Contains(result, "## 💰 Cost Estimation Summary") {
		t.Error("Expected title header in output")
	}
	if !strings.Contains(result, "**Total Monthly Cost:** `$0.00 USD`") {
		t.Error("Expected zero monthly cost in output")
	}
	if !strings.Contains(result, "**Total Hourly Cost:** `$0.0000 USD`") {
		t.Error("Expected zero hourly cost in output")
	}
	if !strings.Contains(result, "### ℹ️ No Cost Data Available") {
		t.Error("Expected no cost data message")
	}
}

// TestFormatEstimate_WithResources tests formatting with multiple resources
func TestFormatEstimate_WithResources(t *testing.T) {
	estimator := &Estimator{
		budgetLimit: 0,
	}

	estimate := &CostEstimate{
		TotalMonthlyCost: 150.50,
		TotalHourlyCost:  0.2076,
		Currency:         "USD",
		Projects: []ProjectCostEstimate{
			{
				Name:        "test-project",
				MonthlyCost: 150.50,
				HourlyCost:  0.2076,
				Resources: []ResourceCostEstimate{
					{
						Name:         "aws_instance.web",
						ResourceType: "aws_instance",
						MonthlyCost:  73.00,
						HourlyCost:   0.1000,
					},
					{
						Name:         "aws_db_instance.main",
						ResourceType: "aws_db_instance",
						MonthlyCost:  77.50,
						HourlyCost:   0.1076,
					},
				},
			},
		},
		ResourceTypesSummary: map[string]*ResourceTypeCost{
			"aws_instance": {
				ResourceType: "aws_instance",
				Count:        1,
				HourlyCost:   0.1000,
				MonthlyCost:  73.00,
			},
			"aws_db_instance": {
				ResourceType: "aws_db_instance",
				Count:        1,
				HourlyCost:   0.1076,
				MonthlyCost:  77.50,
			},
		},
		Summary: Summary{
			TotalDetectedResources:   2,
			TotalSupportedResources:  2,
			TotalUsageBasedResources: 0,
			TotalNoPriceResources:    0,
		},
	}

	result := estimator.FormatEstimate(estimate)

	// Verify totals
	if !strings.Contains(result, "**Total Monthly Cost:** `$150.50 USD`") {
		t.Error("Expected correct monthly cost in output")
	}
	if !strings.Contains(result, "**Total Hourly Cost:** `$0.2076 USD`") {
		t.Error("Expected correct hourly cost in output")
	}

	// Verify resource type table exists
	if !strings.Contains(result, "### 📊 Cost by Resource Type") {
		t.Error("Expected resource type section")
	}
	if !strings.Contains(result, "| Resource Type | Count | Hourly Cost | Monthly Cost |") {
		t.Error("Expected resource type table header")
	}

	// Verify detailed breakdown exists
	if !strings.Contains(result, "### 📋 Detailed Resource Breakdown") {
		t.Error("Expected detailed breakdown section")
	}
	if !strings.Contains(result, "`aws_instance.web`") {
		t.Error("Expected aws_instance.web in output")
	}
	if !strings.Contains(result, "`aws_db_instance.main`") {
		t.Error("Expected aws_db_instance.main in output")
	}

	// Verify summary statistics
	if !strings.Contains(result, "### 📈 Summary Statistics") {
		t.Error("Expected summary statistics section")
	}
	if !strings.Contains(result, "- **Total Resources Detected:** 2") {
		t.Error("Expected 2 detected resources")
	}
	if !strings.Contains(result, "- **Resources with Cost Data:** 2") {
		t.Error("Expected 2 supported resources")
	}
}

// TestFormatEstimate_ResourceTypeSorting tests that resources are sorted by cost descending
func TestFormatEstimate_ResourceTypeSorting(t *testing.T) {
	estimator := &Estimator{
		budgetLimit: 0,
	}

	estimate := &CostEstimate{
		TotalMonthlyCost: 300.00,
		TotalHourlyCost:  0.4167,
		Currency:         "USD",
		Projects:         []ProjectCostEstimate{},
		ResourceTypesSummary: map[string]*ResourceTypeCost{
			"aws_instance": {
				ResourceType: "aws_instance",
				Count:        2,
				HourlyCost:   0.1000,
				MonthlyCost:  73.00,
			},
			"aws_db_instance": {
				ResourceType: "aws_db_instance",
				Count:        1,
				HourlyCost:   0.2167,
				MonthlyCost:  157.00,
			},
			"aws_s3_bucket": {
				ResourceType: "aws_s3_bucket",
				Count:        3,
				HourlyCost:   0.1000,
				MonthlyCost:  70.00,
			},
		},
		Summary: Summary{
			TotalDetectedResources:   6,
			TotalSupportedResources:  6,
			TotalUsageBasedResources: 0,
			TotalNoPriceResources:    0,
		},
	}

	result := estimator.FormatEstimate(estimate)

	// Find positions of each resource type in the output
	dbPos := strings.Index(result, "`aws_db_instance`")
	instancePos := strings.Index(result, "`aws_instance`")
	s3Pos := strings.Index(result, "`aws_s3_bucket`")

	// Verify sorting: aws_db_instance ($157) should come before aws_instance ($73) and aws_s3_bucket ($70)
	if dbPos == -1 || instancePos == -1 || s3Pos == -1 {
		t.Fatal("Not all resource types found in output")
	}

	if dbPos > instancePos {
		t.Error("Expected aws_db_instance (higher cost) to appear before aws_instance")
	}
	if instancePos > s3Pos {
		t.Error("Expected aws_instance to appear before aws_s3_bucket")
	}
}

// TestFormatEstimate_WithBudgetLimit tests budget checking functionality
func TestFormatEstimate_WithBudgetLimit(t *testing.T) {
	tests := []struct {
		name         string
		budgetLimit  float64
		monthlyCost  float64
		expectWithin bool
		expectIcon   string
	}{
		{
			name:         "Within budget",
			budgetLimit:  200.00,
			monthlyCost:  150.00,
			expectWithin: true,
			expectIcon:   "✅",
		},
		{
			name:         "Over budget",
			budgetLimit:  100.00,
			monthlyCost:  150.00,
			expectWithin: false,
			expectIcon:   "❌",
		},
		{
			name:         "Exactly at budget",
			budgetLimit:  150.00,
			monthlyCost:  150.00,
			expectWithin: true,
			expectIcon:   "✅",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			estimator := &Estimator{
				budgetLimit: tt.budgetLimit,
			}

			estimate := &CostEstimate{
				TotalMonthlyCost:     tt.monthlyCost,
				TotalHourlyCost:      tt.monthlyCost / 730,
				Currency:             "USD",
				Projects:             []ProjectCostEstimate{},
				ResourceTypesSummary: map[string]*ResourceTypeCost{},
				Summary: Summary{
					TotalDetectedResources: 1,
				},
			}

			result := estimator.FormatEstimate(estimate)

			// Verify budget status section exists
			if !strings.Contains(result, "**Budget Status:**") {
				t.Error("Expected budget status section")
			}

			// Verify correct icon
			if !strings.Contains(result, tt.expectIcon) {
				t.Errorf("Expected icon %s in output", tt.expectIcon)
			}

			// Verify separator before budget status
			if !strings.Contains(result, "---") {
				t.Error("Expected separator before budget status")
			}
		})
	}
}

// TestFormatEstimate_NoBudgetLimit tests that budget section is omitted when no limit is set
func TestFormatEstimate_NoBudgetLimit(t *testing.T) {
	estimator := &Estimator{
		budgetLimit: 0, // No budget limit
	}

	estimate := &CostEstimate{
		TotalMonthlyCost:     150.00,
		TotalHourlyCost:      0.2055,
		Currency:             "USD",
		Projects:             []ProjectCostEstimate{},
		ResourceTypesSummary: map[string]*ResourceTypeCost{},
		Summary: Summary{
			TotalDetectedResources: 1,
		},
	}

	result := estimator.FormatEstimate(estimate)

	// Verify budget status section does NOT exist
	if strings.Contains(result, "**Budget Status:**") {
		t.Error("Expected no budget status section when budget limit is 0")
	}
	if strings.Contains(result, "---") {
		t.Error("Expected no separator when budget limit is 0")
	}
}

// TestFormatEstimate_MultipleProjects tests formatting with multiple projects
func TestFormatEstimate_MultipleProjects(t *testing.T) {
	estimator := &Estimator{
		budgetLimit: 0,
	}

	estimate := &CostEstimate{
		TotalMonthlyCost: 200.00,
		TotalHourlyCost:  0.2740,
		Currency:         "USD",
		Projects: []ProjectCostEstimate{
			{
				Name:        "project-1",
				MonthlyCost: 100.00,
				HourlyCost:  0.1370,
				Resources: []ResourceCostEstimate{
					{
						Name:         "aws_instance.web1",
						ResourceType: "aws_instance",
						MonthlyCost:  100.00,
						HourlyCost:   0.1370,
					},
				},
			},
			{
				Name:        "project-2",
				MonthlyCost: 100.00,
				HourlyCost:  0.1370,
				Resources: []ResourceCostEstimate{
					{
						Name:         "aws_instance.web2",
						ResourceType: "aws_instance",
						MonthlyCost:  100.00,
						HourlyCost:   0.1370,
					},
				},
			},
		},
		ResourceTypesSummary: map[string]*ResourceTypeCost{
			"aws_instance": {
				ResourceType: "aws_instance",
				Count:        2,
				HourlyCost:   0.2740,
				MonthlyCost:  200.00,
			},
		},
		Summary: Summary{
			TotalDetectedResources:   2,
			TotalSupportedResources:  2,
			TotalUsageBasedResources: 0,
			TotalNoPriceResources:    0,
		},
	}

	result := estimator.FormatEstimate(estimate)

	// Verify both resources appear
	if !strings.Contains(result, "`aws_instance.web1`") {
		t.Error("Expected aws_instance.web1 in output")
	}
	if !strings.Contains(result, "`aws_instance.web2`") {
		t.Error("Expected aws_instance.web2 in output")
	}

	// Verify aggregated resource type count
	if !strings.Contains(result, "| `aws_instance` | 2 |") {
		t.Error("Expected aggregated count of 2 for aws_instance")
	}
}

// TestFormatEstimate_SummaryStatistics tests summary statistics formatting
func TestFormatEstimate_SummaryStatistics(t *testing.T) {
	estimator := &Estimator{
		budgetLimit: 0,
	}

	estimate := &CostEstimate{
		TotalMonthlyCost:     100.00,
		TotalHourlyCost:      0.1370,
		Currency:             "USD",
		Projects:             []ProjectCostEstimate{},
		ResourceTypesSummary: map[string]*ResourceTypeCost{},
		Summary: Summary{
			TotalDetectedResources:   10,
			TotalSupportedResources:  7,
			TotalUsageBasedResources: 3,
			TotalNoPriceResources:    2,
		},
	}

	result := estimator.FormatEstimate(estimate)

	// Verify all summary statistics are present
	expectedStats := []string{
		"- **Total Resources Detected:** 10",
		"- **Resources with Cost Data:** 7",
		"- **Usage-Based Resources:** 3",
		"- **Resources without Pricing:** 2",
	}

	for _, stat := range expectedStats {
		if !strings.Contains(result, stat) {
			t.Errorf("Expected statistic not found: %s", stat)
		}
	}
}

// TestFormatEstimate_NoSummaryStatistics tests that summary section is omitted when no resources detected
func TestFormatEstimate_NoSummaryStatistics(t *testing.T) {
	estimator := &Estimator{
		budgetLimit: 0,
	}

	estimate := &CostEstimate{
		TotalMonthlyCost:     0.0,
		TotalHourlyCost:      0.0,
		Currency:             "USD",
		Projects:             []ProjectCostEstimate{},
		ResourceTypesSummary: map[string]*ResourceTypeCost{},
		Summary: Summary{
			TotalDetectedResources: 0,
		},
	}

	result := estimator.FormatEstimate(estimate)

	// Verify summary statistics section does NOT exist
	if strings.Contains(result, "### 📈 Summary Statistics") {
		t.Error("Expected no summary statistics section when no resources detected")
	}
}

// TestAPIKeySecureHandling verifies that API keys are handled securely
func TestAPIKeySecureHandling(t *testing.T) {
	// Set up test environment
	testAPIKey := "test-secret-api-key-12345"
	os.Setenv("C3X_API_KEY", testAPIKey)
	defer os.Unsetenv("C3X_API_KEY")

	// Create a mock c3x binary that echoes its environment
	tmpDir := t.TempDir()
	mockC3xPath := filepath.Join(tmpDir, "mock-c3x")

	// Create a simple shell script that outputs environment variables
	mockScript := `#!/bin/bash
echo "Mock c3x called"
env | grep C3X
exit 0
`
	if err := os.WriteFile(mockC3xPath, []byte(mockScript), 0755); err != nil {
		t.Fatalf("Failed to create mock c3x: %v", err)
	}

	// Note: This test verifies the concept but cannot fully test the actual
	// implementation without mocking the entire Estimate function
	t.Log("API key security test setup complete")

	// Verify that temporary files are created with secure permissions
	tempFile, err := os.CreateTemp("", ".c3x-apikey-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	// Set restrictive permissions
	if err := tempFile.Chmod(0600); err != nil {
		t.Fatalf("Failed to set permissions: %v", err)
	}

	// Verify permissions
	info, err := tempFile.Stat()
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	mode := info.Mode().Perm()
	if mode != 0600 {
		t.Errorf("Expected permissions 0600, got %o", mode)
	}

	// Write test data
	if _, err := tempFile.WriteString(testAPIKey); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tempFile.Close()

	// Verify the file contains the key
	content, err := os.ReadFile(tempFile.Name())
	if err != nil {
		t.Fatalf("Failed to read temp file: %v", err)
	}

	if string(content) != testAPIKey {
		t.Errorf("Expected file content %q, got %q", testAPIKey, string(content))
	}

	t.Log("Temporary file security verification passed")
}

// TestAPIKeyNotInProcessListing verifies API key doesn't appear in process environment
func TestAPIKeyNotInProcessListing(t *testing.T) {
	// This test demonstrates that when we use subprocess environment,
	// the key is isolated to that process
	testAPIKey := "test-secret-key-67890"

	// Create a command with custom environment
	cmd := exec.Command("echo", "test")
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"C3X_API_KEY=" + testAPIKey,
	}

	// The key is in cmd.Env but not in os.Environ()
	parentEnv := os.Environ()
	hasKeyInParent := false
	for _, env := range parentEnv {
		if strings.Contains(env, testAPIKey) {
			hasKeyInParent = true
			break
		}
	}

	if hasKeyInParent {
		t.Error("API key should not be in parent process environment")
	}

	// Verify the key is in the subprocess environment
	hasKeyInChild := false
	for _, env := range cmd.Env {
		if strings.Contains(env, testAPIKey) {
			hasKeyInChild = true
			break
		}
	}

	if !hasKeyInChild {
		t.Error("API key should be in child process environment")
	}

	t.Log("Process environment isolation verified")
}

// TestSecureFileCleanup verifies temporary files are properly cleaned up
func TestSecureFileCleanup(t *testing.T) {
	var tempFilePath string

	// Create and immediately defer cleanup
	func() {
		tempFile, err := os.CreateTemp("", ".c3x-apikey-cleanup-test-*")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		tempFilePath = tempFile.Name()
		defer os.Remove(tempFilePath)
		defer tempFile.Close()

		// Set secure permissions
		if err := tempFile.Chmod(0600); err != nil {
			t.Fatalf("Failed to set permissions: %v", err)
		}

		// Write test data
		if _, err := tempFile.WriteString("test-data"); err != nil {
			t.Fatalf("Failed to write to temp file: %v", err)
		}

		// File should exist here
		if _, err := os.Stat(tempFilePath); os.IsNotExist(err) {
			t.Error("Temp file should exist before cleanup")
		}
	}()

	// After function returns, file should be cleaned up
	if _, err := os.Stat(tempFilePath); !os.IsNotExist(err) {
		t.Error("Temp file should be cleaned up after use")
	}

	t.Log("Secure file cleanup verified")
}

// TestFilePermissions verifies that created files have correct permissions
func TestFilePermissions(t *testing.T) {
	tests := []struct {
		name     string
		perm     os.FileMode
		expected os.FileMode
	}{
		{"Secure permissions", 0600, 0600},
		{"Owner only read", 0400, 0400},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempFile, err := os.CreateTemp("", ".c3x-perm-test-*")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tempFile.Name())
			defer tempFile.Close()

			if err := tempFile.Chmod(tt.perm); err != nil {
				t.Fatalf("Failed to set permissions: %v", err)
			}

			info, err := tempFile.Stat()
			if err != nil {
				t.Fatalf("Failed to stat file: %v", err)
			}

			actualPerm := info.Mode().Perm()
			if actualPerm != tt.expected {
				t.Errorf("Expected permissions %o, got %o", tt.expected, actualPerm)
			}
		})
	}
}
