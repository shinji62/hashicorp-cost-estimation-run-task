## 🤖 AI Agent Analysis

*Analysis performed by {{.AgentCount}} AI agent(s)*

{{range $i, $resp := .Responses}}
### {{add $i 1}}. {{$resp.AgentName}}

**Status:** {{statusEmoji $resp.Status}} {{upper $resp.Status}}

{{if $resp.Summary -}}
**Summary:** {{$resp.Summary}}
{{- end}}

{{if $resp.Error}}
⚠️ **Error:** {{$resp.Error}}

---
{{else}}
  {{- if $resp.Findings}}
**Findings:** {{len $resp.Findings}} issue(s) identified

    {{- if $resp.Critical}}
#### 🔴 Critical Issues
      {{- range $resp.Critical}}
**{{.Title}}**
{{.Description}}

        {{- if .ResourceName}}
- **Resource:** `{{.ResourceName}}`{{if .ResourceType}} ({{.ResourceType}}){{end}}
        {{- end}}
        {{- if .Recommendation}}
- **Recommendation:** {{.Recommendation}}
        {{- end}}
        {{- if .EstimatedImpact}}
- **Impact:** {{.EstimatedImpact}}
        {{- end}}

      {{- end}}
    {{- end}}

    {{- if $resp.High}}
#### 🟠 High Priority
      {{- range $resp.High}}
**{{.Title}}**
{{.Description}}

        {{- if .ResourceName}}
- **Resource:** `{{.ResourceName}}`{{if .ResourceType}} ({{.ResourceType}}){{end}}
        {{- end}}
        {{- if .Recommendation}}
- **Recommendation:** {{.Recommendation}}
        {{- end}}
        {{- if .EstimatedImpact}}
- **Impact:** {{.EstimatedImpact}}
        {{- end}}

      {{- end}}
    {{- end}}

    {{- if $resp.Medium}}
#### 🟡 Medium Priority
      {{- range $resp.Medium}}
**{{.Title}}**
{{.Description}}

        {{- if .ResourceName}}
- **Resource:** `{{.ResourceName}}`{{if .ResourceType}} ({{.ResourceType}}){{end}}
        {{- end}}
        {{- if .Recommendation}}
- **Recommendation:** {{.Recommendation}}
        {{- end}}
        {{- if .EstimatedImpact}}
- **Impact:** {{.EstimatedImpact}}
        {{- end}}

      {{- end}}
    {{- end}}

    {{- if $resp.Low}}
#### 🔵 Low Priority
      {{- range $resp.Low}}
**{{.Title}}**
{{.Description}}

        {{- if .ResourceName}}
- **Resource:** `{{.ResourceName}}`{{if .ResourceType}} ({{.ResourceType}}){{end}}
        {{- end}}
        {{- if .Recommendation}}
- **Recommendation:** {{.Recommendation}}
        {{- end}}
        {{- if .EstimatedImpact}}
- **Impact:** {{.EstimatedImpact}}
        {{- end}}

      {{- end}}
    {{- end}}

  {{- else}}
✅ **No issues found**
  {{- end}}

  {{- if gt $resp.Confidence 0.0}}
**Confidence:** {{printf "%.0f" (mul $resp.Confidence 100)}}%
  {{- end}}

---
{{end}}
{{- end}}