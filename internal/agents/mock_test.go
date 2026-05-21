package agents

import (
	"log"
	"os"
	"testing"

	tfjson "github.com/hashicorp/terraform-json"
	"github.com/hashicorp/terraform-run-task-scaffolding-go/internal/c3x"
)

// TestMockAgentResponses demonstrates example agent responses
func TestMockAgentResponses(t *testing.T) {
	// Mock Terraform plan
	plan := tfjson.Plan{
		PlannedValues: &tfjson.StateValues{
			RootModule: &tfjson.StateModule{
				Resources: []*tfjson.StateResource{
					{
						Address: "aws_s3_bucket.data",
						Type:    "aws_s3_bucket",
						Name:    "data",
					},
					{
						Address: "aws_instance.web",
						Type:    "aws_instance",
						Name:    "web",
					},
				},
			},
		},
		ResourceChanges: []*tfjson.ResourceChange{
			{
				Address: "aws_s3_bucket.data",
				Type:    "aws_s3_bucket",
				Change: &tfjson.Change{
					Actions: []tfjson.Action{tfjson.ActionCreate},
				},
			},
			{
				Address: "aws_instance.web",
				Type:    "aws_instance",
				Change: &tfjson.Change{
					Actions: []tfjson.Action{tfjson.ActionCreate},
				},
			},
		},
	}

	// Mock cost estimate
	estimate := &c3x.CostEstimate{
		TotalMonthlyCost: 150.50,
		TotalHourlyCost:  0.21,
		Currency:         "USD",
		Projects: []c3x.ProjectCostEstimate{
			{
				Name:        "main",
				MonthlyCost: 150.50,
				HourlyCost:  0.21,
				Resources: []c3x.ResourceCostEstimate{
					{
						Name:         "aws_s3_bucket.data",
						ResourceType: "aws_s3_bucket",
						MonthlyCost:  5.50,
						HourlyCost:   0.01,
					},
					{
						Name:         "aws_instance.web",
						ResourceType: "aws_instance",
						MonthlyCost:  145.00,
						HourlyCost:   0.20,
					},
				},
			},
		},
	}

	// Create mock coordinator
	coordinator := &Coordinator{
		logger: nil,
	}

	// Test building analysis input
	input := coordinator.buildAnalysisInput(plan, estimate, 200.00)

	if len(input) == 0 {
		t.Error("Expected non-empty analysis input")
	}

	t.Logf("Generated analysis input:\n%s", input)
}

// MockSecurityResponse returns an example security analysis response
func MockSecurityResponse() *AgentResponse {
	return &AgentResponse{
		AgentType:  "security",
		AgentName:  "Security Risk Analyzer",
		Status:     "warning",
		Summary:    "Found 2 security issues requiring attention",
		Confidence: 0.95,
		Findings: []Finding{
			{
				Severity:        "critical",
				Title:           "S3 Bucket Publicly Accessible",
				Description:     "The S3 bucket 'data' is configured with public read access, exposing all stored data to the internet.",
				ResourceName:    "aws_s3_bucket.data",
				ResourceType:    "aws_s3_bucket",
				Recommendation:  "Remove public access configuration and use IAM policies for controlled access. Enable bucket encryption and versioning.",
				EstimatedImpact: "High risk of data breach and compliance violations",
			},
			{
				Severity:        "high",
				Title:           "EC2 Instance Missing Encryption",
				Description:     "The EC2 instance 'web' does not have EBS volume encryption enabled.",
				ResourceName:    "aws_instance.web",
				ResourceType:    "aws_instance",
				Recommendation:  "Enable EBS encryption for all volumes. Use AWS KMS for key management.",
				EstimatedImpact: "Data at rest is not protected, potential compliance violation",
			},
		},
	}
}

// MockPricingResponse returns an example pricing optimization response
func MockPricingResponse() *AgentResponse {
	return &AgentResponse{
		AgentType:  "pricing",
		AgentName:  "Pricing Optimization Advisor",
		Status:     "passed",
		Summary:    "Found 2 cost optimization opportunities totaling $45/month savings",
		Confidence: 0.90,
		Findings: []Finding{
			{
				Severity:        "medium",
				Title:           "EC2 Instance Right-Sizing Opportunity",
				Description:     "The t3.large instance appears oversized for typical web server workloads. Historical data suggests t3.medium would be sufficient.",
				ResourceName:    "aws_instance.web",
				ResourceType:    "aws_instance",
				Recommendation:  "Consider using t3.medium instance type instead of t3.large. This maintains performance while reducing costs.",
				EstimatedImpact: "Save approximately $35/month (24% reduction)",
			},
			{
				Severity:        "low",
				Title:           "S3 Storage Tier Optimization",
				Description:     "S3 bucket using Standard storage tier. If data access is infrequent, consider using Intelligent-Tiering.",
				ResourceName:    "aws_s3_bucket.data",
				ResourceType:    "aws_s3_bucket",
				Recommendation:  "Enable S3 Intelligent-Tiering to automatically move objects between access tiers based on usage patterns.",
				EstimatedImpact: "Save approximately $10/month on storage costs",
			},
		},
	}
}

// TestMockResponseFormatting tests the formatting of mock responses
func TestMockResponseFormatting(t *testing.T) {
	coordinator := &Coordinator{
		logger: log.New(os.Stdout, "TEST: ", log.Ldate|log.Ltime),
	}

	responses := []*AgentResponse{
		MockSecurityResponse(),
		MockPricingResponse(),
	}

	formatted := coordinator.FormatResponses(responses)

	if len(formatted) == 0 {
		t.Error("Expected non-empty formatted output")
	}

	t.Logf("Formatted agent responses:\n%s", formatted)
}
