//go:build !windows

package app

import "errors"

// 非 Windows 平台不做降级明文存储：宁可失败，也不悄悄把 token 写进磁盘。
func protect([]byte) ([]byte, error) {
	return nil, errors.New("凭据加密仅支持 Windows")
}

func unprotect([]byte) ([]byte, error) {
	return nil, errors.New("凭据加密仅支持 Windows")
}
