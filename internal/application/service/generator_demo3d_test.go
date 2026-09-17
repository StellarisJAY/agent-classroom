package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/StellarisJAY/agent-classroom/internal/types"
)

const testDemo3DJSON = `{
  "scene": {"background": "#0f172a", "axesHelper": true},
  "geometries": [
    {"id": "g1", "type": "Sphere", "args": [1, 32, 16], "position": [0,0,0], "materialId": "m1"},
    {"id": "g2", "type": "SomeUnknownType", "materialId": "missing"}
  ],
  "materials": [{"id": "m1", "type": "MeshStandardMaterial", "color": "#f43f5e"}],
  "lights": [{"type": "AmbientLight", "intensity": 0.5}],
  "cameras": [{"type": "PerspectiveCamera", "position": [0,2,8], "fov": 45, "lookAt": [0,0,0]}],
  "controls": [{"type": "orbit", "title": "环绕视角"}, {"type": "rotation", "title": "核心自转", "targetId": "g1", "axis": "y"}]
}`

// 正常生成：场景 JSON 直接写入 content，未知类型元素按契约保留由前端渲染期剔除。
func TestDemo3DGenerateStoresScene(t *testing.T) {
	g := &demo3DGenerator{}
	sec := genSection("水分子模型", []string{"共价键"})
	sec.Type = types.SectionTypeDemo3D
	client := &seqLLM{contents: []string{testDemo3DJSON}}
	require.NoError(t, g.Generate(context.Background(), sec, genCtxWithLLM(client)))

	var content demo3DContent
	require.NoError(t, json.Unmarshal(sec.Content, &content))
	require.Equal(t, 2, len(content.Geometries))
	require.Equal(t, "Sphere", content.Geometries[0].Type)
	require.Equal(t, "m1", content.Geometries[0].MaterialID)
	require.Equal(t, 1, len(content.Cameras))
	require.Equal(t, 2, len(content.Controls))
	require.Equal(t, "环绕视角", content.Controls[0].Title)
	require.Equal(t, "核心自转", content.Controls[1].Title)
	require.NotNil(t, content.Scene)
	require.Equal(t, "#0f172a", content.Scene.Background)
}

// 解析失败自动重试，第二次成功。
func TestDemo3DGenerateRetriesOnce(t *testing.T) {
	g := &demo3DGenerator{}
	sec := genSection("x", nil)
	sec.Type = types.SectionTypeDemo3D
	client := &seqLLM{contents: []string{"not json", testDemo3DJSON}}
	require.NoError(t, g.Generate(context.Background(), sec, genCtxWithLLM(client)))

	var content demo3DContent
	require.NoError(t, json.Unmarshal(sec.Content, &content))
	require.Equal(t, 1, len(content.Cameras))
}

// 结构错误（缺场景/超限/相机数量/id 冲突）触发重试；两次都失败则报错。
func TestDemo3DGenerateStructureFailure(t *testing.T) {
	cases := []struct {
		name    string
		payload string
	}{
		{"缺少集合", `{"geometries":[{"id":"g1","type":"Box","args":[1,1,1]}],"materials":[],"cameras":[{"type":"PerspectiveCamera"}],"controls":[]}`},
		{"几何体为空", `{"geometries":[],"materials":[],"lights":[],"cameras":[{"type":"PerspectiveCamera"}],"controls":[]}`},
		{"几何体超限", func() string {
			s := `{"geometries":[`
			for range 21 {
				s += `{"id":"g","type":"Box"},`
			}
			return s[:len(s)-1] + `],"materials":[],"lights":[],"cameras":[{"type":"PerspectiveCamera"}],"controls":[]}`
		}()},
		{"灯光超限", `{"geometries":[{"id":"g1","type":"Box"}],"materials":[],"lights":[{"type":"AmbientLight"},{"type":"AmbientLight"},{"type":"AmbientLight"},{"type":"AmbientLight"}],"cameras":[{"type":"PerspectiveCamera"}],"controls":[]}`},
		{"相机缺失", `{"geometries":[{"id":"g1","type":"Box"}],"materials":[],"lights":[],"cameras":[],"controls":[]}`},
		{"相机重复", `{"geometries":[{"id":"g1","type":"Box"}],"materials":[],"lights":[],"cameras":[{"type":"PerspectiveCamera"},{"type":"PerspectiveCamera"}],"controls":[]}`},
		{"几何体 id 冲突", `{"geometries":[{"id":"g1","type":"Box"},{"id":"g1","type":"Sphere"}],"materials":[],"lights":[],"cameras":[{"type":"PerspectiveCamera"}],"controls":[]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := &demo3DGenerator{}
			sec := genSection("x", nil)
			sec.Type = types.SectionTypeDemo3D
			client := &seqLLM{contents: []string{tc.payload, tc.payload}}
			require.Error(t, g.Generate(context.Background(), sec, genCtxWithLLM(client)))
			require.Empty(t, sec.Content)
		})
	}
}

// 枚举外的元素类型 / 悬空 materialId 不触发重试，一次成功（由前端兜底）。
func TestDemo3DGenerateUnknownTypesNoRetry(t *testing.T) {
	g := &demo3DGenerator{}
	sec := genSection("x", nil)
	sec.Type = types.SectionTypeDemo3D
	client := &seqLLM{contents: []string{testDemo3DJSON}}
	require.NoError(t, g.Generate(context.Background(), sec, genCtxWithLLM(client)))
	require.Equal(t, 0, len(client.contents), "校验失败不应额外消耗 LLM 调用")
}

// scale 兼容数字与三轴数组两种形态。
func TestDemo3DGenerateScaleForms(t *testing.T) {
	payload := `{"geometries":[{"id":"g1","type":"Box","scale":2},{"id":"g2","type":"Box","scale":[1,2,3]}],"materials":[],"lights":[],"cameras":[{"type":"PerspectiveCamera"}],"controls":[]}`
	g := &demo3DGenerator{}
	sec := genSection("x", nil)
	sec.Type = types.SectionTypeDemo3D
	require.NoError(t, g.Generate(context.Background(), sec, genCtxWithLLM(&seqLLM{contents: []string{payload}})))

	var content demo3DContent
	require.NoError(t, json.Unmarshal(sec.Content, &content))
	require.Equal(t, any(float64(2)), content.Geometries[0].Scale)
	// any 字段解码后数组形态为 []interface{}(json 数字)，仅需验证逐项相等。
	arr, ok := content.Geometries[1].Scale.([]interface{})
	require.True(t, ok, "scale 三轴数组形态应可解析")
	require.Equal(t, 3, len(arr))
	require.Equal(t, float64(2), arr[1])
}

// 缺少 LLM 客户端直接报错。
func TestDemo3DGenerateRequiresClient(t *testing.T) {
	sec := genSection("x", nil)
	require.Error(t, (&demo3DGenerator{}).Generate(context.Background(), sec, &types.GenerationContext{}))
}
