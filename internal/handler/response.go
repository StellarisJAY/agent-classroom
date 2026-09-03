package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"github.com/StellarisJAY/agent-classroom/internal/types"
)

// 统一 JSON 响应格式
type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

// OK 成功响应
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Response{Code: types.CodeOK, Message: "ok", Data: data})
}

// OKNoData 成功无数据
func OKNoData(c *gin.Context) {
	c.JSON(http.StatusOK, Response{Code: types.CodeOK, Message: "ok"})
}

// Fail 以业务错误码与消息返回失败。HTTP 层统一返回 200，业务成败由 code 表达。
func Fail(c *gin.Context, code int, msg string) {
	c.JSON(http.StatusOK, Response{Code: code, Message: msg})
}

// Error 根据错误类型决定响应：BizError → 对应业务 code；其余 → 500
func Error(c *gin.Context, err error) {
	var be *types.BizError
	if errors.As(err, &be) {
		Fail(c, be.Code, be.Msg)
		return
	}
	c.Error(err)
	Fail(c, types.CodeInternalError, types.ErrInternal.Msg)
}

// bindQuery 绑定并校验 URL 查询参数；失败返回 400/422
func bindQuery(c *gin.Context, dst any) error {
	if err := c.ShouldBindQuery(dst); err != nil {
		var verr validator.ValidationErrors
		if errors.As(err, &verr) {
			return types.NewError(types.CodeValidationError, err.Error())
		}
		return types.NewError(types.CodeBadRequest, "查询参数不合法")
	}
	return nil
}

// bindJSON 绑定并校验请求体；失败返回 400/422
func bindJSON(c *gin.Context, dst any) error {
	if err := c.ShouldBindJSON(dst); err != nil {
		var verr validator.ValidationErrors
		if errors.As(err, &verr) {
			return types.NewError(types.CodeValidationError, err.Error())
		}
		return types.NewError(types.CodeBadRequest, "请求体格式错误")
	}
	return nil
}

// pathID 解析 :id 路由参数并校验为合法 ID
func pathID(c *gin.Context) (types.ID, error) {
	id := c.Param("id")
	if id == "" {
		return types.NilID, types.ErrInvalidRequest
	}
	parsed, err := types.ParseID(id)
	if err != nil {
		return types.NilID, types.NewError(types.CodeValidationError, "id 格式不正确")
	}
	return parsed, nil
}
