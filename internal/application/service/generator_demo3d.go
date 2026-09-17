package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"gorm.io/datatypes"

	"github.com/StellarisJAY/agent-classroom/internal/model"
	"github.com/StellarisJAY/agent-classroom/internal/types"
	"github.com/StellarisJAY/agent-classroom/internal/util"
)

// Demo 3D 环节单阶段生成：一次 LLM 调用产出纯数据的场景描述 JSON
// （几何体/材质/灯光/相机/控制器五集合），解析与结构校验通过后直接写入 section.content。
// 解析/结构校验失败自动重试（共 2 次尝试）；集合超限属结构错误，同样重试。
// 枚举外的元素类型与悬空 materialId 引用不在后端处理，由前端渲染期兜底（见 docs/3D演示数据结构.md）。

// demo3DVec3 三维向量。
type demo3DVec3 []float64

// demo3DScene 场景全局配置。
type demo3DScene struct {
	Background string `json:"background,omitempty"`
	AxesHelper *bool  `json:"axesHelper,omitempty"`
}

// demo3DGeometry 几何体：内置 primitive 的实例化与初值变换。
// Scale 允许数字（整体倍率）或三轴数组，前端按两者兼容解析。
// ParentID 为场景树父子关系（缺省挂场景根）：不做后端校验，悬空/成环由前端剥环兜底。
type demo3DGeometry struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Args       []float64       `json:"args,omitempty"`
	Position   demo3DVec3      `json:"position,omitempty"`
	Rotation   demo3DVec3      `json:"rotation,omitempty"`
	Scale      any             `json:"scale,omitempty"`
	MaterialID string          `json:"materialId,omitempty"`
	ParentID   string          `json:"parentId,omitempty"`
}

// demo3DMaterial 材质。
type demo3DMaterial struct {
	ID        string   `json:"id"`
	Type      string   `json:"type"`
	Color     string   `json:"color,omitempty"`
	Roughness *float64 `json:"roughness,omitempty"`
	Metalness *float64 `json:"metalness,omitempty"`
	Opacity   *float64 `json:"opacity,omitempty"`
	Emissive  string   `json:"emissive,omitempty"`
}

// demo3DLight 灯光：DirectionalLight / SpotLight 以 position+target 表达朝向。
type demo3DLight struct {
	Type       string     `json:"type"`
	Color      string     `json:"color,omitempty"`
	Intensity  *float64   `json:"intensity,omitempty"`
	Position   demo3DVec3 `json:"position,omitempty"`
	Target     demo3DVec3 `json:"target,omitempty"`
	CastShadow *bool      `json:"castShadow,omitempty"`
}

// demo3DCamera 相机（必须有且仅 1 个）。
type demo3DCamera struct {
	Type     string     `json:"type"`
	Position demo3DVec3 `json:"position,omitempty"`
	Fov      *float64   `json:"fov,omitempty"`
	LookAt   demo3DVec3 `json:"lookAt,omitempty"`
}

// demo3DControl 控制器：LLM 只声明 type/targetId/axis/title，滑块值域由系统固定。
// Title 为前端展示用名称；AutoRotate 为 orbit 的意向声明，当前版本仅存储不实现。
type demo3DControl struct {
	Type       string `json:"type"`
	Title      string `json:"title,omitempty"`
	TargetID   string `json:"targetId,omitempty"`
	Axis       string `json:"axis,omitempty"`
	AutoRotate *bool  `json:"autoRotate,omitempty"`
}

// demo3DContent 落库的 content 结构（与前端 Demo3DContent 契约对齐）。
type demo3DContent struct {
	Scene      *demo3DScene     `json:"scene,omitempty"`
	Geometries []demo3DGeometry `json:"geometries"`
	Materials  []demo3DMaterial `json:"materials"`
	Lights     []demo3DLight    `json:"lights"`
	Cameras    []demo3DCamera   `json:"cameras"`
	Controls   []demo3DControl  `json:"controls"`
}

// 数量上限（与 docs/3D演示数据结构.md 一致）。
const (
	demo3DMaxGeometries = 20
	demo3DMaxLights     = 3
	demo3DMaxMaterials  = 20
)

// demo3DGenerator Demo 3D 环节内容生成器。模型客户端与上下文由 SectionService
// 通过 GenerationContext 注入；无额外持久依赖。
type demo3DGenerator struct{}

// Generate 单次调用 LLM 产出场景描述 JSON，结构校验通过后写入 section.Content。
func (g *demo3DGenerator) Generate(ctx context.Context, section *types.Section, genCtx *types.GenerationContext) error {
	if genCtx == nil || genCtx.Client == nil {
		return errors.New("demo 3d generator: missing llm client")
	}

	msgData := demoBasicUserData{
		CourseTitle:      courseTitle(genCtx),
		SectionTitle:     section.Title,
		KnowledgePoints:  sectionKnowledgePoints(section),
		Prompt:           sectionPrompt(section),
		PreviousSections: slideSectionContext(genCtx, section.Position),
		DocsSummary:      strings.TrimSpace(genCtx.DocsText),
		UserRequest:      genCtxUserRequest(genCtx),
	}
	messages, err := buildPromptMessages(demo3DUserTpl, msgData, demo3DSystemPrompt)
	if err != nil {
		return err
	}

	err = util.Retry(ctx, genCtx.Retry, func() error {
		slog.Debug("generating demo 3d scene")
		resp, cerr := genCtx.Client.Chat(ctx, model.ChatRequest{
			Messages:    messages,
			Temperature: new(0.3),
			Thinking:    genCtx.Thinking,
		})
		if cerr != nil {
			return cerr
		}
		var out demo3DContent
		if xerr := util.ExtractJSON(resp.Content, &out); xerr != nil {
			slog.Warn("invalid demo 3d content", "content", resp.Content)
			return fmt.Errorf("parse demo 3d json: %w", xerr)
		}
		if verr := validDemo3D(&out); verr != nil {
			return verr
		}
		content, merr := json.Marshal(&out)
		if merr != nil {
			return fmt.Errorf("marshal demo 3d content: %w", merr)
		}
		section.Content = datatypes.JSON(content)
		slog.Debug("generate demo 3d scene done")
		return nil
	})
	if err != nil {
		slog.Warn("demo 3d generation failed", "section_id", section.ID.String(), "error", err)
	}
	return err
}

// validDemo3D 场景结构校验：五集合必须齐备、数量在限、id 唯一。
// 元素类型枚举校验交给前端兜底，不在此检查。
func validDemo3D(d *demo3DContent) error {
	if d.Geometries == nil || d.Materials == nil ||
		d.Lights == nil || d.Cameras == nil || d.Controls == nil {
		return errors.New("demo 3d: missing required collections")
	}
	switch {
	case len(d.Geometries) == 0:
		return errors.New("demo 3d: empty geometries")
	case len(d.Geometries) > demo3DMaxGeometries:
		return fmt.Errorf("demo 3d: too many geometries (%d > %d)", len(d.Geometries), demo3DMaxGeometries)
	case len(d.Materials) > demo3DMaxMaterials:
		return fmt.Errorf("demo 3d: too many materials (%d > %d)", len(d.Materials), demo3DMaxMaterials)
	case len(d.Lights) > demo3DMaxLights:
		return fmt.Errorf("demo 3d: too many lights (%d > %d)", len(d.Lights), demo3DMaxLights)
	case len(d.Cameras) != 1:
		return fmt.Errorf("demo 3d: exactly one camera required, got %d", len(d.Cameras))
	}
	geomIDs := make(map[string]struct{}, len(d.Geometries))
	for i := range d.Geometries {
		id := d.Geometries[i].ID
		if id == "" {
			return errors.New("demo 3d: geometry missing id")
		}
		if _, dup := geomIDs[id]; dup {
			return errors.New("demo 3d: duplicate geometry id " + id)
		}
		geomIDs[id] = struct{}{}
	}
	matIDs := make(map[string]struct{}, len(d.Materials))
	for i := range d.Materials {
		id := d.Materials[i].ID
		if id == "" {
			return errors.New("demo 3d: material missing id")
		}
		if _, dup := matIDs[id]; dup {
			return errors.New("demo 3d: duplicate material id " + id)
		}
		matIDs[id] = struct{}{}
	}
	return nil
}
