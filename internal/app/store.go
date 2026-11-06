package app

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Credential 是登录成功后落盘的最小信息。
type Credential struct {
	Token    string
	Username string
	Host     string
}

// Store 用平台加密能力保存凭据，避免明文 token 落在磁盘上。
type Store struct {
	Path string
}

// NewStore 返回默认凭据路径：<UserConfigDir>/MiraMCP/credentials.bin
func NewStore() (*Store, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	full := filepath.Join(dir, "MiraMCP")
	if err := os.MkdirAll(full, 0o700); err != nil {
		return nil, err
	}
	return &Store{Path: filepath.Join(full, "credentials.bin")}, nil
}

// Save 加密写入凭据。
func (s *Store) Save(cred Credential) error {
	if cred.Token == "" {
		return errors.New("拒绝保存空 token")
	}
	// 每行一个字段；机密性由平台加密保证，格式本身不承载安全职责。
	plain := []byte("v1\n" + cred.Host + "\n" + cred.Username + "\n" + cred.Token)
	sealed, err := protect(plain)
	if err != nil {
		return err
	}
	return os.WriteFile(s.Path, sealed, 0o600)
}

// Load 读取并解密凭据；文件不存在时返回 nil, nil。
func (s *Store) Load() (*Credential, error) {
	sealed, err := os.ReadFile(s.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	plain, err := unprotect(sealed)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(string(plain), "\n")
	if len(parts) < 4 || parts[0] != "v1" {
		return nil, errors.New("凭据文件格式不正确")
	}
	return &Credential{Token: parts[3], Username: parts[2], Host: parts[1]}, nil
}

// Fingerprint 返回 token 的短指纹，用于界面展示而不泄露原文。
func Fingerprint(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])[:8]
}

// NewLocalSecret 生成本机 MCP 端点的一次性访问口令。
func NewLocalSecret() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
