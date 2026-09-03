package service

import (
	_ "embed"
	"text/template"
)

// 提示词模板均独立维护在 prompts/ 目录下的 markdown 文件中，
// 支持以 Go text/template 占位符（如 {{.Field}}）留缺口，组装时填充。

// outlineSystemPrompt 大纲生成的 system 提示词（核心任务 / 输出语言 / 课程设计原则 / 输出格式）。
//
//go:embed prompts/outline.md
var outlineSystemPrompt string

// outlineUserPromptTpl 大纲生成的 user 提示词模板源码。
//
//go:embed prompts/outline_user.md
var outlineUserPromptTpl string

// outlineUserTpl 解析后的 user 提示词模板。template.Must 在启动期校验语法。
var outlineUserTpl = template.Must(template.New("outline_user").Parse(outlineUserPromptTpl))
