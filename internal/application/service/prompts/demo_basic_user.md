# 用户输入

## 当前环节

- 课程标题：{{.CourseTitle}}
- 本环节标题：{{.SectionTitle}}
- 本环节要演示的知识点：{{.KnowledgePoints}}

{{ if .Prompt -}}
## 用户对本环节的补充提示

{{.Prompt}}
{{- end }}

{{ if .PreviousSections -}}
## 课程结构与已讲内容（供演示紧扣前序，避免重复或跳脱）

{{.PreviousSections}}
{{- end }}

{{ if .DocsSummary -}}
## 参考文档（据此组织本环节演示内容）

{{.DocsSummary}}
{{- end }}

{{ if .UserRequest -}}
## 课程内容要求

{{.UserRequest}}
{{- end }}
