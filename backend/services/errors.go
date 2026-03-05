package services

import "fmt"

// AppError 统一封装服务层业务错误，包含 HTTP 语义与底层错误链。
type AppError struct {
	// HTTPCode 表示应返回给客户端的 HTTP 状态码。
	HTTPCode int
	// Message 是面向调用方的业务错误描述。
	Message string
	// Data 用于附带结构化扩展信息（可选）。
	Data interface{}
	// Err 保留底层错误，便于日志与错误链追踪。
	Err error
}

// Error 返回可读错误文本，并在存在底层错误时拼接详细信息。
func (e *AppError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Unwrap 允许 errors.Is / errors.As 沿错误链继续匹配底层错误。
func (e *AppError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// newAppError 构造包含 HTTP 状态码的服务层错误。
func newAppError(httpCode int, message string, err error) *AppError {
	return &AppError{HTTPCode: httpCode, Message: message, Err: err}
}

// newAppErrorWithData 在基础错误上附加结构化数据，便于前端展示与调试。
func newAppErrorWithData(httpCode int, message string, data interface{}, err error) *AppError {
	return &AppError{HTTPCode: httpCode, Message: message, Data: data, Err: err}
}
