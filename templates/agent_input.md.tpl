# Terraform Infrastructure Plan Analysis

## Summary

- **Total Resources**: {{.TotalResources}}
- **Creates**: {{.Creates}}
- **Updates**: {{.Updates}}
- **Deletes**: {{.Deletes}}
- **Estimated Monthly Cost**: ${{printf "%.2f" .EstimatedMonthlyCost}} {{.Currency}}
{{- if .HasBudget}}
- **Budget Limit**: ${{printf "%.2f" .BudgetLimit}}
{{- if .IsOverBudget}}
- **⚠️ Over Budget**: ${{printf "%.2f" .OverBudgetAmount}} ({{printf "%.1f" .OverBudgetPercent}}%)
{{- end}}
{{- end}}

## Resource Changes
{{range .ResourceChanges}}
- **[{{.Action}}]** `{{.Address}}` ({{.Type}})
{{- if .AfterJSON}}
  ```json
  {{.AfterJSON}}
  ```
{{- end}}
{{end}}

## Cost Breakdown
{{if .CostResources}}
{{- range .CostResources}}
- `{{.Name}}` ({{.ResourceType}}): **${{printf "%.2f" .MonthlyCost}}/month**
{{end}}
{{- else}}
*No cost data available*
{{- end}}