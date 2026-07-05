//go:build windows

// Windows 占位实现 of platform interface。
//
// 本 slice(#2 骨架)只放空实现,让 GOOS=windows go build 编译通过。真实
// syscall 调用在后续 slice 填入:
//
//   - ScreenCapturer   → #4(DXGI Desktop Duplication)
//   - AudioCapturer    → #5(WASAPI loopback)
//   - InputInjector    → #6(SendInput)
//   - ClipboardSync    → #7(AddClipboardFormatListener)
//
// 实现期遵循 ADR-0004:纯 Go syscall(`golang.org/x/sys/windows`)+ COM
// vtable 手写,仅在成本不可承受时局部兜底 cgo。
package platform

import "errors"

// ErrNotImplemented 标记 Windows 占位实现尚未填入真实逻辑。
// 本 slice 只保证编译通过;真实 syscall 在后续 slice 引入后此错误消失。
var ErrNotImplemented = errors.New("platform: not implemented yet (skeleton slice #2)")

// NewScreenCapturer 返回一个 Windows ScreenCapturer。
// 真实 DXGI Desktop Duplication 实现见 #4。
func NewScreenCapturer() (ScreenCapturer, error) {
	return nil, ErrNotImplemented
}

// NewAudioCapturer 返回一个 Windows AudioCapturer。
// 真实 WASAPI loopback 实现见 #5。
func NewAudioCapturer() (AudioCapturer, error) {
	return nil, ErrNotImplemented
}

// NewInputInjector 返回一个 Windows InputInjector。
// 真实 SendInput 实现见 #6。
func NewInputInjector() (InputInjector, error) {
	return nil, ErrNotImplemented
}

// NewClipboardSync 返回一个 Windows ClipboardSync。
// 真实 AddClipboardFormatListener 实现见 #7。
func NewClipboardSync() (ClipboardSync, error) {
	return nil, ErrNotImplemented
}
