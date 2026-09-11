# 用户输入

## 当前环节

- 课程标题：{{.CourseTitle}}
- 本环节标题：{{.SectionTitle}}
- 本环节要讲清的知识点：{{.KnowledgePoints}}

{{ if .Prompt -}}
## 本环节内容描述（来自大纲，讲解须遵循此描述）

{{.Prompt}}
{{- end }}

{{ if .PreviousSections -}}
## 课程结构与已讲内容（供衔接参考）

{{.PreviousSections}}
{{- end }}

## 页面元素清单（讲解时只允许引用这些 id）

{{.ElementSummary}}
