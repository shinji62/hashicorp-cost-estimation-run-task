package agents

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-run-task-scaffolding-go/prompts"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model/gemini"
)

// NewPricingAgent creates a new pricing optimization agent
func NewPricingAgent(ctx context.Context, modelName string) agent.Agent {
	model, err := gemini.NewModel(ctx, modelName, nil)
	if err != nil {
		log.Fatalf("Failed to create model for pricing agent: %v", err)
	}

	pricingAgent, err := llmagent.New(llmagent.Config{
		Name:            "pricing_optimizer",
		Model:           model,
		Description:     "Provides cost optimization recommendations for cloud infrastructure",
		Instruction:     prompts.GetPricingSystemPrompt(),
		IncludeContents: "none",
		OutputKey:       "pricing_analysis_output",
	})
	if err != nil {
		log.Fatalf("Failed to create pricing agent: %v", err)
	}

	return pricingAgent
}
