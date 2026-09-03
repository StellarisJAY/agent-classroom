package middleware

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// gin context 中用户身份的 key
const ctxUserID = "ctx_user_id"

// SetUserID 将当前登录用户 ID 写入 context
func SetUserID(c *gin.Context, id uuid.UUID) {
	c.Set(ctxUserID, id.String())
}

// UserID 从 context 取当前用户 ID；未登录返回错误
func UserID(c *gin.Context) (uuid.UUID, error) {
	v, ok := c.Get(ctxUserID)
	if !ok {
		return uuid.Nil, errors.New("missing user id in context")
	}
	s, ok := v.(string)
	if !ok {
		return uuid.Nil, errors.New("invalid user id in context")
	}
	return uuid.Parse(s)
}
