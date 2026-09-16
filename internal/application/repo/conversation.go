package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/StellarisJAY/agent-classroom/internal/types"
)

// conversationRepo 实现 types.ConversationRepo。
type conversationRepo struct {
	base
}

var _ types.ConversationRepo = (*conversationRepo)(nil)

// NewConversationRepo 创建问答会话数据访问实现。
func NewConversationRepo(db *gorm.DB) types.ConversationRepo {
	return &conversationRepo{base: newBase(db)}
}

// Create 新建一条空会话（title 为空，首问时写入标题）。
func (r *conversationRepo) Create(ctx context.Context, courseID, userID types.ID) (*types.Conversation, error) {
	c := types.Conversation{ID: types.NewID(), CourseID: courseID, UserID: userID}
	if err := r.db(ctx).Create(&c).Error; err != nil {
		return nil, fmt.Errorf("create conversation: %w", err)
	}
	return &c, nil
}

// GetByID 按会话 ID 取会话；user_id 不匹配（非属主）按 not found 处理。
func (r *conversationRepo) GetByID(ctx context.Context, userID, id types.ID) (*types.Conversation, error) {
	var out types.Conversation
	err := r.db(ctx).Where("id = ? AND user_id = ?", id, userID).First(&out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, types.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get conversation: %w", err)
	}
	return &out, nil
}

// ListByCourse 返回某用户某课程的全部会话，按 update_at 降序（最近活跃在前）。
func (r *conversationRepo) ListByCourse(ctx context.Context, courseID, userID types.ID) ([]types.Conversation, error) {
	var out []types.Conversation
	err := r.db(ctx).
		Where("course_id = ? AND user_id = ?", courseID, userID).
		Order("update_at DESC").
		Find(&out).Error
	if err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateTitle 更新会话标题并刷新 update_at（同一事务保证一致）。
func (r *conversationRepo) UpdateTitle(ctx context.Context, id types.ID, title string) error {
	return r.db(ctx).Transaction(func(tx *gorm.DB) error {
		// 仅首问（title 仍为空）写入标题，避免后续提问覆盖首问标题。
		if err := tx.Model(&types.Conversation{}).
			Where("id = ? AND title = ''", id).
			Update("title", title).Error; err != nil {
			return err
		}
		return tx.Model(&types.Conversation{}).
			Where("id = ?", id).
			Update("update_at", time.Now()).Error
	})
}

func (r *conversationRepo) ListMessages(ctx context.Context, conversationID types.ID) ([]types.Message, error) {
	var out []types.Message
	err := r.db(ctx).
		Where("conversation_id = ?", conversationID).
		Order("created_at ASC").
		Find(&out).Error
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Append 追加一条消息并刷新会话 update_at（同一事务保证一致）。
func (r *conversationRepo) Append(ctx context.Context, conversationID types.ID, role string, content json.RawMessage, sectionID *types.ID) error {
	return r.db(ctx).Transaction(func(tx *gorm.DB) error {
		m := types.Message{
			ID:             types.NewID(),
			ConversationID: conversationID,
			Role:           role,
			Content:        content,
			SectionID:      sectionID,
			CreateAt:       time.Now(),
		}
		if err := tx.Create(&m).Error; err != nil {
			return err
		}
		return tx.Model(&types.Conversation{}).
			Where("id = ?", conversationID).
			Update("update_at", time.Now()).Error
	})
}
