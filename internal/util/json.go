package util

import (
	"bytes"
	"encoding/json"
	"strings"
)

// ExtractJSON 从 LLM 输出中提取一段合法 JSON 并解析到 v。
// 模型可能用 ```json … ``` 代码围栏包裹或在前后夹带说明文字，
// 这里去掉代码围栏后定位首个结构化括号，保证解析健壮。
func ExtractJSON(raw string, v any) error {
	s := stripCodeFence(raw)
	s = strings.TrimSpace(s)

	start := indexOfAny(s, '{', '[')
	if start < 0 {
		return &json.SyntaxError{}
	}
	// 去掉围栏后若剩余明显是散乱文字（非以括号开头），仍尝试从首个括号取子串。
	end := matchEnd(s, start)
	if end < 0 {
		end = len(s)
	}
	return json.Unmarshal([]byte(s[start:end+1]), v)
}

// stripCodeFence 去除 ```json 与 ``` 围栏行。
func stripCodeFence(s string) string {
	var b bytes.Buffer
	for _, line := range strings.Split(s, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "```") {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func indexOfAny(s string, cs ...rune) int {
	for i, r := range s {
		for _, c := range cs {
			if r == c {
				return i
			}
		}
	}
	return -1
}

// matchEnd 返回与 start 处括号匹配的结束下标；不匹配返回 -1。
func matchEnd(s string, start int) int {
	open := rune(s[start])
	var close rune
	switch open {
	case '{':
		close = '}'
	case '[':
		close = ']'
	default:
		return -1
	}
	depth := 0
	inStr := false
	escaped := false
	for i := start; i < len(s); i++ {
		c := rune(s[i])
		if inStr {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
