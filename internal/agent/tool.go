package agent

import "github.com/StellarisJAY/agent-classroom/internal/model"

// FunctionTool 组装一个 OpenAI function 协议的工具定义。
// params 为标准 JSON Schema 的对象体（建议由 ObjectSchema 构造）。
func FunctionTool(name, description string, params map[string]any) Tool {
	return Tool{Definition: model.Tool{
		Type: "function",
		Function: model.ToolFunction{
			Name:        name,
			Description: description,
			Parameters:  params,
		},
	}}
}

// ObjectSchema 构造 JSON Schema 的 object 字段：type/properties/required 三件套。
// required 为空时不下发 required 约束。
func ObjectSchema(props map[string]any, required ...string) map[string]any {
	m := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		m["required"] = required
	}
	return m
}
