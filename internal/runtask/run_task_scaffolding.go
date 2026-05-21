package runtask

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	tfjson "github.com/hashicorp/terraform-json"

	"github.com/hashicorp/terraform-run-task-scaffolding-go/internal/agents"
	"github.com/hashicorp/terraform-run-task-scaffolding-go/internal/c3x"
	"github.com/hashicorp/terraform-run-task-scaffolding-go/internal/config"
	"github.com/hashicorp/terraform-run-task-scaffolding-go/internal/sdk/api"
	"github.com/hashicorp/terraform-run-task-scaffolding-go/internal/sdk/handler"
)

// ScaffoldingRunTask defines the run task implementation.
type ScaffoldingRunTask struct {
	config           handler.Configuration
	logger           *log.Logger
	appConfig        *config.Config
	budgetLimit      float64
	agentCoordinator *agents.Coordinator
}

// SetAppConfig sets the application configuration
func (r *ScaffoldingRunTask) SetAppConfig(cfg *config.Config) {
	r.appConfig = cfg
}

// Configure defines the configuration for the server and run task.
// This method is called before the server is initialized.
func (r *ScaffoldingRunTask) Configure(addr string, path string, hmacKey string) {
	r.config = handler.Configuration{
		Addr:    fmt.Sprintf(":%s", addr),
		Path:    path,
		HmacKey: hmacKey,
	}

	// Load budget limit from app config
	if r.appConfig != nil {
		r.budgetLimit = r.appConfig.C3XBudgetLimit
		if r.budgetLimit > 0 {
			r.logger.Printf("Budget limit set to $%.2f", r.budgetLimit)
		}
		r.logger.Printf("C3X Pricing API endpoint: %s", r.appConfig.C3XPricingAPIEndpoint)
	}

	// Initialize AI agent coordinator with config
	ctx := context.Background()
	coordinator, err := agents.NewCoordinator(ctx, r.logger, r.appConfig)
	if err != nil {
		r.logger.Printf("Warning: Failed to initialize AI agents: %v", err)
	} else {
		r.agentCoordinator = coordinator
		if coordinator.HasAgents() {
			r.logger.Println("AI agents initialized successfully")
		}
	}
}

// VerifyRequest defines custom run task integration logic.
// This method is called after the run task receives and validates the run task request from TFC.
func (r *ScaffoldingRunTask) VerifyRequest(request api.Request) (*handler.CallbackBuilder, error) {

	// Run custom verification logic
	r.logger.Println("Successfully verified request")
	return handler.NewCallbackBuilder(api.TaskPassed).WithMessage("Custom Passed Message"), nil
}

// VerifyPlan defines custom integration logic for verifying the run's plan from TFC.
// This method is only called if the run task is running in the post-plan or pre-apply stages
// and if VerifyRequest returns a nil response with no error.
func (r *ScaffoldingRunTask) VerifyPlan(request api.Request, plan tfjson.Plan) (*handler.CallbackBuilder, error) {
	r.logger.Println("Starting cost estimation with c3x pricing API...")

	// Extract budget_limit from plan variables if present
	budgetLimit := r.budgetLimit // Start with environment variable value
	if plan.Variables != nil {
		if budgetVar, exists := plan.Variables["budget_limit"]; exists && budgetVar != nil {
			if budgetValue, ok := budgetVar.Value.(float64); ok {
				budgetLimit = budgetValue
				r.logger.Printf("Using budget_limit from plan variable: $%.2f", budgetLimit)
			} else if budgetValue, ok := budgetVar.Value.(string); ok {
				if parsed, err := strconv.ParseFloat(budgetValue, 64); err == nil {
					budgetLimit = parsed
					r.logger.Printf("Using budget_limit from plan variable (parsed from string): $%.2f", budgetLimit)
				}
			}
		}
	}

	// Create c3x estimator with the budget limit and config
	estimator, err := c3x.NewEstimator("", budgetLimit, r.appConfig)
	if err != nil {
		r.logger.Printf("Error creating estimator: %v", err)
		return handler.NewCallbackBuilder(api.TaskFailed).
			WithMessage(fmt.Sprintf("❌ Cost estimation failed: %v", err)), nil
	}

	// Estimate costs from the plan
	estimate, err := estimator.EstimateFromPlan(plan)
	if err != nil {
		r.logger.Printf("Error estimating costs: %v", err)
		return handler.NewCallbackBuilder(api.TaskFailed).
			WithMessage(fmt.Sprintf("❌ Cost estimation failed: %v", err)), nil
	}

	// Format the estimate
	formattedEstimate := estimator.FormatEstimate(estimate)
	r.logger.Printf("Cost estimate: $%.2f/month", estimate.TotalMonthlyCost)

	// Log the full formatted estimate to console
	r.logger.Println("Full cost breakdown:")
	r.logger.Println(formattedEstimate)

	// Check budget if configured
	var status api.TaskStatus
	var summaryMessage string

	// Always create the detailed cost breakdown outcome
	detailedOutcome := api.Outcome{
		OutcomeID:   "cost-estimation-details",
		Description: fmt.Sprintf("Detailed cost breakdown for $%.2f/month", estimate.TotalMonthlyCost),
		Body:        formattedEstimate,
	}

	if budgetLimit > 0 {
		withinBudget, message := estimator.CheckBudget(estimate)
		r.logger.Println(message)

		if !withinBudget {
			// Set global status to failed
			status = api.TaskFailed
			summaryMessage = fmt.Sprintf("❌ Cost estimate $%.2f exceeds budget limit of $%.2f", estimate.TotalMonthlyCost, budgetLimit)

			// Create additional outcome for over-budget scenario
			overBudgetAmount := estimate.TotalMonthlyCost - budgetLimit
			overBudgetPercent := (overBudgetAmount / budgetLimit) * 100

			overBudgetOutcome := api.Outcome{
				OutcomeID:   "budget-exceeded",
				Description: fmt.Sprintf("⚠️ Budget Exceeded by $%.2f (%.1f%%)", overBudgetAmount, overBudgetPercent),
				Body: fmt.Sprintf(`## ❌ Budget Limit Exceeded

**Budget Limit:** $%.2f %s
**Estimated Cost:** $%.2f %s
**Over Budget:** $%.2f (%.1f%%)

### Action Required
The estimated monthly cost exceeds your configured budget limit. Please review the cost breakdown and consider:
- Reducing resource sizes or quantities
- Removing unnecessary resources
- Adjusting your budget limit if appropriate

`, budgetLimit, estimate.Currency, estimate.TotalMonthlyCost, estimate.Currency, overBudgetAmount, overBudgetPercent),
			}

			// Build callback with failed status and both outcomes
			return handler.NewCallbackBuilder(status).
				WithMessage(summaryMessage).
				AddOutcome(detailedOutcome).
				AddOutcome(overBudgetOutcome), nil
		} else {
			status = api.TaskPassed
			summaryMessage = fmt.Sprintf("✅ Cost estimate $%.2f is within budget limit of $%.2f", estimate.TotalMonthlyCost, budgetLimit)
		}
	} else {
		status = api.TaskPassed
		summaryMessage = fmt.Sprintf("💰 Estimated monthly cost: $%.2f", estimate.TotalMonthlyCost)
	}

	// Run AI agent analysis if configured
	var agentOutcome *api.Outcome
	if r.agentCoordinator != nil && r.agentCoordinator.HasAgents() {
		r.logger.Println("Running AI agent analysis...")
		ctx := context.Background()
		agentResponses := r.agentCoordinator.AnalyzeAll(ctx, plan, estimate, budgetLimit, request.RunID)

		if len(agentResponses) > 0 {
			agentMarkdown := r.agentCoordinator.FormatResponses(agentResponses)

			// Count critical findings
			criticalCount := 0
			for _, resp := range agentResponses {
				for _, finding := range resp.Findings {
					if finding.Severity == "critical" {
						criticalCount++
					}
				}
			}

			agentOutcome = &api.Outcome{
				OutcomeID:   "ai-agent-analysis",
				Description: fmt.Sprintf("AI Agent Analysis - %d agent(s), %d critical finding(s)", len(agentResponses), criticalCount),
				Body:        agentMarkdown,
			}

			// If there are critical findings, consider failing the task
			if criticalCount > 0 && r.appConfig != nil && r.appConfig.AgentFailOnCritical {
				status = api.TaskFailed
				summaryMessage = fmt.Sprintf("❌ %s - AI agents found %d critical issue(s)", summaryMessage, criticalCount)
			}
		}
	}

	// Build callback response with status, message and outcomes
	builder := handler.NewCallbackBuilder(status).
		WithMessage(summaryMessage).
		AddOutcome(detailedOutcome)

	if agentOutcome != nil {
		builder = builder.AddOutcome(*agentOutcome)
	}

	return builder, nil
}

// createCostOutcomes creates detailed outcomes for each resource in the cost estimate
func (r *ScaffoldingRunTask) createCostOutcomes(estimate *c3x.CostEstimate, estimator *c3x.Estimator, runURL string) []api.Outcome {
	outcomes := []api.Outcome{}

	// Add overall summary outcome
	summaryOutcome := api.Outcome{
		OutcomeID:   "cost-summary",
		Description: fmt.Sprintf("Total Monthly Cost: $%.2f %s", estimate.TotalMonthlyCost, estimate.Currency),
		Body:        estimator.FormatEstimate(estimate),
		URL:         runURL,
	}
	outcomes = append(outcomes, summaryOutcome)

	// Add individual resource outcomes
	for _, project := range estimate.Projects {
		for _, resource := range project.Resources {
			outcome := api.Outcome{
				OutcomeID:   fmt.Sprintf("resource-%s", resource.Name),
				Description: fmt.Sprintf("%s: $%.2f/month", resource.Name, resource.MonthlyCost),
				Body:        fmt.Sprintf("**Resource:** `%s`\n\n**Monthly Cost:** $%.2f", resource.Name, resource.MonthlyCost),
				URL:         runURL,
			}
			outcomes = append(outcomes, outcome)
		}
	}

	return outcomes
}

// NewRunTask instantiates a new ScaffoldingRunTask with a new Logger.
func NewRunTask() *ScaffoldingRunTask {
	return &ScaffoldingRunTask{
		logger: log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime),
	}
}
