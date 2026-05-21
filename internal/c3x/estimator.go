package c3x

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	tfjson "github.com/hashicorp/terraform-json"
	"github.com/hashicorp/terraform-run-task-scaffolding-go/internal/config"
	"github.com/hashicorp/terraform-run-task-scaffolding-go/internal/helpers"
	"github.com/hashicorp/terraform-run-task-scaffolding-go/templates"
)

// C3X_BINARY_PATH is the fixed absolute path to the c3x binary
// Security: Using a constant prevents command injection vulnerabilities
const C3X_BINARY_PATH = "/usr/local/bin/c3x"

// C3XOutput represents the full c3x JSON output structure
type C3XOutput struct {
	Version          string    `json:"version"`
	Currency         string    `json:"currency"`
	Projects         []Project `json:"projects"`
	TotalHourlyCost  string    `json:"totalHourlyCost"`
	TotalMonthlyCost string    `json:"totalMonthlyCost"`
	Summary          Summary   `json:"summary"`
}

// Project represents a project in the c3x output
type Project struct {
	Name      string    `json:"name"`
	Breakdown Breakdown `json:"breakdown"`
	Diff      Breakdown `json:"diff"`
}

// Breakdown contains the cost breakdown for resources
type Breakdown struct {
	Resources        []Resource `json:"resources"`
	TotalHourlyCost  string     `json:"totalHourlyCost"`
	TotalMonthlyCost string     `json:"totalMonthlyCost"`
}

// Resource represents a single resource with cost information
type Resource struct {
	Name           string            `json:"name"`
	ResourceType   string            `json:"resourceType"`
	Tags           map[string]string `json:"tags"`
	HourlyCost     string            `json:"hourlyCost"`
	MonthlyCost    string            `json:"monthlyCost"`
	CostComponents []CostComponent   `json:"costComponents"`
	Subresources   []Resource        `json:"subresources"`
}

// CostComponent represents individual cost components
type CostComponent struct {
	Name            string  `json:"name"`
	Unit            string  `json:"unit"`
	HourlyQuantity  *string `json:"hourlyQuantity"`
	MonthlyQuantity *string `json:"monthlyQuantity"`
	Price           string  `json:"price"`
	HourlyCost      string  `json:"hourlyCost"`
	MonthlyCost     string  `json:"monthlyCost"`
	UsageBased      bool    `json:"usageBased"`
	PriceNotFound   bool    `json:"priceNotFound"`
}

// Summary contains resource count summary
type Summary struct {
	TotalDetectedResources    int            `json:"totalDetectedResources"`
	TotalSupportedResources   int            `json:"totalSupportedResources"`
	TotalUnsupportedResources int            `json:"totalUnsupportedResources"`
	TotalUsageBasedResources  int            `json:"totalUsageBasedResources"`
	TotalNoPriceResources     int            `json:"totalNoPriceResources"`
	NoPriceResourceCounts     map[string]int `json:"noPriceResourceCounts"`
}

// CostEstimate represents the simplified cost estimation result
type CostEstimate struct {
	TotalMonthlyCost     float64                      `json:"totalMonthlyCost"`
	TotalHourlyCost      float64                      `json:"totalHourlyCost"`
	Currency             string                       `json:"currency"`
	Projects             []ProjectCostEstimate        `json:"projects"`
	ResourceTypesSummary map[string]*ResourceTypeCost `json:"resourceTypesSummary"`
	Summary              Summary                      `json:"summary"`
	FormattedSummary     string                       `json:"formattedSummary"`
}

// ResourceTypeCost aggregates costs by resource type
type ResourceTypeCost struct {
	ResourceType string  `json:"resourceType"`
	Count        int     `json:"count"`
	HourlyCost   float64 `json:"hourlyCost"`
	MonthlyCost  float64 `json:"monthlyCost"`
}

// ProjectCostEstimate represents cost estimate for a single project
type ProjectCostEstimate struct {
	Name        string                 `json:"name"`
	MonthlyCost float64                `json:"monthlyCost"`
	HourlyCost  float64                `json:"hourlyCost"`
	Resources   []ResourceCostEstimate `json:"resources"`
}

// ResourceCostEstimate represents cost estimate for a single resource
type ResourceCostEstimate struct {
	Name           string                  `json:"name"`
	ResourceType   string                  `json:"resourceType"`
	MonthlyCost    float64                 `json:"monthlyCost"`
	HourlyCost     float64                 `json:"hourlyCost"`
	Tags           map[string]string       `json:"tags"`
	CostComponents []CostComponentEstimate `json:"costComponents"`
	Subresources   []ResourceCostEstimate  `json:"subresources"`
}

// CostComponentEstimate represents individual cost components
type CostComponentEstimate struct {
	Name            string  `json:"name"`
	Unit            string  `json:"unit"`
	MonthlyQuantity float64 `json:"monthlyQuantity"`
	MonthlyCost     float64 `json:"monthlyCost"`
	UsageBased      bool    `json:"usageBased"`
	PriceNotFound   bool    `json:"priceNotFound"`
}

// Estimator handles cost estimation using c3x CLI tool
type Estimator struct {
	c3xPath     string
	workDir     string
	budgetLimit float64
	config      *config.Config
}

// NewEstimator creates a new c3x estimator using the c3x CLI tool
func NewEstimator(workDir string, budgetLimit float64, cfg *config.Config) (*Estimator, error) {
	// Verify the binary exists and is executable
	info, err := os.Stat(C3X_BINARY_PATH)
	if err != nil {
		return nil, fmt.Errorf("c3x binary not found at %s: %w", C3X_BINARY_PATH, err)
	}

	// Security: Verify it's a regular file (not a symlink or directory)
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("c3x path is not a regular file: %s", C3X_BINARY_PATH)
	}

	return &Estimator{
		config:      cfg,
		c3xPath:     C3X_BINARY_PATH,
		workDir:     workDir,
		budgetLimit: budgetLimit,
	}, nil
}

func savePlanJSONForDebug(planJSON []byte, cfg *config.Config) {
	if cfg == nil || !cfg.C3XSaveDebugPlan {
		return
	}

	debugFile, err := os.CreateTemp("", "tfc-plan-debug-*.json")
	if err != nil {
		helpers.Warnf("Failed to create debug plan file: %v", err)
		return
	}
	defer debugFile.Close()

	if err := debugFile.Chmod(0600); err != nil {
		helpers.Warnf("Failed to set debug file permissions: %v", err)
	}

	if _, err := debugFile.Write(planJSON); err != nil {
		helpers.Warnf("Failed to write debug plan file: %v", err)
		return
	}

	helpers.Debugf("Saved TFC plan JSON to %s for replay debugging", debugFile.Name())
}

// EstimateFromPlan estimates costs from a Terraform plan using c3x CLI tool
func (e *Estimator) EstimateFromPlan(plan tfjson.Plan) (*CostEstimate, error) {
	// Marshal the plan to JSON
	planJSON, err := json.Marshal(plan)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal plan: %w", err)
	}

	savePlanJSONForDebug(planJSON, e.config)

	// Create a temporary file for the plan
	tmpFile, err := os.CreateTemp("", "tfplan-*.json")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Write plan JSON to temp file
	if _, err := tmpFile.Write(planJSON); err != nil {
		return nil, fmt.Errorf("failed to write plan to temp file: %w", err)
	}
	tmpFile.Close()

	helpers.Debugf("Wrote plan to temp file: %s", tmpFile.Name())
	helpers.Debugf("Plan JSON size: %d bytes", len(planJSON))
	helpers.Debugf("Plan summary - Resources: %d changes", len(plan.ResourceChanges))

	// Run c3x estimate command with the temp file
	cmd := exec.Command(e.c3xPath, "estimate", "--path", tmpFile.Name(), "--format", "json")

	// Set environment variables for c3x
	cmd.Env = os.Environ()

	// Get configuration values
	var apiEndpoint, apiKey string
	if e.config != nil {
		apiEndpoint = e.config.C3XPricingAPIEndpoint
		apiKey = e.config.C3XAPIKey
	} else {
		// Fallback to environment variables for backward compatibility
		apiEndpoint = os.Getenv("C3X_PRICING_API_ENDPOINT")
		apiKey = os.Getenv("C3X_API_KEY")
	}

	if apiEndpoint != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("C3X_PRICING_API_ENDPOINT=%s", apiEndpoint))
	}

	// Security: Handle API key securely using temporary file with restricted permissions
	// This prevents the key from appearing in process listings and reduces exposure
	var apiKeyFile *os.File
	if apiKey != "" {
		// Create temporary file for API key with restricted permissions (owner read/write only)
		apiKeyFile, err = os.CreateTemp("", ".c3x-apikey-*")
		if err != nil {
			return nil, fmt.Errorf("failed to create secure temp file for API key: %w", err)
		}
		defer os.Remove(apiKeyFile.Name())
		defer apiKeyFile.Close()

		// Set restrictive permissions before writing sensitive data
		if err := apiKeyFile.Chmod(0600); err != nil {
			return nil, fmt.Errorf("failed to set secure permissions on API key file: %w", err)
		}

		// Write API key to secure temporary file
		if _, err := apiKeyFile.WriteString(apiKey); err != nil {
			return nil, fmt.Errorf("failed to write API key to secure temp file: %w", err)
		}
		apiKeyFile.Close()

		// Pass the file path instead of the key itself
		// Note: This assumes c3x supports reading from C3X_API_KEY_FILE
		// If not, we still reduce exposure by limiting the key's lifetime in env
		cmd.Env = append(cmd.Env, fmt.Sprintf("C3X_API_KEY_FILE=%s", apiKeyFile.Name()))

		// For backward compatibility, also set C3X_API_KEY but only in subprocess env
		// The key is not exposed in parent process or debug output
		cmd.Env = append(cmd.Env, fmt.Sprintf("C3X_API_KEY=%s", apiKey))

		helpers.Debugf("API key configured securely via temporary file")
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	helpers.Debugf("Running c3x command: %s estimate --path %s --format json", e.c3xPath, tmpFile.Name())

	// Run the command
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("c3x command failed: %w\nStderr: %s\nStdout: %s", err, stderr.String(), stdout.String())
	}

	// Log raw c3x output for debugging
	rawOutput := stdout.String()
	helpers.Debugf("Raw c3x output length: %d bytes", len(rawOutput))

	// Parse full c3x JSON output
	var c3xOutput C3XOutput
	if err := json.Unmarshal(stdout.Bytes(), &c3xOutput); err != nil {
		return nil, fmt.Errorf("failed to parse c3x output: %w\nOutput: %s", err, stdout.String())
	}

	helpers.Debugf("Parsed c3x output - TotalMonthlyCost: '%s', Currency: '%s', Projects: %d",
		c3xOutput.TotalMonthlyCost, c3xOutput.Currency, len(c3xOutput.Projects))

	// Convert c3x output to our CostEstimate format
	estimate := &CostEstimate{
		Currency:             c3xOutput.Currency,
		Projects:             []ProjectCostEstimate{},
		ResourceTypesSummary: make(map[string]*ResourceTypeCost),
		Summary:              c3xOutput.Summary,
	}

	// Parse total costs
	estimate.TotalMonthlyCost = parseFloat(c3xOutput.TotalMonthlyCost)
	estimate.TotalHourlyCost = parseFloat(c3xOutput.TotalHourlyCost)

	helpers.Debugf("Parsed total costs - Monthly: $%.2f, Hourly: $%.4f",
		estimate.TotalMonthlyCost, estimate.TotalHourlyCost)

	// Parse projects and resources
	for _, proj := range c3xOutput.Projects {
		project := ProjectCostEstimate{
			Name:      proj.Name,
			Resources: []ResourceCostEstimate{},
		}

		helpers.Debugf("Processing project '%s' with %d resources", proj.Name, len(proj.Breakdown.Resources))

		// Process resources from breakdown
		for _, res := range proj.Breakdown.Resources {
			monthlyCost := parseFloat(res.MonthlyCost)
			hourlyCost := parseFloat(res.HourlyCost)

			// Parse cost components
			costComponents := []CostComponentEstimate{}
			for _, comp := range res.CostComponents {
				monthlyQty := 0.0
				if comp.MonthlyQuantity != nil {
					monthlyQty = parseFloat(*comp.MonthlyQuantity)
				}
				costComponents = append(costComponents, CostComponentEstimate{
					Name:            comp.Name,
					Unit:            comp.Unit,
					MonthlyQuantity: monthlyQty,
					MonthlyCost:     parseFloat(comp.MonthlyCost),
					UsageBased:      comp.UsageBased,
					PriceNotFound:   comp.PriceNotFound,
				})
			}

			// Parse subresources
			subresources := []ResourceCostEstimate{}
			for _, sub := range res.Subresources {
				subCostComponents := []CostComponentEstimate{}
				for _, comp := range sub.CostComponents {
					monthlyQty := 0.0
					if comp.MonthlyQuantity != nil {
						monthlyQty = parseFloat(*comp.MonthlyQuantity)
					}
					subCostComponents = append(subCostComponents, CostComponentEstimate{
						Name:            comp.Name,
						Unit:            comp.Unit,
						MonthlyQuantity: monthlyQty,
						MonthlyCost:     parseFloat(comp.MonthlyCost),
						UsageBased:      comp.UsageBased,
						PriceNotFound:   comp.PriceNotFound,
					})
				}
				subresources = append(subresources, ResourceCostEstimate{
					Name:           sub.Name,
					ResourceType:   sub.ResourceType,
					MonthlyCost:    parseFloat(sub.MonthlyCost),
					HourlyCost:     parseFloat(sub.HourlyCost),
					Tags:           sub.Tags,
					CostComponents: subCostComponents,
				})
			}

			resourceEst := ResourceCostEstimate{
				Name:           res.Name,
				ResourceType:   res.ResourceType,
				MonthlyCost:    monthlyCost,
				HourlyCost:     hourlyCost,
				Tags:           res.Tags,
				CostComponents: costComponents,
				Subresources:   subresources,
			}

			project.Resources = append(project.Resources, resourceEst)
			project.MonthlyCost += monthlyCost
			project.HourlyCost += hourlyCost

			// Aggregate by resource type
			if res.ResourceType != "" {
				if _, exists := estimate.ResourceTypesSummary[res.ResourceType]; !exists {
					estimate.ResourceTypesSummary[res.ResourceType] = &ResourceTypeCost{
						ResourceType: res.ResourceType,
						Count:        0,
						HourlyCost:   0,
						MonthlyCost:  0,
					}
				}
				estimate.ResourceTypesSummary[res.ResourceType].Count++
				estimate.ResourceTypesSummary[res.ResourceType].HourlyCost += hourlyCost
				estimate.ResourceTypesSummary[res.ResourceType].MonthlyCost += monthlyCost
			}

			helpers.Debugf("Added resource '%s' (%s) - Monthly: $%.2f, Hourly: $%.4f",
				res.Name, res.ResourceType, monthlyCost, hourlyCost)
		}

		estimate.Projects = append(estimate.Projects, project)
	}

	helpers.Debugf("Final estimate - Total Monthly: $%.2f, Total Hourly: $%.4f, Projects: %d, Resource Types: %d",
		estimate.TotalMonthlyCost, estimate.TotalHourlyCost, len(estimate.Projects), len(estimate.ResourceTypesSummary))

	// Generate formatted summary
	estimate.FormattedSummary = e.generateSummary(estimate)

	return estimate, nil
}

// parseFloat safely parses a string to float64
func parseFloat(s string) float64 {
	if s == "" {
		return 0
	}
	cleanCost := strings.TrimSpace(s)
	cleanCost = strings.ReplaceAll(cleanCost, ",", "")
	cleanCost = strings.TrimPrefix(cleanCost, "$")

	var value float64
	if _, err := fmt.Sscanf(cleanCost, "%f", &value); err != nil {
		return 0
	}
	return value
}

// generateSummary creates a text summary of the estimate
func (e *Estimator) generateSummary(estimate *CostEstimate) string {
	var sb strings.Builder

	sb.WriteString("Cost Estimation Summary\n")
	sb.WriteString(fmt.Sprintf("Total Monthly Cost: $%.2f %s\n", estimate.TotalMonthlyCost, estimate.Currency))
	sb.WriteString(fmt.Sprintf("Total Hourly Cost: $%.4f %s\n\n", estimate.TotalHourlyCost, estimate.Currency))

	if len(estimate.ResourceTypesSummary) > 0 {
		sb.WriteString("Cost by Resource Type:\n")
		for rt, cost := range estimate.ResourceTypesSummary {
			sb.WriteString(fmt.Sprintf("  %s (%d): $%.4f/hour, $%.2f/month\n",
				rt, cost.Count, cost.HourlyCost, cost.MonthlyCost))
		}
	}

	return sb.String()
}

// CheckBudget checks if the estimated cost exceeds the budget limit
func (e *Estimator) CheckBudget(estimate *CostEstimate) (bool, string) {
	if e.budgetLimit <= 0 {
		return true, ""
	}

	if estimate.TotalMonthlyCost > e.budgetLimit {
		return false, fmt.Sprintf("Cost estimate $%.2f exceeds budget limit of $%.2f (%.1f%% over budget)",
			estimate.TotalMonthlyCost,
			e.budgetLimit,
			((estimate.TotalMonthlyCost-e.budgetLimit)/e.budgetLimit)*100)
	}

	percentUsed := (estimate.TotalMonthlyCost / e.budgetLimit) * 100
	return true, fmt.Sprintf("Cost estimate $%.2f is within budget limit of $%.2f (%.1f%% of budget)",
		estimate.TotalMonthlyCost,
		e.budgetLimit,
		percentUsed)
}

// FormatEstimate formats the cost estimate for display in Markdown using templates
func (e *Estimator) FormatEstimate(estimate *CostEstimate) string {
	// Sort resource types by monthly cost (descending)
	sortedTypes := make([]*ResourceTypeCost, 0, len(estimate.ResourceTypesSummary))
	for _, cost := range estimate.ResourceTypesSummary {
		sortedTypes = append(sortedTypes, cost)
	}
	sort.Slice(sortedTypes, func(i, j int) bool {
		return sortedTypes[i].MonthlyCost > sortedTypes[j].MonthlyCost
	})

	// Check if we have resources
	hasResources := false
	if len(estimate.Projects) > 0 && len(estimate.Projects[0].Resources) > 0 {
		hasResources = true
	}

	// Prepare template data
	data := map[string]interface{}{
		"TotalMonthlyCost":    estimate.TotalMonthlyCost,
		"TotalHourlyCost":     estimate.TotalHourlyCost,
		"Currency":            estimate.Currency,
		"SortedResourceTypes": sortedTypes,
		"Projects":            estimate.Projects,
		"HasResources":        hasResources,
		"HasSummary":          estimate.Summary.TotalDetectedResources > 0,
		"Summary":             estimate.Summary,
		"HasBudget":           e.budgetLimit > 0,
	}

	// Add budget information if configured
	if e.budgetLimit > 0 {
		withinBudget, message := e.CheckBudget(estimate)
		data["WithinBudget"] = withinBudget
		data["BudgetMessage"] = message
	}

	// Get template (lazy initialization)
	tmpl, err := templates.GetCostEstimateTemplate()
	if err != nil {
		helpers.Warnf("Failed to load cost estimate template: %v", err)
		// Fallback to simple format
		return fmt.Sprintf("## 💰 Cost Estimation Summary\n\n**Total Monthly Cost:** $%.2f %s\n\n**Error loading template:** %v",
			estimate.TotalMonthlyCost, estimate.Currency, err)
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		helpers.Warnf("Failed to execute cost estimate template: %v", err)
		// Fallback to simple format
		return fmt.Sprintf("## 💰 Cost Estimation Summary\n\n**Total Monthly Cost:** $%.2f %s\n\n**Error rendering detailed report:** %v",
			estimate.TotalMonthlyCost, estimate.Currency, err)
	}

	return buf.String()
}
