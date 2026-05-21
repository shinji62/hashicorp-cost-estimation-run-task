## 💰 Cost Estimation Summary

**Total Monthly Cost:** `${{printf "%.2f" .TotalMonthlyCost}} {{.Currency}}`
**Total Hourly Cost:** `${{printf "%.4f" .TotalHourlyCost}} {{.Currency}}`


{{if .SortedResourceTypes}}
### 📊 Cost by Resource Type

| Resource Type | Count | Hourly Cost | Monthly Cost |
|---------------|------:|------------:|-------------:|
{{range .SortedResourceTypes}}| `{{.ResourceType}}` | {{.Count}} | ${{printf "%.4f" .HourlyCost}} | ${{printf "%.2f" .MonthlyCost}} |
{{end}}
{{end}}


{{if .HasResources}}
### 📋 Detailed Resource Breakdown

| Resource Name | Type | Hourly | Monthly |
|---------------|------|-------:|--------:|
{{range .Projects -}}
  {{range .Resources -}}
| `{{.Name}}` | `{{.ResourceType}}` | ${{printf "%.4f" .HourlyCost}} | ${{printf "%.2f" .MonthlyCost}} |
  {{end}}
{{end}}

{{else}}
### ℹ️ No Cost Data Available

No resources with cost data found in this plan. This may be because:

- Resources don't have pricing data available yet
- The c3x pricing database is still being populated
- Resources are being destroyed (not created/updated)
{{end}}


{{if .HasSummary}}
### 📈 Summary Statistics

- **Total Resources Detected:** {{.Summary.TotalDetectedResources}}
- **Resources with Cost Data:** {{.Summary.TotalSupportedResources}}
- **Usage-Based Resources:** {{.Summary.TotalUsageBasedResources}}
- **Resources without Pricing:** {{.Summary.TotalNoPriceResources}}
{{end}}


{{if .HasBudget}}
---

{{if .WithinBudget}}✅{{else}}❌{{end}} **Budget Status:** {{.BudgetMessage}}
{{end}}