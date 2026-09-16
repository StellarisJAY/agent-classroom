package service

import (
	"github.com/StellarisJAY/agent-classroom/internal/agent"
	"github.com/StellarisJAY/agent-classroom/internal/types"
)

// 讨论模式的工具 schema 定义。与 prompts/discussion.md 的工具说明同源引用，
// 语义以提示词为准；本文件仅声明 OpenAI 协议的 function 定义供流式注入。
// 动作类工具一律由后端合成 "success" 结果（agent.SyntheticResult），
// 前端是动作执行器；参数合法性由提示词约束 + 前端静默降级兜底。

// buildDiscussionTools 按环节类型裁剪工具集。
//   - slide：全部可用
//   - quiz：全部可用（防作弊在上下文装配层硬剔除答案，见 discussionContext）
//   - demo 系：仅 jump_to_section（iframe 沙箱，幻灯片动作无处安放）
func buildDiscussionTools(sectionType string) []agent.Tool {
	tools := []agent.Tool{
		toolJumpToSection,
		toolHighlight,
		toolUnderline,
		toolBox,
		toolDraw,
		toolClearBoard,
		toolLaser,
	}
	if sectionType == types.SectionTypeDemoBasic ||
		sectionType == types.SectionTypeDemo3D ||
		sectionType == types.SectionTypeDemoFunction {
		tools = tools[1:]
	}
	// 动作类工具全部由后端合成固定结果。
	for i := range tools {
		tools[i].Execute = agent.SyntheticResult("success")
	}
	return tools
}

var toolJumpToSection = agent.FunctionTool("jump_to_section",
	"切换到某个已生成的环节进行讲解。",
	agent.ObjectSchema(map[string]any{
		"section_id": map[string]any{
			"type":        "string",
			"description": "目标环节 id（只能引用 [sections] 中给出的环节 uuid）",
		},
		"step_index": map[string]any{
			"type":        "integer",
			"description": "目标步骤下标（从 0 开始）；缺省跳到该环节末步",
		},
	}, "section_id"))

// element 强调类工具共用参数：element_id。
func emphasisTool(name, descr string) agent.Tool {
	return agent.FunctionTool(name, descr, agent.ObjectSchema(map[string]any{
		"element_id": map[string]any{
			"type":        "string",
			"description": "当前环节页面元素的 id（只允许引用 [current_section] 中列出的元素 id）",
		},
	}, "element_id"))
}

var toolHighlight = emphasisTool("highlight", "高亮强调当前环节的指定页面元素。")
var toolUnderline = emphasisTool("underline", "对当前环节的指定页面元素划线强调。")
var toolBox = emphasisTool("box", "框选强调当前环节的指定页面元素。")

var toolDraw = agent.FunctionTool("draw",
	"在白板上作画。坐标与幻灯片画布同系（绝对坐标，默认画布 1280x720）。",
	agent.ObjectSchema(map[string]any{
		"drawing": agent.ObjectSchema(map[string]any{
			"kind": map[string]any{
				"type": "string",
				"enum": []string{types.SlideDrawPen, types.SlideDrawLine, types.SlideDrawArrow, types.SlideDrawRect, types.SlideDrawCircle, types.SlideDrawText},
			},
			"size": map[string]any{
				"type": "string",
				"enum": []string{types.SlideDrawSizeThin, types.SlideDrawSizeMedium, types.SlideDrawSizeThick},
			},
			"color":   map[string]any{"type": "string"},
			"points":  map[string]any{"type": "array", "description": "pen/line/arrow：折线点集 [[x,y],...]，画布系绝对坐标", "items": map[string]any{"type": "array", "items": map[string]any{"type": "number"}}},
			"x":       map[string]any{"type": "number", "description": "rect/circle：包围盒 x"},
			"y":       map[string]any{"type": "number", "description": "rect/circle：包围盒 y"},
			"width":   map[string]any{"type": "number", "description": "rect/circle：包围盒宽"},
			"height":  map[string]any{"type": "number", "description": "rect/circle：包围盒高"},
			"content": map[string]any{"type": "string", "description": "text：文字内容"},
		}, "kind"),
	}, "drawing"))

var toolClearBoard = agent.FunctionTool("clear_board",
	"清空白板上已积累的全部笔画。",
	agent.ObjectSchema(map[string]any{}))

var toolLaser = agent.FunctionTool("laser",
	"瞬时激光指示：引用元素 id 或画布坐标至少其一。",
	agent.ObjectSchema(map[string]any{
		"element_id": map[string]any{
			"type":        "string",
			"description": "要指示的元素 id（与 x/y 至少给出其一）",
		},
		"x": map[string]any{"type": "number", "description": "画布系绝对坐标 x"},
		"y": map[string]any{"type": "number", "description": "画布系绝对坐标 y"},
	}))
