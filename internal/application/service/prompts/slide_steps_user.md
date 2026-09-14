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

## 电子白板说明

白板笔画会跨环节累积：前序环节的板书可能仍留在白板上（是否清空由你用 `clearBoard` 决定）。白板遮罩以半透明形式叠在幻灯片上，视图由用户自行切换，你只产出绘画/擦拭/激光动作。可依据课程结构与"前序已讲内容"判断是否先用 `clearBoard` 擦掉无关板书，再安排本环节的绘画动作。
