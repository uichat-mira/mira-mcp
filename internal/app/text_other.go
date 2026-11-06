//go:build !windows

package app

// connectedText 在非 Windows 平台不会被调用，仅为保持包可编译。
func connectedText(Credential) string { return "" }
