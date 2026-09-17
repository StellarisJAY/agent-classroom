package service

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"

	"github.com/StellarisJAY/agent-classroom/internal/config"
	"github.com/StellarisJAY/agent-classroom/internal/model"
	"github.com/StellarisJAY/agent-classroom/internal/types"
	"github.com/StellarisJAY/agent-classroom/internal/util"
)

// 默认分页参数与上限
const (
	defaultCoursePageSize = 20
	maxCoursePageSize     = 100
)

// CourseService 课程业务实现。
type CourseService struct {
	courseRepo  types.CourseRepo
	outlineRepo types.OutlineRepo
	historyRepo types.OutlineHistoryRepo
	docRepo     types.DocumentRepo
	tm          types.TransactionManager
	storage     types.Storage
	modelCfgSvc types.ModelConfigService
	registry    *model.Registry
	docs        *docLoader
	retry       types.RetryPolicy
	// outlineMu 保护 outlineTasks；单进程内存任务表，与课程状态/大纲表共同推导任务状态
	outlineMu    sync.Mutex
	outlineTasks map[types.ID]*outlineTask
}

var _ types.CourseService = (*CourseService)(nil)

// NewCourseService 创建课程业务实现。
func NewCourseService(
	courseRepo types.CourseRepo,
	outlineRepo types.OutlineRepo,
	historyRepo types.OutlineHistoryRepo,
	docRepo types.DocumentRepo,
	tm types.TransactionManager,
	storage types.Storage,
	modelCfgSvc types.ModelConfigService,
	registry *model.Registry,
	docs *docLoader,
	cfg *config.Config,
) types.CourseService {
	return &CourseService{
		courseRepo:   courseRepo,
		outlineRepo:  outlineRepo,
		historyRepo:  historyRepo,
		docRepo:      docRepo,
		tm:           tm,
		storage:      storage,
		modelCfgSvc:  modelCfgSvc,
		registry:     registry,
		docs:         docs,
		retry:        cfg.Model.Retry.Policy(),
		outlineTasks: make(map[types.ID]*outlineTask),
	}
}

// ---- 列表 ----

// ListDocuments 返回课程全部参考文档的提取状态（归属校验走 courseRepo.GetByID）。
func (s *CourseService) ListDocuments(ctx context.Context, userID, courseID types.ID) ([]types.DocumentStatusView, error) {
	if _, err := s.courseRepo.GetByID(ctx, userID, courseID); err != nil {
		if errors.Is(err, types.ErrNotFound) {
			return nil, types.ErrCourseNotFound
		}
		return nil, err
	}
	docs, err := s.docRepo.ListByCourse(ctx, courseID)
	if err != nil {
		return nil, err
	}
	items := make([]types.DocumentStatusView, 0, len(docs))
	for i := range docs {
		items = append(items, types.DocumentStatusView{
			ID:              docs[i].ID,
			Filename:        docs[i].Filename,
			ExtractedStatus: docs[i].ExtractedStatus,
		})
	}
	return items, nil
}

func (s *CourseService) List(ctx context.Context, userID types.ID, req types.CourseListReq) (*types.CourseListResp, error) {
	scope := types.CourseScopeAll
	if req.Scope != "" {
		scope = req.Scope
	}
	if scope != types.CourseScopeMine && scope != types.CourseScopePublic {
		scope = types.CourseScopeAll
	}

	page, size := 1, defaultCoursePageSize
	if req.Page > 0 {
		page = req.Page
	}
	if req.PageSize > 0 {
		size = req.PageSize
	}
	if size > maxCoursePageSize {
		size = maxCoursePageSize
	}

	progress := strings.TrimSpace(req.Progress)

	rows, total, err := s.courseRepo.List(ctx, userID, types.CourseListFilter{
		Scope:    scope,
		Keyword:  strings.TrimSpace(req.Keyword),
		Progress: progress,
		Page:     page,
		PageSize: size,
		Offset:   (page - 1) * size,
	})
	if err != nil {
		return nil, err
	}

	items := make([]types.CourseListItem, 0, len(rows))
	for i := range rows {
		c := &rows[i].Course
		items = append(items, types.CourseListItem{
			ID:        c.ID,
			Title:     c.Title,
			Status:    c.Status,
			IsPublic:  c.IsPublic,
			Owned:     c.OwnerID == userID,
			Progress:  rows[i].ProgressStatus,
			CreatedAt: c.CreateAt,
			UpdatedAt: c.UpdateAt,
		})
	}

	return &types.CourseListResp{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: size,
	}, nil
}

// ---- 创建 ----

// Create 创建草稿课程并保存参考文档。文档先落库为 pending，
// 提取由大纲生成后台任务首步执行（见 generate_outline.go），提取结果落库缓存。
func (s *CourseService) Create(ctx context.Context, userID types.ID, req types.CreateCourseReq) (*types.CourseCreateResp, error) {
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return nil, types.ErrPromptRequired
	}
	// 前置校验全部文件格式与大小，避免写入半途失败
	for _, f := range req.Files {
		if !util.IsSupportedDoc(f.Name) {
			return nil, types.ErrUnsupportedFile
		}
		if len(f.Data) > util.MaxUploadBytes {
			return nil, types.ErrFileTooLarge
		}
	}

	course := &types.Course{
		OwnerID:            userID,
		Title:              "",
		Prompt:             prompt,
		Status:             types.CourseStatusDraft,
		ModelConfigID:      req.ModelConfigID,
		GenerateImages:     req.GenerateImages,
		ImageModelConfigID: req.ImageModelConfigID,
		Thinking:           normalizeThinking(req.Thinking),
		OutlineCount:       normalizeOutlineCount(req.OutlineCount),
		CreateBy:           &userID,
	}

	err := s.tm.Transaction(ctx, func(ctx context.Context) error {
		if err := s.courseRepo.Create(ctx, course); err != nil {
			return err
		}
		for _, f := range req.Files {
			key := course.ID.String() + "/" + filepath.Base(f.Name)
			url, err := s.storage.Put(ctx, key, strings.NewReader(string(f.Data)))
			if err != nil {
				return err
			}
			by := userID
			if err := s.docRepo.Create(ctx, &types.Document{
				CourseID:        course.ID,
				Filename:        f.Name,
				URL:             url,
				ExtractedStatus: types.ExtractStatusPending,
				CreateBy:        &by,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &types.CourseCreateResp{
		ID:     course.ID,
		Title:  course.Title,
		Prompt: course.Prompt,
		Status: course.Status,
	}, nil
}

// normalizeThinking 归一化思考限制取值；空或非法一律回退为 default。
func normalizeThinking(v string) string {
	switch v {
	case model.ThinkingOff, model.ThinkingDefault, model.ThinkingMax:
		return v
	default:
		return model.ThinkingDefault
	}
}
