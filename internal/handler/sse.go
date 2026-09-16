package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// 讨论模式的 SSE 事件统一封装（docs/讨论模式方案.md §8）。
type sseEvent struct {
	Type  string          `json:"type"` // text | action | end | error
	Delta string          `json:"delta,omitempty"`
	Name  string          `json:"name,omitempty"`
	Args  json.RawMessage `json:"args,omitempty"`
	Msg   string          `json:"msg,omitempty"`
}

// sseWriter 单条请求生命周期的 SSE 写出器：
// 严格按序写事件并逐条 Flush；路由无 gzip 包装，recovery/logger 均用原生
// gin writer 封装，不影响 Flusher（docs/讨论模式方案.md §9 验证清单已核）。
type sseWriter struct {
	c        *gin.Context
	flusher  http.Flusher
	done     chan struct{}
	flushErr chan error
}

// openSSE 建立 SSE 响应并启动心跳。调用后本次 HTTP 响应只能经此器写事件。
func openSSE(c *gin.Context) (*sseWriter, error) {
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("当前 writer 不支持 Flush")
	}
	headers := c.Writer.Header()
	headers.Set("Content-Type", "text/event-stream; charset=utf-8")
	headers.Set("Cache-Control", "no-cache")
	headers.Set("Connection", "keep-alive")
	headers.Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)
	flusher.Flush()

	w := &sseWriter{c: c, flusher: flusher,
		done:     make(chan struct{}),
		flushErr: make(chan error, 1)}
	go w.heartbeat()
	return w, nil
}

// Close 停止心跳 goroutine。
func (w *sseWriter) Close() {
	select {
	case <-w.done:
	default:
		close(w.done)
	}
}

// heartbeat 注释行保活，30s 间隔，覆盖最长 loop 时长（8 轮 LLM 流式调用的中的静默间隙）。
func (w *sseWriter) heartbeat() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-w.done:
			return
		case <-ticker.C:
			if _, err := fmt.Fprint(w.c.Writer, ": keep-alive\n\n"); err != nil {
				return
			}
			w.flusher.Flush()
		}
	}
}

// write 写出一个事件帧并 Flush。
func (w *sseWriter) write(ev sseEvent) error {
	payload, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w.c.Writer, "data: %s\n\n", payload); err != nil {
		return err
	}
	w.flusher.Flush()
	return nil
}
