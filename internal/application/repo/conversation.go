package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

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

// GetOrCreate 取（或建）某用户某课程的唯一会话。
// 用 INSERT ... ON CONFLICT DO NOTHING 抢占 (course_id, user_id) 唯一约束，
// 冲突后回查既有记录，应对并发首提问。
func (r *conversationRepo) GetOrCreate(ctx context.Context, courseID, userID types.ID) (*types.Conversation, error) {
	c := types.Conversation{ID: types.NewID(), CourseID: courseID, UserID: userID}
	if err := r.db(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&c).Error; err != nil {
		return nil, fmt.Errorf("create conversation: %w", err)
	}
	var out types.Conversation
	if err := r.db(ctx).
		Where("course_id = ? AND user_id = ?", courseID, userID).
		First(&out).Error; err != nil {
		return nil, fmt.Errorf("get conversation: %w", err)
	}
	return &out, nil
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
