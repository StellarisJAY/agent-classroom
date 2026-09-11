# 用户输入

## 当前环节

- 课程标题：{{.CourseTitle}}
- 本环节标题：{{.SectionTitle}}
- 本环节要讲清的知识点：{{.KnowledgePoints}}

{{ if .Prompt -}}
## 本环节内容描述（来自大纲，生成须严格遵循此描述）

{{.Prompt}}
{{- end }}

{{ if .PreviousSections -}}
## 课程结构与已讲内容（供衔接参考，可让本页自然承前启后）

{{.PreviousSections}}
{{- end }}

{{ if .DocsSummary -}}
## 参考文档（据此组织本环节具体内容与示例）

{{.DocsSummary}}
{{- end }}

{{ if .UserRequest -}}
## 课程内容要求

{{.UserRequest}}
{{- end }}
