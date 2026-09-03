package service

import (
	"context"
	"strings"

	"github.com/StellarisJAY/agent-classroom/internal/types"
)

// 默认分页参数与上限
const (
	defaultCoursePageSize = 20
	maxCoursePageSize     = 100
)

// CourseService 课程业务实现。
type CourseService struct {
	repo types.CourseRepo
}

var _ types.CourseService = (*CourseService)(nil)

// NewCourseService 创建课程业务实现。
func NewCourseService(repo types.CourseRepo) types.CourseService {
	return &CourseService{repo: repo}
}

func (s *CourseService) List(ctx context.Context, userID types.ID, req *types.CourseListReq) (*types.CourseListResp, error) {
	scope := types.CourseScopeAll
	if req != nil && req.Scope != "" {
		scope = req.Scope
	}
	if scope != types.CourseScopeMine && scope != types.CourseScopePublic {
		scope = types.CourseScopeAll
	}

	page, size := 1, defaultCoursePageSize
	if req != nil {
		if req.Page > 0 {
			page = req.Page
		}
		if req.PageSize > 0 {
			size = req.PageSize
		}
	}
	if size > maxCoursePageSize {
		size = maxCoursePageSize
	}

	progress := ""
	if req != nil {
		progress = strings.TrimSpace(req.Progress)
	}

	rows, total, err := s.repo.List(ctx, userID, types.CourseListFilter{
		Scope:    scope,
		Keyword:  strings.TrimSpace(keywordOf(req)),
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
			ID:          c.ID,
			Title:       c.Title,
			Description: c.Description,
			Status:      c.Status,
			IsPublic:    c.IsPublic,
			Owned:       c.OwnerID == userID,
			Progress:    rows[i].ProgressStatus,
			CreatedAt:   c.CreateAt,
			UpdatedAt:   c.UpdateAt,
		})
	}

	return &types.CourseListResp{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: size,
	}, nil
}

func keywordOf(req *types.CourseListReq) string {
	if req == nil {
		return ""
	}
	return req.Keyword
}
