# 用户输入

## 用户课程要求

{{.Requirement}}

{{ if .DocsSummary -}}
## 参考文件摘要

{{.DocsSummary}}
{{- end }}

{{ if .ExistingOutline -}}
## 已生成的大纲（当前版本）

{{.ExistingOutline}}

{{- end }}

## 修改意见

{{.Feedback}}

请参考以上已生成的大纲（如有），结合修改意见对大纲进行调整：修改意见未涉及的部分尽量保留原样，仅改动需要调整的环节；如修改意见要求整体重做，则重新构思。最终输出完整的、更新后的大纲 JSON。
