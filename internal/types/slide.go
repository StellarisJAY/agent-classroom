package types

// Slide 环节的领域数据结构，对齐 docs/slide数据结构.md v1.0。
// 存储：slide 的视觉内容写入 section.content，讲解步骤写入 section.steps。
// JSON 字段命名严格遵循该文档（顶层字段小写、元素内 fontSize 为 camelCase），
// 以保证与前端/模型产物的结构完全一致。

// Slide 元素类型。
const (
	SlideElementText    = "text"
	SlideElementFormula = "formula"
	SlideElementShape   = "shape"
	SlideElementList    = "list"
	SlideElementImage   = "image"
)

// Slide 动作类型。
const (
	SlideActionUnderline = "underline"
	SlideActionHighlight = "highlight"
	SlideActionBox       = "box"
)

// SlideElementShape 的形状取值。
const (
	SlideShapeRect   = "rect"
	SlideShapeCircle = "circle"
	SlideShapeLine   = "line"
	SlideShapeArrow  = "arrow"
)

// SlideContent 画布属性 + 页面元素（section.content 的可视结构）。
type SlideContent struct {
	Width      int            `json:"width"`
	Height     int            `json:"height"`
	Background string         `json:"background"`
	Accent     string         `json:"accent"`
	Elements   []SlideElement `json:"elements"`
}

// SlideElement 单个页面元素。公共字段 id/x/y；其余按 type 不同取用。
// 未用到的类型专用字段以 omitempty 省略，避免冗余。
type SlideElement struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	X     int    `json:"x"`
	Y     int    `json:"y"`
	Width int    `json:"width,omitempty"`
	// Height 仅供 shape 使用。
	Height int `json:"height,omitempty"`

	// text 专用
	Content string          `json:"content,omitempty"`
	Style   *SlideTextStyle `json:"style,omitempty"`

	// formula / list 字号（camelCase 对齐文档）
	FontSize int `json:"fontSize,omitempty"`

	// shape 专用
	Shape  string `json:"shape,omitempty"`
	Fill   string `json:"fill,omitempty"`
	Stroke string `json:"stroke,omitempty"`
	Label  string `json:"label,omitempty"`

	// list 专用
	Ordered bool     `json:"ordered,omitempty"`
	Items   []string `json:"items,omitempty"`

	// image 专用：Prompt 为文生图提示词（生成前），Src 为生成后的图片 URL（生成后回填）。
	Prompt string `json:"prompt,omitempty"`
	Src    string `json:"src,omitempty"`
}

// SlideTextStyle text 元素的行内样式。
type SlideTextStyle struct {
	FontSize int    `json:"fontSize"`
	Align    string `json:"align,omitempty"` // left | center | right
	Bold     bool   `json:"bold,omitempty"`
	Color    string `json:"color,omitempty"`
}

// SlideStep 一条讲解步骤（section.steps 的组成元素）。
type SlideStep struct {
	Text    string        `json:"text"`
	Actions []SlideAction `json:"actions"`
}

// SlideAction 步骤动作：通过 targetElementId 引用页面元素。
type SlideAction struct {
	Type            string `json:"type"`
	TargetElementID string `json:"targetElementId"`
}
