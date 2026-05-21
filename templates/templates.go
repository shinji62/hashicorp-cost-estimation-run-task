package templates

import (
	"embed"
	"strings"
	"sync"
	"text/template"
)

//go:embed *.tpl
var templateFS embed.FS

var (
	agentAnalysisTemplate     *template.Template
	agentAnalysisTemplateOnce sync.Once
	agentAnalysisTemplateErr  error

	costEstimateTemplate     *template.Template
	costEstimateTemplateOnce sync.Once
	costEstimateTemplateErr  error

	agentInputTemplate     *template.Template
	agentInputTemplateOnce sync.Once
	agentInputTemplateErr  error
)

// GetAgentAnalysisTemplate lazily initializes and returns the agent analysis template with custom functions
func GetAgentAnalysisTemplate() (*template.Template, error) {
	agentAnalysisTemplateOnce.Do(func() {
		// Create template with custom functions
		funcMap := template.FuncMap{
			"add":   func(a, b int) int { return a + b },
			"mul":   func(a, b float64) float64 { return a * b },
			"upper": strings.ToUpper,
			"statusEmoji": func(status string) string {
				switch strings.ToLower(status) {
				case "passed":
					return "✅"
				case "warning":
					return "⚠️"
				case "failed", "error":
					return "❌"
				default:
					return "ℹ️"
				}
			},
		}

		agentAnalysisTemplate, agentAnalysisTemplateErr = template.New("agent_analysis.md.tpl").Funcs(funcMap).ParseFS(templateFS, "agent_analysis.md.tpl")
	})
	return agentAnalysisTemplate, agentAnalysisTemplateErr
}

// GetCostEstimateTemplate lazily initializes and returns the cost estimate template
func GetCostEstimateTemplate() (*template.Template, error) {
	costEstimateTemplateOnce.Do(func() {
		costEstimateTemplate, costEstimateTemplateErr = template.ParseFS(templateFS, "cost_estimate.md.tpl")
	})
	return costEstimateTemplate, costEstimateTemplateErr
}

// GetAgentInputTemplate lazily initializes and returns the agent input template
func GetAgentInputTemplate() (*template.Template, error) {
	agentInputTemplateOnce.Do(func() {
		agentInputTemplate, agentInputTemplateErr = template.ParseFS(templateFS, "agent_input.md.tpl")
	})
	return agentInputTemplate, agentInputTemplateErr
}
