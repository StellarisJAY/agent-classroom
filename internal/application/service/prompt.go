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

// ---- Slide 环节生成（两阶段） ----

// slideSystemPrompt Slide 阶段一（视觉内容）的 system 提示词。
//
//go:embed prompts/slide.md
var slideSystemPrompt string

// slideUserPromptTpl Slide 阶段一 user 提示词模板源码。
//
//go:embed prompts/slide_user.md
var slideUserPromptTpl string

// slideUserTpl 解析后的阶段一 user 模板。
var slideUserTpl = template.Must(template.New("slide_user").Parse(slideUserPromptTpl))

// slideStepsSystemPrompt Slide 阶段二（讲解步骤）的 system 提示词。
//
//go:embed prompts/slide_steps.md
var slideStepsSystemPrompt string

// slideStepsUserPromptTpl Slide 阶段二 user 提示词模板源码。
//
//go:embed prompts/slide_steps_user.md
var slideStepsUserPromptTpl string

// slideStepsUserTpl 解析后的阶段二 user 模板。
var slideStepsUserTpl = template.Must(template.New("slide_steps_user").Parse(slideStepsUserPromptTpl))
