# 用户输入

## 当前环节

- 课程标题：{{.CourseTitle}}
- 本环节标题：{{.SectionTitle}}
- 本环节要检验的知识点：{{.KnowledgePoints}}

{{ if .Prompt -}}
## 本环节内容描述（来自大纲，出题须严格遵守题型/题量/考察方式要求）

{{.Prompt}}
{{- end }}

{{ if .PreviousSections -}}
## 课程结构与已讲内容（供出题紧扣前序，避免重复或跳脱）

{{.PreviousSections}}
{{- end }}

{{ if .DocsSummary -}}
## 参考文档（据此组织本环节题目内容）

{{.DocsSummary}}
{{- end }}

{{ if .UserRequest -}}
## 课程内容要求

{{.UserRequest}}
{{- end }}
