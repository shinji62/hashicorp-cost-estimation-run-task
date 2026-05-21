package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	tfjson "github.com/hashicorp/terraform-json"
	"github.com/hashicorp/terraform-run-task-scaffolding-go/internal/c3x"
	"github.com/hashicorp/terraform-run-task-scaffolding-go/internal/config"
	"github.com/hashicorp/terraform-run-task-scaffolding-go/internal/secrets"
	"github.com/hashicorp/terraform-run-task-scaffolding-go/templates"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/workflowagents/parallelagent"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

var titleCaser = cases.Title(language.English, cases.NoLower)

// AgentResponse represents the response from an agent
type AgentResponse struct {
	AgentType  string    `json:"agent_type"`
	AgentName  string    `json:"agent_name"`
	Status     string    `json:"status"`
	Summary    string    `json:"summary"`
	Findings   []Finding `json:"findings"`
	Confidence float64   `json:"confidence"`
	Error      string    `json:"error,omitempty"`
}

// Finding represents a specific finding from an agent
type Finding struct {
	Severity        string `json:"severity"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	ResourceName    string `json:"resource_name,omitempty"`
	ResourceType    string `json:"resource_type,omitempty"`
	Recommendation  string `json:"recommendation,omitempty"`
	EstimatedImpact string `json:"estimated_impact,omitempty"`
}

// Coordinator manages multiple AI agents
type Coordinator struct {
	securityAgent   agent.Agent
	pricingAgent    agent.Agent
	parallelAgent   agent.Agent
	logger          *log.Logger
	enabled         bool
	secretsDetector *secrets.Detector
}

// ResponseWithGroupedFindings extends AgentResponse with grouped findings for template
type ResponseWithGroupedFindings struct {
	*AgentResponse
	Critical []Finding
	High     []Finding
	Medium   []Finding
	Low      []Finding
}

// ResourceChangeData represents a resource change for template rendering
type ResourceChangeData struct {
	Action    string
	Address   string
	Type      string
	AfterJSON string
}

// CostResourceData represents a cost resource for template rendering
type CostResourceData struct {
	Name         string
	ResourceType string
	MonthlyCost  float64
}

// AgentInputData represents the data for agent input template
type AgentInputData struct {
	TotalResources       int
	Creates              int
	Updates              int
	Deletes              int
	EstimatedMonthlyCost float64
	Currency             string
	HasBudget            bool
	BudgetLimit          float64
	IsOverBudget         bool
	OverBudgetAmount     float64
	OverBudgetPercent    float64
	ResourceChanges      []ResourceChangeData
	CostResources        []CostResourceData
}

// toTitle converts the first character of a string to uppercase using proper Unicode title casing
func toTitle(s string) string {
	return titleCaser.String(s)
}

// NewCoordinator creates a new agent coordinator
func NewCoordinator(ctx context.Context, logger *log.Logger, cfg *config.Config) (*Coordinator, error) {
	// Initialize secrets detector - pass nil to use the global logger from helpers
	secretsDetector, err := secrets.NewDetector(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize secrets detector: %w", err)
	}

	coordinator := &Coordinator{
		logger:          logger,
		enabled:         false,
		secretsDetector: secretsDetector,
	}

	// Get configuration values
	var apiKey, modelName string
	var securityEnabled, pricingEnabled bool

	if cfg != nil {
		apiKey = cfg.GetAPIKey()
		modelName = cfg.GeminiModel
		securityEnabled = cfg.AgentSecurityEnabled
		pricingEnabled = cfg.AgentPricingEnabled
	} else {
		// Fallback to environment variables for backward compatibility
		apiKey = os.Getenv("GOOGLE_API_KEY")
		if apiKey == "" {
			apiKey = os.Getenv("GEMINI_API_KEY")
		}
		modelName = os.Getenv("GEMINI_MODEL")
		if modelName == "" {
			modelName = "gemini-3.1-flash-lite"
		}
		securityEnabled = os.Getenv("AGENT_SECURITY_ENABLED") == "true"
		pricingEnabled = os.Getenv("AGENT_PRICING_ENABLED") == "true"
	}

	// Check if agents are enabled
	if apiKey == "" {
		logger.Println("No GOOGLE_API_KEY or GEMINI_API_KEY found. AI agents will not be available.")
		return coordinator, nil
	}

	logger.Printf("Using Gemini model: %s", modelName)

	subAgents := []agent.Agent{}

	// Initialize security agent if enabled
	if securityEnabled {
		coordinator.securityAgent = NewSecurityAgent(ctx, modelName)
		subAgents = append(subAgents, coordinator.securityAgent)
		coordinator.enabled = true
		logger.Println("Loaded Security Risk Analyzer agent")
	}

	// Initialize pricing agent if enabled
	if pricingEnabled {
		coordinator.pricingAgent = NewPricingAgent(ctx, modelName)
		subAgents = append(subAgents, coordinator.pricingAgent)
		coordinator.enabled = true
		logger.Println("Loaded Pricing Optimization Advisor agent")
	}

	if !coordinator.enabled {
		logger.Println("No agents configured. Set AGENT_*_ENABLED=true to enable agents.")
		return coordinator, nil
	}

	// Create a parallel agent to run all agents concurrently
	if len(subAgents) > 0 {
		parallelAgent, err := parallelagent.New(parallelagent.Config{
			AgentConfig: agent.Config{
				Name:        "terraform_analysis_coordinator",
				Description: "Coordinates parallel analysis of Terraform infrastructure by security and pricing agents",
				SubAgents:   subAgents,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create parallel agent: %w", err)
		}
		coordinator.parallelAgent = parallelAgent
		logger.Printf("Created parallel agent coordinator with %d sub-agents", len(subAgents))
	}

	coordinator.logger.Println("Agent coordinator initialized successfully")

	return coordinator, nil
}

// AnalyzeAll runs all configured agents in parallel
func (c *Coordinator) AnalyzeAll(ctx context.Context, plan tfjson.Plan, estimate *c3x.CostEstimate, budgetLimit float64, runID string) []*AgentResponse {
	if !c.enabled || c.parallelAgent == nil {
		return nil
	}

	c.logger.Printf("Starting parallel AI agent analysis...")

	// Prepare input for agents
	input := c.buildAnalysisInput(plan, estimate, budgetLimit)

	// Create a session service
	sessionService := session.InMemoryService()
	resp, err := sessionService.Create(ctx, &session.CreateRequest{
		AppName:   "terraform-cost-estimation",
		SessionID: runID,
		UserID:    "terraform-cost-estimation",
	})

	if err != nil {
		c.logger.Printf("Failed to create session: %v", err)
		return []*AgentResponse{{
			AgentType: "coordinator",
			AgentName: "Agent Coordinator",
			Status:    "error",
			Summary:   fmt.Sprintf("Failed to create session: %v", err),
			Error:     err.Error(),
		}}
	}

	// Create a runner for the parallel agent (which coordinates all sub-agents)
	r, err := runner.New(runner.Config{
		AppName:           "terraform-cost-estimation",
		Agent:             c.parallelAgent,
		SessionService:    sessionService,
		AutoCreateSession: true,
	})

	if err != nil {
		c.logger.Printf("Failed to create runner: %v", err)
		return []*AgentResponse{{
			AgentType: "coordinator",
			AgentName: "Agent Coordinator",
			Status:    "error",
			Summary:   fmt.Sprintf("Failed to create runner: %v", err),
			Error:     err.Error(),
		}}
	}

	// Create user message
	userMsg := &genai.Content{
		Role: "user",
		Parts: []*genai.Part{
			{Text: input},
		},
	}

	// Run the parallel agent and collect events from all sub-agents
	responses := make([]*AgentResponse, 0)
	var mu sync.Mutex
	agentResults := make(map[string]strings.Builder)

	for event, err := range r.Run(ctx,
		"terraform-user",
		resp.Session.ID(),
		userMsg,
		agent.RunConfig{}) {
		if err != nil {
			c.logger.Printf("Agent error: %v", err)
			return []*AgentResponse{{
				AgentType: "coordinator",
				AgentName: "Agent Coordinator",
				Status:    "error",
				Summary:   fmt.Sprintf("Analysis failed: %v", err),
				Error:     err.Error(),
			}}
		}

		// Collect text from each agent's response
		if event.Content != nil && len(event.Content.Parts) > 0 {
			// Determine which agent this event is from using the Author field
			agentName := event.Author
			if agentName == "" {
				agentName = "unknown"
			}

			mu.Lock()
			builder := agentResults[agentName]
			for _, part := range event.Content.Parts {
				builder.WriteString(part.Text)
			}
			agentResults[agentName] = builder
			mu.Unlock()
		}
	}

	// Parse results from each agent
	for agentName, result := range agentResults {
		resultText := result.String()
		if resultText == "" {
			continue
		}

		// Determine agent type from name
		agentType := strings.ToLower(strings.ReplaceAll(agentName, "_", ""))
		if strings.Contains(agentType, "security") {
			agentType = "security"
		} else if strings.Contains(agentType, "pricing") {
			agentType = "pricing"
		}

		// Parse the agent response
		response, err := c.parseAgentResponse(agentType, resultText)
		if err != nil {
			c.logger.Printf("Failed to parse %s agent response: %v", agentType, err)
			responses = append(responses, &AgentResponse{
				AgentType: agentType,
				AgentName: agentName,
				Status:    "error",
				Summary:   fmt.Sprintf("Failed to parse response: %v", err),
				Error:     err.Error(),
			})
			continue
		}

		c.logger.Printf("%s agent completed: %d findings", agentType, len(response.Findings))
		responses = append(responses, response)
	}

	if len(responses) == 0 {
		return []*AgentResponse{{
			AgentType: "coordinator",
			AgentName: "Agent Coordinator",
			Status:    "error",
			Summary:   "No responses from any agent",
			Error:     "empty response",
		}}
	}

	c.logger.Printf("Completed AI agent analysis: %d response(s)", len(responses))

	return responses
}

// parseAgentResponse parses the agent's JSON response
func (c *Coordinator) parseAgentResponse(agentType string, result string) (*AgentResponse, error) {
	// Extract JSON from the response
	jsonText := extractJSON(result)

	var response AgentResponse
	if err := json.Unmarshal([]byte(jsonText), &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Set agent metadata
	response.AgentType = agentType
	if response.AgentName == "" {
		response.AgentName = fmt.Sprintf("%s Agent", toTitle(agentType))
	}

	return &response, nil
}

// HasAgents returns true if any agents are configured
func (c *Coordinator) HasAgents() bool {
	return c.enabled
}

// FormatResponses formats agent responses into markdown using templates
func (c *Coordinator) FormatResponses(responses []*AgentResponse) string {
	if len(responses) == 0 {
		return ""
	}

	// Prepare responses with grouped findings
	groupedResponses := make([]*ResponseWithGroupedFindings, len(responses))
	for i, resp := range responses {
		critical, high, medium, low := groupFindingsBySeverity(resp.Findings)
		groupedResponses[i] = &ResponseWithGroupedFindings{
			AgentResponse: resp,
			Critical:      critical,
			High:          high,
			Medium:        medium,
			Low:           low,
		}
	}

	// Prepare template data
	data := map[string]interface{}{
		"AgentCount": len(responses),
		"Responses":  groupedResponses,
	}

	// Get template (lazy initialization)
	tmpl, err := templates.GetAgentAnalysisTemplate()
	if err != nil {
		c.logger.Printf("Failed to load agent analysis template: %v", err)
		// Fallback to simple format
		return fmt.Sprintf("## 🤖 AI Agent Analysis\n\n*Analysis performed by %d AI agent(s)*\n\n**Error loading template:** %v",
			len(responses), err)
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		c.logger.Printf("Failed to execute agent analysis template: %v", err)
		// Fallback to simple format
		return fmt.Sprintf("## 🤖 AI Agent Analysis\n\n*Analysis performed by %d AI agent(s)*\n\n**Error rendering detailed report:** %v",
			len(responses), err)
	}

	return buf.String()
}

// Helper functions

func countResourceChanges(plan tfjson.Plan, action string) int {
	count := 0
	for _, rc := range plan.ResourceChanges {
		if rc.Change != nil {
			for _, a := range rc.Change.Actions {
				if string(a) == action {
					count++
					break
				}
			}
		}
	}
	return count
}

func extractJSON(text string) string {
	start, end := -1, -1
	braceCount := 0

	for i, ch := range text {
		if ch == '{' {
			if start == -1 {
				start = i
			}
			braceCount++
		} else if ch == '}' {
			braceCount--
			if braceCount == 0 && start != -1 {
				end = i + 1
				break
			}
		}
	}

	if start >= 0 && end > start {
		return text[start:end]
	}
	return text
}

func groupFindingsBySeverity(findings []Finding) (critical, high, medium, low []Finding) {
	for _, f := range findings {
		switch strings.ToLower(f.Severity) {
		case "critical":
			critical = append(critical, f)
		case "high":
			high = append(high, f)
		case "medium":
			medium = append(medium, f)
		default:
			low = append(low, f)
		}
	}
	return
}
