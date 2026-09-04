package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/StellarisJAY/agent-classroom/internal/types"
)

const testDemoBasicJSON = `{"style":"body{color:#333}","body":"<button id=\"b\">点击</button>","script":"document.getElementById('b').onclick=function(){return 1;};"}`

// 正常单阶段生成：三段逻辑被拼接成完整 HTML 写入 content.code，含骨架与 CSP。
func TestDemoBasicGenerateAssemblesHTML(t *testing.T) {
	g := &demoBasicGenerator{}
	sec := genSection("数组反转", []string{"下标访问"})
	sec.Type = types.SectionTypeDemoBasic
	client := &seqLLM{contents: []string{testDemoBasicJSON}}
	require.NoError(t, g.Generate(context.Background(), sec, genCtxWithLLM(client)))

	var content demoBasicContent
	require.NoError(t, json.Unmarshal(sec.Content, &content))
	require.Contains(t, content.Code, "<!DOCTYPE html>")
	require.Contains(t, content.Code, "Content-Security-Policy")
	require.Contains(t, content.Code, `<button id="b">点击</button>`)
	require.Contains(t, content.Code, `document.getElementById('b').onclick`)
	require.Contains(t, content.Code, "body{color:#333}")
}

// 解析失败自动重试一次，第二次成功。
func TestDemoBasicGenerateRetriesOnce(t *testing.T) {
	g := &demoBasicGenerator{}
	sec := genSection("x", nil)
	client := &seqLLM{contents: []string{"not json", testDemoBasicJSON}}
	require.NoError(t, g.Generate(context.Background(), sec, genCtxWithLLM(client)))

	var content demoBasicContent
	require.NoError(t, json.Unmarshal(sec.Content, &content))
	require.Contains(t, content.Code, "<!DOCTYPE html>")
}

// 三段内容全空判无效并重试，两次仍失败则返回 error。
func TestDemoBasicGenerateEmptyFails(t *testing.T) {
	g := &demoBasicGenerator{}
	sec := genSection("x", nil)
	client := &seqLLM{contents: []string{`{"style":"","body":"","script":""}`, `{"style":"","body":"","script":""}`}}
	require.Error(t, g.Generate(context.Background(), sec, genCtxWithLLM(client)))
	require.Empty(t, sec.Content)
}

// 缺少 LLM 客户端直接报错。
func TestDemoBasicGenerateRequiresClient(t *testing.T) {
	sec := genSection("x", nil)
	require.Error(t, (&demoBasicGenerator{}).Generate(context.Background(), sec, &types.GenerationContext{}))
}
