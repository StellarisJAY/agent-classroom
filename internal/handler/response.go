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

// Fail 以指定 HTTP 状态码与业务错误码返回失败
func Fail(c *gin.Context, httpStatus, code int, msg string) {
	c.JSON(httpStatus, Response{Code: code, Message: msg})
}

// Error 根据错误类型决定响应：BizError → 对应语义；其余 → 500
func Error(c *gin.Context, err error) {
	var be *types.BizError
	if errors.As(err, &be) {
		status := bizErrToHTTP(be.Code)
		Fail(c, status, be.Code, be.Msg)
		return
	}
	c.Error(err)
	Fail(c, http.StatusInternalServerError, types.CodeInternalError, types.ErrInternal.Msg)
}

func bizErrToHTTP(code int) int {
	switch code {
	case types.CodeUnauthorized:
		return http.StatusUnauthorized
	case types.CodeForbidden:
		return http.StatusForbidden
	case types.CodeNotFound:
		return http.StatusNotFound
	case types.CodeConflict:
		return http.StatusConflict
	case types.CodeValidationError:
		return http.StatusUnprocessableEntity
	case types.CodeBadRequest:
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
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
