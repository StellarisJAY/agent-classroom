package repo

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/StellarisJAY/agent-classroom/internal/types"
)

// courseRepo 实现 types.CourseRepo。
type courseRepo struct {
	base
}

var _ types.CourseRepo = (*courseRepo)(nil)

// NewCourseRepo 创建课程数据访问实现。
func NewCourseRepo(db *gorm.DB) types.CourseRepo {
	return &courseRepo{base: newBase(db)}
}

func (r *courseRepo) List(ctx context.Context, userID types.ID, f types.CourseListFilter) ([]types.CourseListRow, int64, error) {
	q := r.base.db(ctx).Table("course c").
		Select("c.*, COALESCE(p.status, 'unstarted') AS progress_status").
		Joins("LEFT JOIN progress p ON p.course_id = c.id AND p.user_id = ?", userID)

	switch f.Scope {
	case types.CourseScopeMine:
		q = q.Where("c.owner_id = ?", userID)
	case types.CourseScopePublic:
		q = q.Where("c.is_public = ? AND c.status = ?", true, types.CourseStatusCompleted)
	default: // all
		q = q.Where("(c.owner_id = ? OR (c.is_public = ? AND c.status = ?))",
			userID, true, types.CourseStatusCompleted)
	}

	if kw := strings.TrimSpace(f.Keyword); kw != "" {
		q = q.Where("c.title ILIKE ?", "%"+escapeLike(kw)+"%")
	}
	if f.Progress != "" {
		q = q.Where("COALESCE(p.status, 'unstarted')::text = ?", f.Progress)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []types.CourseListRow
	if err := q.Order("c.create_at DESC").Offset(f.Offset).Limit(f.PageSize).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *courseRepo) Create(ctx context.Context, c *types.Course) error {
	if c.ID == types.NilID {
		c.ID = types.NewID()
	}
	now := time.Now()
	if c.CreateAt.IsZero() {
		c.CreateAt = now
	}
	if c.UpdateAt.IsZero() {
		c.UpdateAt = now
	}
	return r.db(ctx).Create(c).Error
}

func (r *courseRepo) GetByID(ctx context.Context, ownerID, id types.ID) (*types.Course, error) {
	var c types.Course
	err := r.db(ctx).Where("id = ? AND owner_id = ?", id, ownerID).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, types.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *courseRepo) UpdateTitle(ctx context.Context, id types.ID, title string) error {
	res := r.db(ctx).
		Model(&types.Course{}).
		Where("id = ?", id).
		Updates(map[string]any{"title": title, "update_at": time.Now()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return types.ErrNotFound
	}
	return nil
}

// escapeLike 转义 ILIKE 通配符与反斜杠，使关键字按字面匹配。
func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}
