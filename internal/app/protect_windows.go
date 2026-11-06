//go:build windows

package app

import (
	"syscall"
	"unsafe"
)

// dataBlob 对应 Windows DATA_BLOB。
type dataBlob struct {
	cbData uint32
	pbData *byte
}

var (
	crypt32dll         = syscall.NewLazyDLL("crypt32.dll")
	kernel32dll        = syscall.NewLazyDLL("kernel32.dll")
	procCryptProtect   = crypt32dll.NewProc("CryptProtectData")
	procCryptUnprotect = crypt32dll.NewProc("CryptUnprotectData")
	procLocalFree      = kernel32dll.NewProc("LocalFree")
)

// 禁止 DPAPI 弹出任何 UI，避免登录失败时出现系统对话框。
const cryptProtectUIForbidden = 0x1

func protect(plain []byte) ([]byte, error) {
	return dpapi(procCryptProtect, plain)
}

func unprotect(sealed []byte) ([]byte, error) {
	return dpapi(procCryptUnprotect, sealed)
}

func dpapi(fn *syscall.LazyProc, in []byte) ([]byte, error) {
	src := dataBlob{cbData: uint32(len(in))}
	if len(in) > 0 {
		src.pbData = &in[0]
	}
	var out dataBlob
	ret, _, callErr := fn.Call(
		uintptr(unsafe.Pointer(&src)),
		0, 0, 0, 0,
		uintptr(cryptProtectUIForbidden),
		uintptr(unsafe.Pointer(&out)),
	)
	if ret == 0 {
		return nil, callErr
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	if out.cbData == 0 || out.pbData == nil {
		return nil, nil
	}
	result := make([]byte, out.cbData)
	copy(result, unsafe.Slice(out.pbData, out.cbData))
	return result, nil
}
