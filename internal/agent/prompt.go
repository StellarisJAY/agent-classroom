package agent

import (
	_ "embed"
	"strings"
)

// forceFinalSrc 轮次上限强制收尾时注入的 system 提示词。
// 提示词沿项目铁律以 go:embed 独立文件管理：本包暂无可变占位符，
// 后续需要注入（如剩余话题名）时再升级为 text/template 渲染。

//go:embed prompts/force_final.md
var forceFinalSrc string

// forceFinalText 返回强制收尾提示词（原样嵌入文本，去除首尾空白）。
func forceFinalText() string {
	return strings.TrimSpace(forceFinalSrc)
}
