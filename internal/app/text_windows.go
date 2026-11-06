//go:build windows

package app

import "fmt"

// connectedText 生成界面状态栏的说明文字。
func connectedText(cred Credential) string {
	return fmt.Sprintf("已连接：%s（令牌指纹 %s）", cred.Username, Fingerprint(cred.Token))
}
