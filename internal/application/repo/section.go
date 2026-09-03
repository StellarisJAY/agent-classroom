package repo

import (
	"context"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/StellarisJAY/agent-classroom/internal/types"
)

// sectionRepo 实现 types.SectionRepo。
type sectionRepo struct {
	base
}

var _ types.SectionRepo = (*sectionRepo)(nil)

// NewSectionRepo 创建环节数据访问实现。
func NewSectionRepo(db *gorm.DB) types.SectionRepo {
	return &sectionRepo{base: newBase(db)}
}

func (r *sectionRepo) CreateBulk(ctx context.Context, sections []types.Section) error {
	if len(sections) == 0 {
		return nil
	}
	now := time.Now()
	for i := range sections {
		s := &sections[i]
		if s.ID == types.NilID {
			s.ID = types.NewID()
		}
		if s.CreateAt.IsZero() {
			s.CreateAt = now
		}
		if s.UpdateAt.IsZero() {
			s.UpdateAt = now
		}
	}
	return r.db(ctx).Create(&sections).Error
}

func (r *sectionRepo) ListByCourse(ctx context.Context, courseID types.ID) ([]types.Section, error) {
	var out []types.Section
	err := r.db(ctx).
		Where("course_id = ?", courseID).
		Order("position ASC").
		Find(&out).Error
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *sectionRepo) UpdateStatus(ctx context.Context, id types.ID, status string) error {
	return r.update(ctx, id, map[string]any{"status": status})
}

func (r *sectionRepo) UpdateContentSteps(ctx context.Context, id types.ID, content, steps datatypes.JSON) error {
	return r.update(ctx, id, map[string]any{"content": content, "steps": steps})
}

// update 更新单个环节并推进 update_at。
func (r *sectionRepo) update(ctx context.Context, id types.ID, fields map[string]any) error {
	fields["update_at"] = time.Now()
	res := r.db(ctx).Model(&types.Section{}).Where("id = ?", id).Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return types.ErrNotFound
	}
	return nil
}
