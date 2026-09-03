package util

import (
	"fmt"
	"io"
)

// 说明：本文件提供 SSE（Server-Sent Events）的写入 helper。
// 事件结构约定：每个事件 data 行统一为 JSON。Gin 需在路由中
// 提前设置 Content-Type: text/event-stream 与 X-Accel-Buffering: no。

// WriteSSEEvent 写入一个 SSE 事件。eventName 可空；data 必须是单行（JSON 不应含换行）。
// 结束事件请调用 WriteSSEDone 或发送自定义 done 事件。
func WriteSSEEvent(w io.Writer, eventName, data string) error {
	if eventName != "" {
		if _, err := fmt.Fprintf(w, "event: %s\n", eventName); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
		return err
	}
	return nil
}

// WriteSSEDone 写入一个标准结束事件，前端收到后断开连接。
func WriteSSEDone(w io.Writer) error {
	_, err := fmt.Fprint(w, "event: done\ndata: {}\n\n")
	return err
}

// WriteSSEError 写入错误事件并结束流。
func WriteSSEError(w io.Writer, message string) error {
	_, err := fmt.Fprintf(w, "event: error\ndata: %q\n\n", message)
	return err
}
