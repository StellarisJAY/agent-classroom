package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/StellarisJAY/agent-classroom/internal/types"
	"github.com/StellarisJAY/agent-classroom/internal/util"
)

// Auth 返回 JWT 鉴权中间件。secret 用于校验令牌。
func Auth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			abortUnauthorized(c)
			return
		}
		tokenString := strings.TrimPrefix(header, "Bearer ")
		claims, err := util.ParseToken(secret, tokenString)
		if err != nil {
			abortUnauthorized(c)
			return
		}
		SetUserID(c, claims.UserID)
		c.Next()
	}
}

func abortUnauthorized(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusOK, gin.H{
		"code":    types.CodeUnauthorized,
		"message": "未登录或登录已过期",
	})
}
