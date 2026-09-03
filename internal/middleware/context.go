package middleware

import (
	"errors"

	"github.com/gin-gonic/gin"

	"github.com/StellarisJAY/agent-classroom/internal/types"
)

// gin context 中用户身份的 key
const ctxUserID = "ctx_user_id"

// SetUserID 将当前登录用户 ID 写入 context
func SetUserID(c *gin.Context, id types.ID) {
	c.Set(ctxUserID, id.String())
}

// UserID 从 context 取当前用户 ID；未登录返回错误
func UserID(c *gin.Context) (types.ID, error) {
	v, ok := c.Get(ctxUserID)
	if !ok {
		return types.NilID, errors.New("missing user id in context")
	}
	s, ok := v.(string)
	if !ok {
		return types.NilID, errors.New("invalid user id in context")
	}
	return types.ParseID(s)
}
