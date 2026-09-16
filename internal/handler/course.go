package handler

import (
	"bytes"
	"io"
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
	resp, err := h.svc.List(c.Request.Context(), userID, req)
	if err != nil {
		Error(c, err)
		return
	}
	OK(c, resp)
}

// Create 创建草稿课程（multipart：prompt + files[]，参考文档支持 markdown/pdf/word）。
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
	req := types.CreateCourseReq{
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
	if raw := c.PostForm("generate_images"); raw != "" {
		b, berr := strconv.ParseBool(raw)
		if berr != nil {
			Error(c, types.NewError(types.CodeBadRequest, "generate_images 不合法"))
			return
		}
		req.GenerateImages = b
	}
	if raw := c.PostForm("image_model_config_id"); raw != "" {
		id, perr := types.ParseID(raw)
		if perr != nil {
			Error(c, types.NewError(types.CodeBadRequest, "image_model_config_id 不合法"))
			return
		}
		req.ImageModelConfigID = &id
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

// ListDocuments 查询某课程参考文档提取状态。
func (h *CourseHandler) ListDocuments(c *gin.Context) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}
	courseID, err := pathID(c)
	if err != nil {
		Error(c, err)
		return
	}
	items, err := h.svc.ListDocuments(c.Request.Context(), userID, courseID)
	if err != nil {
		Error(c, err)
		return
	}
	OK(c, items)
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

// GenerateOutline 触发大纲后台生成任务（异步）。
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
	if err := h.svc.StartOutline(c.Request.Context(), userID, courseID, ""); err != nil {
		Error(c, err)
		return
	}
	OK(c, gin.H{})
}

// RegenerateOutline 携带修改意见触发大纲重新生成任务（异步）。
func (h *CourseHandler) RegenerateOutline(c *gin.Context) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}
	courseID, err := pathID(c)
	if err != nil {
		Error(c, err)
		return
	}
	var req types.RegenerateOutlineReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, types.NewError(types.CodeBadRequest, "请求体不合法"))
		return
	}
	if err := h.svc.StartOutline(c.Request.Context(), userID, courseID, req.Feedback); err != nil {
		Error(c, err)
		return
	}
	OK(c, gin.H{})
}

// OutlineTask 轮询大纲生成任务状态（done 附带大纲视图）。
func (h *CourseHandler) OutlineTask(c *gin.Context) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}
	courseID, err := pathID(c)
	if err != nil {
		Error(c, err)
		return
	}
	task, err := h.svc.GetOutlineTask(c.Request.Context(), userID, courseID)
	if err != nil {
		Error(c, err)
		return
	}
	OK(c, task)
}

// ListOutlineVersions 返回大纲历史版本列表。
func (h *CourseHandler) ListOutlineVersions(c *gin.Context) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}
	courseID, err := pathID(c)
	if err != nil {
		Error(c, err)
		return
	}
	versions, err := h.svc.ListOutlineVersions(c.Request.Context(), userID, courseID)
	if err != nil {
		Error(c, err)
		return
	}
	OK(c, versions)
}

// RevertOutline 回退大纲到指定历史版本。
func (h *CourseHandler) RevertOutline(c *gin.Context) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}
	courseID, err := pathID(c)
	if err != nil {
		Error(c, err)
		return
	}
	var req types.RevertOutlineReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, types.NewError(types.CodeBadRequest, "请求体不合法"))
		return
	}
	outline, err := h.svc.RevertOutline(c.Request.Context(), userID, courseID, req.Version)
	if err != nil {
		Error(c, err)
		return
	}
	OK(c, outline)
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
