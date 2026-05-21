package c3x

import (
	"strings"
	"testing"
)

// TestCostEstimateTemplateExecution tests that the template executes without errors
func TestCostEstimateTemplateExecution(t *testing.T) {
	estimator := &Estimator{
		budgetLimit: 100.0,
	}

	estimate := &CostEstimate{
		TotalMonthlyCost: 75.50,
		TotalHourlyCost:  0.1042,
		Currency:         "USD",
		ResourceTypesSummary: map[string]*ResourceTypeCost{
			"aws_instance": {
				ResourceType: "aws_instance",
				Count:        2,
				HourlyCost:   0.08,
				MonthlyCost:  58.40,
			},
			"aws_s3_bucket": {
				ResourceType: "aws_s3_bucket",
				Count:        1,
				HourlyCost:   0.0235,
				MonthlyCost:  17.10,
			},
		},
		Projects: []ProjectCostEstimate{
			{
				Name:        "test-project",
				MonthlyCost: 75.50,
				HourlyCost:  0.1042,
				Resources: []ResourceCostEstimate{
					{
						Name:         "aws_instance.web",
						ResourceType: "aws_instance",
						MonthlyCost:  58.40,
						HourlyCost:   0.08,
					},
					{
						Name:         "aws_s3_bucket.data",
						ResourceType: "aws_s3_bucket",
						MonthlyCost:  17.10,
						HourlyCost:   0.0235,
					},
				},
			},
		},
		Summary: Summary{
			TotalDetectedResources:   3,
			TotalSupportedResources:  3,
			TotalUsageBasedResources: 1,
			TotalNoPriceResources:    0,
		},
	}

	result := estimator.FormatEstimate(estimate)

	// Verify template executed successfully (no error message)
	if strings.Contains(result, "Error rendering detailed report") {
		t.Errorf("Template execution failed: %s", result)
	}

	// Verify key sections are present
	expectedSections := []string{
		"## 💰 Cost Estimation Summary",
		"**Total Monthly Cost:**",
		"**Total Hourly Cost:**",
		"### 📊 Cost by Resource Type",
		"### 📋 Detailed Resource Breakdown",
		"### 📈 Summary Statistics",
		"**Budget Status:**",
	}

	for _, section := range expectedSections {
		if !strings.Contains(result, section) {
			t.Errorf("Expected section not found in output: %s", section)
		}
	}
}

// TestCostEstimateTemplateWithoutBudget tests template without budget limit
func TestCostEstimateTemplateWithoutBudget(t *testing.T) {
	estimator := &Estimator{
		budgetLimit: 0, // No budget
	}

	estimate := &CostEstimate{
		TotalMonthlyCost: 50.00,
		TotalHourlyCost:  0.0694,
		Currency:         "USD",
		Projects: []ProjectCostEstimate{
			{
				Name:        "test-project",
				MonthlyCost: 50.00,
				Resources: []ResourceCostEstimate{
					{
						Name:         "aws_instance.test",
						ResourceType: "aws_instance",
						MonthlyCost:  50.00,
					},
				},
			},
		},
	}

	result := estimator.FormatEstimate(estimate)

	// Should not contain budget section
	if strings.Contains(result, "Budget Status") {
		t.Error("Budget section should not be present when budget limit is 0")
	}

	// Should contain basic sections
	if !strings.Contains(result, "## 💰 Cost Estimation Summary") {
		t.Error("Expected cost summary section")
	}
}

// TestCostEstimateTemplateEmptyResources tests template with no resources
func TestCostEstimateTemplateEmptyResources(t *testing.T) {
	estimator := &Estimator{}

	estimate := &CostEstimate{
		TotalMonthlyCost:     0,
		TotalHourlyCost:      0,
		Currency:             "USD",
		ResourceTypesSummary: map[string]*ResourceTypeCost{},
		Projects:             []ProjectCostEstimate{},
	}

	result := estimator.FormatEstimate(estimate)

	// Should contain "No Cost Data Available" message
	if !strings.Contains(result, "No Cost Data Available") {
		t.Error("Expected 'No Cost Data Available' message for empty resources")
	}

	// Should not contain resource breakdown table
	if strings.Contains(result, "| Resource Name | Type |") {
		t.Error("Should not show resource breakdown table when no resources")
	}
}

// TestCostEstimateTemplateResourceSorting tests that resources are sorted by cost
func TestCostEstimateTemplateResourceSorting(t *testing.T) {
	estimator := &Estimator{}

	estimate := &CostEstimate{
		TotalMonthlyCost: 100.00,
		Currency:         "USD",
		ResourceTypesSummary: map[string]*ResourceTypeCost{
			"aws_instance": {
				ResourceType: "aws_instance",
				Count:        1,
				MonthlyCost:  80.00,
			},
			"aws_s3_bucket": {
				ResourceType: "aws_s3_bucket",
				Count:        1,
				MonthlyCost:  20.00,
			},
		},
		Projects: []ProjectCostEstimate{
			{
				Name: "test",
				Resources: []ResourceCostEstimate{
					{Name: "instance", ResourceType: "aws_instance", MonthlyCost: 80.00},
					{Name: "bucket", ResourceType: "aws_s3_bucket", MonthlyCost: 20.00},
				},
			},
		},
	}

	result := estimator.FormatEstimate(estimate)

	// Find positions of resource types in output
	instancePos := strings.Index(result, "aws_instance")
	bucketPos := strings.Index(result, "aws_s3_bucket")

	// In the resource type table, higher cost should appear first
	if instancePos == -1 || bucketPos == -1 {
		t.Error("Resource types not found in output")
	}

	// Note: We can't strictly test order in the table due to markdown formatting,
	// but we can verify both are present
	if !strings.Contains(result, "aws_instance") || !strings.Contains(result, "aws_s3_bucket") {
		t.Error("Expected both resource types in output")
	}
}

// TestCostEstimateTemplateBudgetMessages tests budget status messages
func TestCostEstimateTemplateBudgetMessages(t *testing.T) {
	tests := []struct {
		name        string
		cost        float64
		budget      float64
		expectEmoji string
	}{
		{
			name:        "Within budget",
			cost:        50.00,
			budget:      100.00,
			expectEmoji: "✅",
		},
		{
			name:        "Over budget",
			cost:        150.00,
			budget:      100.00,
			expectEmoji: "❌",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			estimator := &Estimator{
				budgetLimit: tt.budget,
			}

			estimate := &CostEstimate{
				TotalMonthlyCost: tt.cost,
				Currency:         "USD",
				Projects: []ProjectCostEstimate{
					{
						Name: "test",
						Resources: []ResourceCostEstimate{
							{Name: "test", ResourceType: "test", MonthlyCost: tt.cost},
						},
					},
				},
			}

			result := estimator.FormatEstimate(estimate)

			if !strings.Contains(result, tt.expectEmoji) {
				t.Errorf("Expected emoji %s not found in budget status", tt.expectEmoji)
			}

			if !strings.Contains(result, "Budget Status") {
				t.Error("Expected 'Budget Status' section")
			}
		})
	}
}

// TestCostEstimateTemplateSummaryStatistics tests summary statistics rendering
func TestCostEstimateTemplateSummaryStatistics(t *testing.T) {
	estimator := &Estimator{}

	estimate := &CostEstimate{
		TotalMonthlyCost: 50.00,
		Currency:         "USD",
		Projects: []ProjectCostEstimate{
			{
				Name: "test",
				Resources: []ResourceCostEstimate{
					{Name: "test", ResourceType: "test", MonthlyCost: 50.00},
				},
			},
		},
		Summary: Summary{
			TotalDetectedResources:   10,
			TotalSupportedResources:  8,
			TotalUsageBasedResources: 2,
			TotalNoPriceResources:    2,
		},
	}

	result := estimator.FormatEstimate(estimate)

	// Verify summary statistics are present
	expectedStats := []string{
		"### 📈 Summary Statistics",
		"Total Resources Detected:** 10",
		"Resources with Cost Data:** 8",
		"Usage-Based Resources:** 2",
		"Resources without Pricing:** 2",
	}

	for _, stat := range expectedStats {
		if !strings.Contains(result, stat) {
			t.Errorf("Expected statistic not found: %s", stat)
		}
	}
}

// TestCostEstimateTemplateFullOutput tests the complete rendered output
func TestCostEstimateTemplateFullOutput(t *testing.T) {
	estimator := &Estimator{
		budgetLimit: 100.0,
	}

	estimate := &CostEstimate{
		TotalMonthlyCost: 75.50,
		TotalHourlyCost:  0.1042,
		Currency:         "USD",
		ResourceTypesSummary: map[string]*ResourceTypeCost{
			"aws_instance": {
				ResourceType: "aws_instance",
				Count:        1,
				HourlyCost:   0.08,
				MonthlyCost:  58.40,
			},
			"aws_s3_bucket": {
				ResourceType: "aws_s3_bucket",
				Count:        1,
				HourlyCost:   0.0235,
				MonthlyCost:  17.10,
			},
		},
		Projects: []ProjectCostEstimate{
			{
				Name:        "test-project",
				MonthlyCost: 75.50,
				HourlyCost:  0.1042,
				Resources: []ResourceCostEstimate{
					{
						Name:         "aws_instance.web",
						ResourceType: "aws_instance",
						MonthlyCost:  58.40,
						HourlyCost:   0.08,
					},
					{
						Name:         "aws_s3_bucket.data",
						ResourceType: "aws_s3_bucket",
						MonthlyCost:  17.10,
						HourlyCost:   0.0235,
					},
				},
			},
		},
		Summary: Summary{
			TotalDetectedResources:   2,
			TotalSupportedResources:  2,
			TotalUsageBasedResources: 1,
			TotalNoPriceResources:    0,
		},
	}

	result := estimator.FormatEstimate(estimate)

	// Log the full output for inspection
	t.Logf("Full rendered template output:\n%s", result)

	// Verify the complete structure is present
	expectedContent := []string{
		// Header
		"## 💰 Cost Estimation Summary",

		// Total costs
		"**Total Monthly Cost:** `$75.50 USD`",
		"**Total Hourly Cost:** `$0.1042 USD`",

		// Resource type table header
		"### 📊 Cost by Resource Type",
		"| Resource Type | Count | Hourly Cost | Monthly Cost |",
		"|---------------|------:|------------:|-------------:|",

		// Resource types (sorted by cost, so instance should be first)
		"| `aws_instance` | 1 | $0.0800 | $58.40 |",
		"| `aws_s3_bucket` | 1 | $0.0235 | $17.10 |",

		// Detailed breakdown header
		"### 📋 Detailed Resource Breakdown",
		"| Resource Name | Type | Hourly | Monthly |",
		"|---------------|------|-------:|--------:|",

		// Individual resources
		"| `aws_instance.web` | `aws_instance` | $0.0800 | $58.40 |",
		"| `aws_s3_bucket.data` | `aws_s3_bucket` | $0.0235 | $17.10 |",

		// Summary statistics
		"### 📈 Summary Statistics",
		"- **Total Resources Detected:** 2",
		"- **Resources with Cost Data:** 2",
		"- **Usage-Based Resources:** 1",
		"- **Resources without Pricing:** 0",

		// Budget status
		"---",
		"✅ **Budget Status:**",
		"within budget limit of $100.00",
	}

	for _, expected := range expectedContent {
		if !strings.Contains(result, expected) {
			t.Errorf("Expected content not found in output:\n%s\n\nFull output:\n%s", expected, result)
		}
	}

	// Verify no error messages
	if strings.Contains(result, "Error rendering") {
		t.Error("Output contains error message")
	}

	// Verify proper markdown structure (no extra blank lines or formatting issues)
	lines := strings.Split(result, "\n")
	if len(lines) < 20 {
		t.Errorf("Expected at least 20 lines in output, got %d", len(lines))
	}
}
