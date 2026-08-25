package domain

import "errors"

var (
	ErrNotFound      = errors.New("对象不存在")
	ErrConflict      = errors.New("版本冲突")
	ErrInvalid       = errors.New("参数无效")
	ErrState         = errors.New("状态不允许此操作")
	ErrWindowExpired = errors.New("检查窗口已过期")
)
