package agents

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-run-task-scaffolding-go/prompts"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model/gemini"
)

// NewSecurityAgent creates a new security risk analysis agent
func NewSecurityAgent(ctx context.Context, modelName string) agent.Agent {
	model, err := gemini.NewModel(ctx, modelName, nil)
	if err != nil {
		log.Fatalf("Failed to create model for security agent: %v", err)
	}

	securityAgent, err := llmagent.New(llmagent.Config{
		Name:        "security_analyzer",
		Model:       model,
		Description: "Analyzes Terraform infrastructure plans for security risks and vulnerabilities",
		Instruction: prompts.GetSecuritySystemPrompt(),
		OutputKey:   "security_analysis_output",
	})
	if err != nil {
		log.Fatalf("Failed to create security agent: %v", err)
	}

	return securityAgent
}
