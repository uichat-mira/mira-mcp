//go:build windows

package app

import (
	"context"
	"fmt"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procRegisterClassEx  = user32.NewProc("RegisterClassExW")
	procCreateWindowEx   = user32.NewProc("CreateWindowExW")
	procDefWindowProc    = user32.NewProc("DefWindowProcW")
	procShowWindow       = user32.NewProc("ShowWindow")
	procUpdateWindow     = user32.NewProc("UpdateWindow")
	procGetMessage       = user32.NewProc("GetMessageW")
	procTranslateMessage = user32.NewProc("TranslateMessage")
	procDispatchMessage  = user32.NewProc("DispatchMessageW")
	procPostQuitMessage  = user32.NewProc("PostQuitMessage")
	procDestroyWindow    = user32.NewProc("DestroyWindow")
	procSendMessage      = user32.NewProc("SendMessageW")
	procGetWindowText    = user32.NewProc("GetWindowTextW")
	procGetWindowTextLen = user32.NewProc("GetWindowTextLengthW")
	procSetWindowText    = user32.NewProc("SetWindowTextW")
	procEnableWindow     = user32.NewProc("EnableWindow")
	procPostMessage      = user32.NewProc("PostMessageW")
	procSetFocus         = user32.NewProc("SetFocus")
	procCreatePopupMenu  = user32.NewProc("CreatePopupMenu")
	procAppendMenu       = user32.NewProc("AppendMenuW")
	procTrackPopupMenu   = user32.NewProc("TrackPopupMenu")
	procGetCursorPos     = user32.NewProc("GetCursorPos")
	procSetForeground    = user32.NewProc("SetForegroundWindow")
	procLoadCursor       = user32.NewProc("LoadCursorW")
	procLoadIcon         = user32.NewProc("LoadIconW")
	procGetModuleHandle  = kernel32.NewProc("GetModuleHandleW")
	procCreateFont       = gdi32.NewProc("CreateFontW")
	procShellNotifyIcon  = shell32.NewProc("Shell_NotifyIconW")
)

const (
	wsOverlappedWindow = 0x00CF0000
	wsVisible          = 0x10000000
	wsChild            = 0x40000000
	wsTabStop          = 0x00010000
	esAutoHScroll      = 0x0080
	esPassword         = 0x0020

	wmCommand   = 0x0111
	wmClose     = 0x0010
	wmDestroy   = 0x0002
	wmSetFont   = 0x0030
	wmApp       = 0x8000
	wmLButtonUp = 0x0202
	wmRButtonUp = 0x0205

	nifMessage = 0x0001
	nifIcon    = 0x0002
	nifTip     = 0x0004
	nimAdd     = 0x0000

	mfString     = 0x0000
	mfSeparator  = 0x0800
	tpmLeftAlign = 0x0000
	tpmBottom    = 0x0020

	defaultCharset = 1
	fwNormal       = 400

	idiApplication = 32512
	idiArrow       = 32512
	swHide         = 0
	swShow         = 5
)

const (
	ctrlLogin = 101

	msgLoginDone = wmApp + 1
	msgTray      = wmApp + 2
	trayID       = 1
)

type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

type winMsg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ x, y int32 }
}

type winPoint struct{ x, y int32 }

type notifyIconData struct {
	cbSize           uint32
	hWnd             uintptr
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	hIcon            uintptr
	szTip            [128]uint16
	dwState          uint32
	dwStateMask      uint32
	szInfo           [256]uint16
	uVersion         uint32
	szInfoTitle      [64]uint16
	dwInfoFlags      uint32
	guidItem         [16]byte
	hBalloonIcon     uintptr
}

type gui struct {
	hwnd    uintptr
	hUser   uintptr
	hPass   uintptr
	hBtn    uintptr
	hStatus uintptr
	hFont   uintptr
	icon    uintptr

	store *Store

	mu   sync.Mutex
	srv  *Server
	note string
}

var (
	activeGUI  *gui
	wndProcRef uintptr
)

// Run 显示中文登录窗，并在登录成功后启动本机 MCP 端点。
func Run() error {
	g := &gui{}
	activeGUI = g

	store, err := NewStore()
	if err != nil {
		return fmt.Errorf("无法初始化凭据存储：%w", err)
	}
	g.store = store

	if err := g.createWindow(); err != nil {
		return err
	}
	g.restore()
	g.messageLoop()

	g.mu.Lock()
	srv := g.srv
	g.mu.Unlock()
	if srv != nil {
		srv.Stop()
	}
	return nil
}

func (g *gui) createWindow() error {
	hInstance, _, _ := procGetModuleHandle.Call(0)

	// CreateFontW 的高度是有符号值，必须经由变量转换，否则常量 -14 转 uintptr 会溢出。
	height := int32(-14)
	g.hFont, _, _ = procCreateFont.Call(
		uintptr(height), 0, 0, 0, fwNormal, 0, 0, 0,
		defaultCharset, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(utf16ptr("Microsoft YaHei UI"))),
	)

	cursor, _, _ := procLoadCursor.Call(0, idiArrow)
	icon, _, _ := procLoadIcon.Call(0, idiApplication)
	g.icon = icon

	if wndProcRef == 0 {
		wndProcRef = syscall.NewCallback(wndProc)
	}

	className := utf16ptr("MiraMCPLoginWindow")
	wc := wndClassEx{
		style:         0x0003,
		lpfnWndProc:   wndProcRef,
		hInstance:     hInstance,
		hIcon:         icon,
		hCursor:       cursor,
		hbrBackground: 16,
		lpszClassName: className,
		hIconSm:       icon,
	}
	wc.cbSize = uint32(unsafe.Sizeof(wc))

	if ret, _, err := procRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc))); ret == 0 {
		return fmt.Errorf("无法注册窗口类：%v", err)
	}

	hwnd, _, err := procCreateWindowEx.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(utf16ptr("Mira 移动端连接"))),
		wsOverlappedWindow,
		240, 180,
		440, 320,
		0, 0, hInstance, 0,
	)
	if hwnd == 0 {
		return fmt.Errorf("无法创建窗口：%v", err)
	}
	g.hwnd = hwnd

	g.label("账号", 24, 20)
	g.hUser = g.control("EDIT", "", 0, 24, 44, 376, 28,
		wsChild|wsVisible|wsTabStop|esAutoHScroll)
	g.label("密码", 24, 80)
	g.hPass = g.control("EDIT", "", 0, 24, 104, 376, 28,
		wsChild|wsVisible|wsTabStop|esAutoHScroll|esPassword)
	g.hBtn = g.control("BUTTON", "登录并连接", ctrlLogin, 24, 148, 376, 34,
		wsChild|wsVisible|wsTabStop)
	g.hStatus = g.control("STATIC", "未连接", 0, 24, 196, 376, 76, wsChild|wsVisible)

	procShowWindow.Call(hwnd, swShow)
	procUpdateWindow.Call(hwnd)
	procSetFocus.Call(g.hUser)
	return nil
}

func (g *gui) label(text string, x, y int) {
	g.control("STATIC", text, 0, x, y, 376, 20, wsChild|wsVisible)
}

// control 创建子控件。id 作为 hMenu 传入，按钮点击时才会带出正确的控件 ID。
func (g *gui) control(class, text string, id, x, y, w, h int, style uintptr) uintptr {
	hwnd, _, _ := procCreateWindowEx.Call(
		0,
		uintptr(unsafe.Pointer(utf16ptr(class))),
		uintptr(unsafe.Pointer(utf16ptr(text))),
		style,
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		g.hwnd, uintptr(id), 0, 0,
	)
	if hwnd != 0 && g.hFont != 0 {
		procSendMessage.Call(hwnd, wmSetFont, g.hFont, 1)
	}
	return hwnd
}

// restore 复用已保存的凭据，避免每次开机都要重新输入密码。
func (g *gui) restore() {
	cred, err := g.store.Load()
	if err != nil {
		g.setStatus("读取已保存登录信息失败：" + err.Error())
		return
	}
	if cred == nil {
		return
	}
	g.setStatus(connectedText(*cred) + "\n本机端点：" + MCPEndpoint())
	g.startServer(cred)
}

func (g *gui) login() {
	username := g.text(g.hUser)
	password := g.text(g.hPass)

	procEnableWindow.Call(g.hBtn, 0)
	g.setStatus("正在登录…")

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		result, err := Login(ctx, CloudBaseURL, username, password)
		if err != nil {
			g.setNote("登录失败：" + err.Error())
			procPostMessage.Call(g.hwnd, msgLoginDone, 0, 0)
			return
		}

		cred := Credential{Token: result.Token, Username: result.Username, Host: CloudBaseURL}
		if err := g.store.Save(cred); err != nil {
			g.setNote("登录成功，但保存凭据失败：" + err.Error())
			procPostMessage.Call(g.hwnd, msgLoginDone, 0, 0)
			return
		}

		g.startServer(&cred)
		g.setNote(connectedText(cred) + "\n本机端点：" + MCPEndpoint())
		procPostMessage.Call(g.hwnd, msgLoginDone, 0, 0)
	}()
}

func (g *gui) startServer(cred *Credential) {
	g.mu.Lock()
	if g.srv != nil {
		g.srv.SetUser(cred.Username)
		g.mu.Unlock()
		return
	}
	g.mu.Unlock()

	secret, err := NewLocalSecret()
	if err != nil {
		g.setStatus("无法生成本机访问口令：" + err.Error())
		return
	}
	srv := NewServer(secret, cred.Username)
	if err := srv.Start(); err != nil {
		g.setStatus(err.Error())
		return
	}

	g.mu.Lock()
	g.srv = srv
	g.mu.Unlock()
}

func (g *gui) setNote(text string) {
	g.mu.Lock()
	g.note = text
	g.mu.Unlock()
}

func (g *gui) applyNote() {
	procEnableWindow.Call(g.hBtn, 1)
	g.mu.Lock()
	note := g.note
	g.note = ""
	g.mu.Unlock()
	if note != "" {
		g.setStatus(note)
	}
}

func (g *gui) text(hwnd uintptr) string {
	n, _, _ := procGetWindowTextLen.Call(hwnd)
	if n == 0 {
		return ""
	}
	buf := make([]uint16, n+1)
	procGetWindowText.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), n+1)
	return syscall.UTF16ToString(buf)
}

func (g *gui) setStatus(text string) {
	procSetWindowText.Call(g.hStatus, uintptr(unsafe.Pointer(utf16ptr(text))))
}

func (g *gui) addTray() {
	if g.hwnd == 0 {
		return
	}
	var nid notifyIconData
	nid.cbSize = uint32(unsafe.Sizeof(nid))
	nid.hWnd = g.hwnd
	nid.uID = trayID
	nid.uFlags = nifMessage | nifIcon | nifTip
	nid.uCallbackMessage = msgTray
	nid.hIcon = g.icon
	copy(nid.szTip[:], utf16slice("Mira 移动端连接"))
	procShellNotifyIcon.Call(nimAdd, uintptr(unsafe.Pointer(&nid)))
}

func (g *gui) showTrayMenu() {
	menu, _, _ := procCreatePopupMenu.Call()
	procAppendMenu.Call(menu, mfString, 1, uintptr(unsafe.Pointer(utf16ptr("显示登录窗口"))))
	procAppendMenu.Call(menu, mfSeparator, 0, 0)
	procAppendMenu.Call(menu, mfString, 2, uintptr(unsafe.Pointer(utf16ptr("退出"))))

	var pt winPoint
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	procSetForeground.Call(g.hwnd)
	cmd, _, _ := procTrackPopupMenu.Call(
		menu, tpmLeftAlign|tpmBottom,
		uintptr(pt.x), uintptr(pt.y), 0, g.hwnd, 0,
	)
	switch cmd {
	case 1:
		procShowWindow.Call(g.hwnd, swShow)
	case 2:
		procDestroyWindow.Call(g.hwnd)
	}
}

func (g *gui) messageLoop() {
	var msg winMsg
	for {
		ret, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(ret) <= 0 {
			return
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func wndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	g := activeGUI
	if g == nil {
		ret, _, _ := procDefWindowProc.Call(hwnd, msg, wParam, lParam)
		return ret
	}

	switch msg {
	case wmCommand:
		if uint16(wParam) == ctrlLogin && uint16(wParam>>16) == 0 {
			g.login()
			return 0
		}
	case msgLoginDone:
		g.applyNote()
		return 0
	case msgTray:
		switch lParam {
		case wmLButtonUp:
			procShowWindow.Call(hwnd, swShow)
			procSetForeground.Call(hwnd)
		case wmRButtonUp:
			g.showTrayMenu()
		}
		return 0
	case wmClose:
		// 关闭窗口不退出进程：收进托盘，保持本机 MCP 端点可用。
		procShowWindow.Call(hwnd, swHide)
		g.addTray()
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}

	ret, _, _ := procDefWindowProc.Call(hwnd, msg, wParam, lParam)
	return ret
}

func utf16ptr(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		empty, _ := syscall.UTF16PtrFromString("")
		return empty
	}
	return p
}

func utf16slice(s string) []uint16 {
	out, err := syscall.UTF16FromString(s)
	if err != nil {
		return []uint16{}
	}
	return out
}
