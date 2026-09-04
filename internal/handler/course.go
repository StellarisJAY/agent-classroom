package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/StellarisJAY/agent-classroom/internal/types"
	"github.com/StellarisJAY/agent-classroom/internal/util"
)

// CourseHandler 课程相关 HTTP 处理器。
type CourseHandler struct {
	svc types.CourseService
}

// NewCourseHandler 创建课程处理器。
func NewCourseHandler(svc types.CourseService) *CourseHandler {
	return &CourseHandler{svc: svc}
}

// List 查询当前用户课程列表（支持筛选 + 分页）。
func (h *CourseHandler) List(c *gin.Context) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}
	var req types.CourseListReq
	if err := bindQuery(c, &req); err != nil {
		Error(c, err)
		return
	}
	resp, err := h.svc.List(c.Request.Context(), userID, &req)
	if err != nil {
		Error(c, err)
		return
	}
	OK(c, resp)
}

// Create 创建草稿课程（multipart：prompt + files[]，参考文档仅 txt/md）。
func (h *CourseHandler) Create(c *gin.Context) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}
	prompt := c.PostForm("prompt")
	files, err := readUploadedFiles(c)
	if err != nil {
		Error(c, err)
		return
	}
	req := &types.CreateCourseReq{
		Prompt:   prompt,
		Files:    files,
		Thinking: c.PostForm("thinking"),
	}
	if raw := c.PostForm("model_config_id"); raw != "" {
		id, perr := types.ParseID(raw)
		if perr != nil {
			Error(c, types.NewError(types.CodeBadRequest, "model_config_id 不合法"))
			return
		}
		req.ModelConfigID = &id
	}
	if raw := c.PostForm("outline_count"); raw != "" {
		n, nerr := strconv.Atoi(raw)
		if nerr != nil {
			Error(c, types.NewError(types.CodeBadRequest, "outline_count 不合法"))
			return
		}
		req.OutlineCount = n
	}
	resp, err := h.svc.Create(c.Request.Context(), userID, req)
	if err != nil {
		Error(c, err)
		return
	}
	OK(c, resp)
}

// GetOutline 查询某课程已保存的大纲。
func (h *CourseHandler) GetOutline(c *gin.Context) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}
	courseID, err := pathID(c)
	if err != nil {
		Error(c, err)
		return
	}
	outline, err := h.svc.GetOutline(c.Request.Context(), userID, courseID)
	if err != nil {
		Error(c, err)
		return
	}
	OK(c, outline)
}

// GenerateOutline 流式返回生成的大纲（SSE）。
func (h *CourseHandler) GenerateOutline(c *gin.Context) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}
	courseID, err := pathID(c)
	if err != nil {
		Error(c, err)
		return
	}

	w := c.Writer
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flush(w)

	// 立即发送 start 事件以刷出响应头，避免长时空白
	if err := util.WriteSSEEvent(w, "start", "{}"); err != nil {
		return
	}
	flush(w)

	result, err := h.svc.GenerateOutline(c.Request.Context(), userID, courseID)
	if err != nil {
		_ = util.WriteSSEError(w, errSSEMessage(err))
		flush(w)
		return
	}

	if meta, merr := json.Marshal(gin.H{"title": result.Title}); merr == nil {
		_ = util.WriteSSEEvent(w, "meta", string(meta))
		flush(w)
	}
	for i := range result.Sections {
		payload, perr := json.Marshal(gin.H{"index": i, "section": result.Sections[i]})
		if perr != nil {
			continue
		}
		if uerr := util.WriteSSEEvent(w, "section", string(payload)); uerr != nil {
			return
		}
		flush(w)
	}
	_ = util.WriteSSEDone(w)
	flush(w)
}

// readUploadedFiles 读取 multipart 的 files 字段到内存；校验数量与单文件大小。
func readUploadedFiles(c *gin.Context) ([]types.UploadedFile, error) {
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		return nil, types.NewError(types.CodeBadRequest, "上传内容不合法")
	}
	headers := c.Request.MultipartForm.File["files"]
	if len(headers) == 0 {
		return nil, nil
	}
	files := make([]types.UploadedFile, 0, len(headers))
	for _, fh := range headers {
		if fh.Size > util.MaxUploadBytes {
			return nil, types.ErrFileTooLarge
		}
		src, err := fh.Open()
		if err != nil {
			return nil, types.NewError(types.CodeBadRequest, "读取上传文件失败")
		}
		data, rerr := io.ReadAll(src)
		src.Close()
		if rerr != nil {
			return nil, types.NewError(types.CodeBadRequest, "读取上传文件失败")
		}
		if len(data) > util.MaxUploadBytes {
			return nil, types.ErrFileTooLarge
		}
		if !util.IsSupportedDoc(fh.Filename) {
			return nil, types.ErrUnsupportedFile
		}
		files = append(files, types.UploadedFile{Name: fh.Filename, Data: bytes.TrimSpace(data)})
	}
	return files, nil
}

// flush 尽量刷新到客户端；非可刷新 Writer 时忽略。
func flush(w io.Writer) {
	if f, ok := w.(interface{ Flush() }); ok {
		f.Flush()
	}
}

// errSSEMessage 提取 SSE 错误消息；非业务错误返回通用文案。
func errSSEMessage(err error) string {
	var be *types.BizError
	if errors.As(err, &be) {
		return be.Msg
	}
	return types.ErrOutlineFailed.Msg
}
