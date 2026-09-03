package types

import "errors"

// 业务错误码：code 用于统一 JSON 响应的 code 字段
const (
	CodeOK              = 0
	CodeBadRequest      = 40000
	CodeUnauthorized    = 40100
	CodeForbidden       = 40300
	CodeNotFound        = 40400
	CodeConflict        = 40900
	CodeValidationError = 42200
	CodeInternalError   = 50000
)

// BizError 业务错误，携带错误码，用于 controller 层区分响应
type BizError struct {
	Code int
	Msg  string
}

func (e *BizError) Error() string { return e.Msg }

// NewError 构造业务错误
func NewError(code int, msg string) *BizError {
	return &BizError{Code: code, Msg: msg}
}

var (
	ErrUnauthorized   = NewError(CodeUnauthorized, "未登录或登录已过期")
	ErrForbidden      = NewError(CodeForbidden, "无权访问")
	ErrNotFound       = NewError(CodeNotFound, "资源不存在")
	ErrInternal       = NewError(CodeInternalError, "服务器内部错误")
	ErrInvalidRequest = NewError(CodeBadRequest, "请求参数不合法")
)

// IsBizError 判断是否为 *BizError
func IsBizError(err error) bool {
	var be *BizError
	return errors.As(err, &be)
}
