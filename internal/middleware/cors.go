package middleware

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// CORS 处理跨域。origins 为允许的来源列表（不带协议差异时仅精确匹配）。
func CORS(origins, allowedHeaders []string, maxAge int) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		// 允许列表包含 * 或匹配当前 origin
		if origin != "" && containsOrigin(origins, origin) {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", joinHeaders(allowedHeaders))
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Max-Age", strconv.Itoa(maxAge))
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func containsOrigin(origins []string, origin string) bool {
	for _, o := range origins {
		if o == "*" || o == origin {
			return true
		}
	}
	return false
}

func joinHeaders(headers []string) string {
	out := ""
	for i, h := range headers {
		if i > 0 {
			out += ", "
		}
		out += h
	}
	return out
}
