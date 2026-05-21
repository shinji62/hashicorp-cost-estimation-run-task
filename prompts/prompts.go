package prompts

import (
	_ "embed"
)

// Embedded prompt files using Go's embed directive
// These prompts are compiled into the binary at build time

//go:embed security_system.txt
var securitySystemPrompt string

//go:embed pricing_system.txt
var pricingSystemPrompt string

// GetSecuritySystemPrompt returns the system prompt for the security agent
func GetSecuritySystemPrompt() string {
	return securitySystemPrompt
}

// GetPricingSystemPrompt returns the system prompt for the pricing agent
func GetPricingSystemPrompt() string {
	return pricingSystemPrompt
}
