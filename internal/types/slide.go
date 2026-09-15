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

	SlideElementMermaid = "mermaid"
	SlideElementChart   = "chart"

	SlideElementFunctionPlot = "functionPlot"
)

// SlideElementChart 的图表类型取值。
const (
	SlideChartBar  = "bar"
	SlideChartLine = "line"
	SlideChartPie  = "pie"
)

// Slide 动作类型。
const (
	SlideActionUnderline = "underline"
	SlideActionHighlight = "highlight"
	SlideActionBox       = "box"

	// 白板/遮罩类动作。视图（slide | overlay | board）由用户手动控制，模型不产出视图切换。
	// laser：激光指示（瞬时，仅当前步骤显示，targetElementId 与 x/y 至少其一）。
	SlideActionLaser = "laser"
	// draw：白板绘画，笔画数据在 Drawing 字段；笔画跨环节累积。
	SlideActionDraw = "draw"
	// clearBoard：清空全局笔画（由模型自主决定擦除时机）。
	SlideActionClearBoard = "clearBoard"
)

// draw 动作的笔画类型与粗细取值。
const (
	SlideDrawPen    = "pen"
	SlideDrawLine   = "line"
	SlideDrawArrow  = "arrow"
	SlideDrawRect   = "rect"
	SlideDrawCircle = "circle"
	SlideDrawText   = "text"
)

const (
	SlideDrawSizeThin   = "thin"
	SlideDrawSizeMedium = "medium"
	SlideDrawSizeThick  = "thick"
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

	// text / mermaid 共用：text 为纯文本；mermaid 为流程图源码
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

	// chart 专用：封闭的"宽表"数据结构，前端确定性翻译为 ECharts 配置。
	Chart      string             `json:"chart,omitempty"`
	Title      string             `json:"title,omitempty"`
	Categories []string           `json:"categories,omitempty"`
	Series     []SlideChartSeries `json:"series,omitempty"`

	// functionPlot 专用：模型只产出数学表达式与坐标窗口，
	// 前端确定性翻译为绘图库配置（模型不产出 JS）。
	XRange [2]float64                  `json:"xRange,omitempty"`
	YRange [2]float64                  `json:"yRange,omitempty"`
	Grid   bool                        `json:"grid,omitempty"`
	Curves []SlideFunctionPlotCurve    `json:"curves,omitempty"`
}

// SlideChartSeries chart 元素的一个数据系列。
// pie 固定只取第一个系列：categories 为扇区名，values 为扇区值。
type SlideChartSeries struct {
	Name   string    `json:"name"`
	Values []float64 `json:"values"`
}

// SlideFunctionPlotCurve functionPlot 元素的一条函数曲线。
// Expression 为纯数学表达式（字符与函数名经白名单校验，防脚本注入）；
// Color 缺省由前端按 accent 派生色板按序分配。
type SlideFunctionPlotCurve struct {
	Expression string `json:"expression"`
	Color      string `json:"color,omitempty"`
	Dash       bool   `json:"dash,omitempty"`
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

// SlideDrawing draw 动作的笔画数据。坐标与 slide 画布同系（1280×720 绝对坐标），
// 不同环节间可直接对齐；笔画跨环节累积、由 clearBoard 控制擦除。
type SlideDrawing struct {
	Kind string `json:"kind"`
	// 线条粗细：thin | medium | thick
	Size string `json:"size,omitempty"`
	// 描边/填充色，缺省用画布 accent
	Color string `json:"color,omitempty"`
	// pen / line / arrow：折线点集（pen 至少 2 点，line/arrow 恰好 2 点）
	Points [][2]float64 `json:"points,omitempty"`
	// rect / circle：包围盒
	X      float64 `json:"x,omitempty"`
	Y      float64 `json:"y,omitempty"`
	Width  float64 `json:"width,omitempty"`
	Height float64 `json:"height,omitempty"`
	// text：文字内容与字号
	Content  string `json:"content,omitempty"`
	FontSize int    `json:"fontSize,omitempty"`
}

// SlideAction 步骤动作：通过 targetElementId 引用页面元素；
// 白板类动作（laser / draw）使用各自专有字段。
type SlideAction struct {
	Type            string `json:"type"`
	TargetElementID string `json:"targetElementId,omitempty"`

	// laser：画布坐标（画布系绝对坐标）
	X float64 `json:"x,omitempty"`
	Y float64 `json:"y,omitempty"`
	// draw：笔画数据
	Drawing *SlideDrawing `json:"drawing,omitempty"`
}
