package service

import (
	_ "embed"
	"text/template"
)

// 提示词模板均独立维护在 prompts/ 目录下的 markdown 文件中，
// 支持以 Go text/template 占位符（如 {{.Field}}）留缺口，组装时填充。

// outlineSystemPromptSrc 大纲生成的 system 提示词（核心任务 / 输出语言 / 课程设计原则 / 输出格式）源码，
// 以 {{.SectionCount}} 注入环节数量上限。
//
//go:embed prompts/outline.md
var outlineSystemPromptSrc string

// outlineSystemTpl 解析后的 system 提示词模板。template.Must 在启动期校验语法。
var outlineSystemTpl = template.Must(template.New("outline").Parse(outlineSystemPromptSrc))

// outlineUserPromptTpl 大纲生成的 user 提示词模板源码。
//
//go:embed prompts/outline_user.md
var outlineUserPromptTpl string

// outlineUserTpl 解析后的 user 提示词模板。template.Must 在启动期校验语法。
var outlineUserTpl = template.Must(template.New("outline_user").Parse(outlineUserPromptTpl))

// outlineRegenerateUserPromptTpl 大纲重新生成的 user 提示词模板源码（含已有大纲 + 修改意见）。
//
//go:embed prompts/outline_regenerate_user.md
var outlineRegenerateUserPromptTpl string

// outlineRegenerateUserTpl 解析后的重新生成 user 模板。
var outlineRegenerateUserTpl = template.Must(template.New("outline_regenerate_user").Parse(outlineRegenerateUserPromptTpl))

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

// ---- Quiz 环节生成（单阶段） ----

// quizSystemPrompt Quiz 的 system 提示词。
//
//go:embed prompts/quiz.md
var quizSystemPrompt string

// quizUserPromptTpl Quiz user 提示词模板源码。
//
//go:embed prompts/quiz_user.md
var quizUserPromptTpl string

// quizUserTpl 解析后的 Quiz user 模板。
var quizUserTpl = template.Must(template.New("quiz_user").Parse(quizUserPromptTpl))

// ---- Demo Basic 环节生成（单阶段，三段式 style/body/script，后端拼接骨架） ----

// demoBasicSystemPrompt Demo Basic 的 system 提示词。
//
//go:embed prompts/demo_basic.md
var demoBasicSystemPrompt string

// demoBasicUserPromptTpl Demo Basic user 提示词模板源码。
//
//go:embed prompts/demo_basic_user.md
var demoBasicUserPromptTpl string

// demoBasicUserTpl 解析后的 Demo Basic user 模板。
var demoBasicUserTpl = template.Must(template.New("demo_basic_user").Parse(demoBasicUserPromptTpl))

// demoBasicTemplate demo_basic 的 HTML 骨架（占位符 __STYLE__ / __BODY__ / __SCRIPT__），
// 生成时用大模型产出的三段逻辑替换后得到完整可运行页面。
//
//go:embed templates/demo_basic.html
var demoBasicTemplate string
