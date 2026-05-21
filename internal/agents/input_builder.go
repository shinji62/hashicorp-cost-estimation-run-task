package agents

import (
	"bytes"
	"encoding/json"

	tfjson "github.com/hashicorp/terraform-json"
	"github.com/hashicorp/terraform-run-task-scaffolding-go/internal/c3x"
	"github.com/hashicorp/terraform-run-task-scaffolding-go/templates"
)

// buildAnalysisInput creates the input string for agents using a template
func (c *Coordinator) buildAnalysisInput(plan tfjson.Plan, estimate *c3x.CostEstimate, budgetLimit float64) string {
	// Prepare template data
	data := c.prepareAgentInputData(plan, estimate, budgetLimit)

	// Get template
	tmpl, err := templates.GetAgentInputTemplate()
	if err != nil {
		c.logger.Printf("Failed to load agent input template: %v", err)
		return ""
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		c.logger.Printf("Failed to execute agent input template: %v", err)
		return ""
	}

	return buf.String()
}

// prepareAgentInputData prepares the data structure for the agent input template
func (c *Coordinator) prepareAgentInputData(plan tfjson.Plan, estimate *c3x.CostEstimate, budgetLimit float64) AgentInputData {
	// Calculate resource counts once
	creates := countResourceChanges(plan, "create")
	updates := countResourceChanges(plan, "update")
	deletes := countResourceChanges(plan, "delete")

	totalResources := 0
	if plan.PlannedValues != nil && plan.PlannedValues.RootModule != nil {
		totalResources = len(plan.PlannedValues.RootModule.Resources)
	}

	// Prepare budget data
	hasBudget := budgetLimit > 0
	isOverBudget := hasBudget && estimate.TotalMonthlyCost > budgetLimit
	overBudgetAmount := 0.0
	overBudgetPercent := 0.0
	if isOverBudget {
		overBudgetAmount = estimate.TotalMonthlyCost - budgetLimit
		overBudgetPercent = (overBudgetAmount / budgetLimit) * 100
	}

	// Prepare resource changes
	resourceChanges := make([]ResourceChangeData, 0, len(plan.ResourceChanges))
	for _, rc := range plan.ResourceChanges {
		if rc.Change == nil || len(rc.Change.Actions) == 0 {
			continue
		}

		changeData := ResourceChangeData{
			Action:  string(rc.Change.Actions[0]),
			Address: rc.Address,
			Type:    rc.Type,
		}

		// Include resource details/fields
		if rc.Change.After != nil {
			afterJSON, err := json.MarshalIndent(rc.Change.After, "  ", "  ")
			if err == nil {
				jsonStr := string(afterJSON)

				// Scan and redact secrets
				redactedJSON, hasSecrets, scanErr := c.secretsDetector.ScanAndRedactJSON(jsonStr)
				if scanErr != nil {
					c.logger.Printf("Warning: Failed to scan for secrets in resource %s: %v", rc.Address, scanErr)
					changeData.AfterJSON = jsonStr
				} else {
					if hasSecrets {
						c.logger.Printf("Redacted secrets in resource: %s", rc.Address)
					}
					changeData.AfterJSON = redactedJSON
				}
			}
		}

		resourceChanges = append(resourceChanges, changeData)
	}

	// Prepare cost resources
	costResources := make([]CostResourceData, 0)
	if len(estimate.Projects) > 0 && len(estimate.Projects[0].Resources) > 0 {
		for _, resource := range estimate.Projects[0].Resources {
			if resource.MonthlyCost > 0 {
				costResources = append(costResources, CostResourceData{
					Name:         resource.Name,
					ResourceType: resource.ResourceType,
					MonthlyCost:  resource.MonthlyCost,
				})
			}
		}
	}

	return AgentInputData{
		TotalResources:       totalResources,
		Creates:              creates,
		Updates:              updates,
		Deletes:              deletes,
		EstimatedMonthlyCost: estimate.TotalMonthlyCost,
		Currency:             estimate.Currency,
		HasBudget:            hasBudget,
		BudgetLimit:          budgetLimit,
		IsOverBudget:         isOverBudget,
		OverBudgetAmount:     overBudgetAmount,
		OverBudgetPercent:    overBudgetPercent,
		ResourceChanges:      resourceChanges,
		CostResources:        costResources,
	}
}
