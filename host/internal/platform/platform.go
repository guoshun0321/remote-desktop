// Package platform defines the Host's platform-abstraction seams.
//
// 首期 Host 目标是 Windows,需要调用大量 Win32 / COM API:DXGI Desktop
// Duplication(屏幕捕获)、MediaFoundation(H.264 编码)、WASAPI(音频)、
// SendInput(键鼠)、AddClipboardFormatListener(剪贴板)。
//
// 这些 API 只在 Windows 真机可达;而本项目的开发机是 WSL2 Linux,无法本地
// 运行 Windows 二进制(见 ADR-0004)。为了让 Linux 上 `go test ./...` 至少
// 跑通纯逻辑,平台依赖从第一天就用 interface 抽象,实现按 build tags 隔离:
//
//   - platform_windows.go  — Windows 占位实现(后续 slice 填真实 syscall 调用)
//   - platform_stub.go     — !windows 占位实现(Linux/WSL 上 go test 用)
//
// CONTEXT.md 第二期提到"interface 抽象 ScreenCapturer / AudioCapturer /
// InputInjector / ClipboardSync"—— 本 slice 把它从第二期提前到首期,目的
// 就是给 Linux-runnable 测试 seam 让路(见 PRD #1 Testing Decisions)。
package platform

// ScreenCapturer 抓取主屏帧。
//
// 首期 Windows 实现:DXGI Desktop Duplication(见 ADR-0004)。
// 首期只主屏(CONTEXT.md Out of scope: 多显示器)。
type ScreenCapturer interface {
	// Capture 抓取一帧。返回的帧后续会喂给 H.264 编码器。
	// 具体帧格式在 #4(视频 slice)敲定,本 slice 仅定义 seam。
	Capture() (Frame, error)

	// Close 释放捕获资源。
	Close() error
}

// Frame 是一帧屏幕画面。具体字段在 #4 敲定(DXGI Output Duplication 的
// texture / NV12 buffer 形态)。本 slice 留空占位。
type Frame struct{}

// AudioCapturer 采集系统桌面音频混音输出。
//
// 首期 Windows 实现:WASAPI loopback。
type AudioCapturer interface {
	// Capture 读取一段音频 PCM。后续 Opus 编码消费。
	Capture() (AudioChunk, error)

	Close() error
}

// AudioChunk 是一段音频 PCM。具体字段在 #5(音频 slice)敲定。
type AudioChunk struct{}

// InputEvent 是一条 Client → Host 键鼠输入。完整 MessagePack shape 在 #6
// (输入 slice)引入(含 sid、归一化坐标、KeyboardEvent.code)。本 slice
// 仅占位,让 InputInjector interface 成形。
type InputEvent struct{}

// InputInjector 把键鼠事件注入本地系统。
//
// 首期 Windows 实现:SendInput。在 Linux 上由 fake 实现替换,记录将被注入
// 的目标(供 Seam 2 测试断言反归一化结果,见 PRD #1 Testing Decisions)。
type InputInjector interface {
	// Inject 注入一条输入事件。
	Inject(InputEvent) error

	Close() error
}

// ClipboardContent 是一次剪贴板同步内容。完整 shape 在 #7(剪贴板 slice)
// 引入。本 slice 仅占位。
type ClipboardContent struct{}

// ClipboardSync 双向同步 Host 与 Client 的剪贴板。
//
// 首期 Windows 实现:AddClipboardFormatListener。在 Linux 上由 fake 实现
// 替换,供 Seam 3 测试用。
type ClipboardSync interface {
	// Read 读取本地剪贴板当前内容(用于推送给远端)。
	Read() (ClipboardContent, error)

	// Write 把远端推送来的内容写入本地剪贴板。
	Write(ClipboardContent) error

	Close() error
}
