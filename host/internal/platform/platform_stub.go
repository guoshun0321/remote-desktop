//go:build !windows

// Stub 实现 of platform interface,供 Linux/WSL 上 `go test ./...` 跑通
// 纯逻辑(见 ADR-0004:WSL 写代码 → go test 验证 → Windows runner 跑真机)。
//
// 这些 stub 在 Linux 上无副作用、可空跑;真正会被 exercise 的是后续 slice
// 引入的测试 seam(Seam 2 输入、Seam 3 剪贴板 —— 它们用 fake 而非这里
// 的 stub,但这里的 stub 保证 platform 包整体在 Linux 上编译通过)。
package platform

import "errors"

// ErrStubPlatform 表示当前平台无真实实现(非 Windows)。
// 这是预期的:在 Linux/WSL 上只能跑 stub 路径,真机验证在 Windows runner。
var ErrStubPlatform = errors.New("platform: stub on non-windows (real impl is windows-only)")

// NewScreenCapturer 返回 stub ScreenCapturer(Linux 上不可用)。
// 真实 DXGI 实现是 Windows-only,见 #4。
func NewScreenCapturer() (ScreenCapturer, error) {
	return nil, ErrStubPlatform
}

// NewAudioCapturer 返回 stub AudioCapturer(Linux 上不可用)。
// 真实 WASAPI 实现是 Windows-only,见 #5。
func NewAudioCapturer() (AudioCapturer, error) {
	return nil, ErrStubPlatform
}

// NewInputInjector 返回 stub InputInjector(Linux 上不可用)。
// 真实 SendInput 实现是 Windows-only,见 #6。Seam 2 测试用 fake,不用此 stub。
func NewInputInjector() (InputInjector, error) {
	return nil, ErrStubPlatform
}

// NewClipboardSync 返回 stub ClipboardSync(Linux 上不可用)。
// 真实 AddClipboardFormatListener 实现是 Windows-only,见 #7。
// Seam 3 测试用 fake,不用此 stub。
func NewClipboardSync() (ClipboardSync, error) {
	return nil, ErrStubPlatform
}
