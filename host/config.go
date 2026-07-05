// Package host 实现 Host 程序的服务端逻辑:HTTPS server、健康检查、(后续
// slice)WebSocket 信令端点与 WebRTC 媒体端。
//
// Host 在同一个 HTTPS server 上提供信令(因前端 HTTPS,信令必须是 wss://,
// 见 CONTEXT.md TLS Topology)。本 slice(#2 骨架)只铺 HTTPS server 与
// 健康检查端点;信令端点在 #3 引入。
package host

import "errors"

// Config 是 Host 启动配置,由命令行 flag 解析而来(见 cmd/host/main.go)。
type Config struct {
	// Addr 是 HTTPS server 监听地址(如 ":https" 或 ":443")。
	Addr string

	// CertFile / KeyFile 是 TLS 证书路径。Host 不自动生成证书(CONTEXT.md
	// TLS Topology):缺失则报错退出,用户须用 mkcert / openssl 等自备。
	// 仅当 Insecure=true 时可省略。
	CertFile string
	KeyFile  string

	// Insecure 跳过 TLS,仅纯 localhost 开发便利。代价(见 CONTEXT.md):
	//   - WS 与密码首帧明文传输,同网段任何人可嗅探。
	//   - 非 localhost 访问时 navigator.clipboard 因 Secure Context 不满足
	//     而不可用 → 剪贴板同步自动禁用(#7 通过会话元数据告知 Client 降级)。
	//   - 启动时打印显眼 INSECURE MODE 警告。
	Insecure bool

	// ICECandidates 控制 ICE 候选收集模式(ADR-0002)。
	//   - "host"(默认,首期):只收集 host candidate,要求同子网。
	//   - "stun" / "turn":后续放开(本 slice 不消费,仅占位)。
	ICECandidates string
}

// Validate 校验 Config 在当前 flag 组合下是否自洽。
//
// 规则(CONTEXT.md TLS Topology + --insecure mode):
//   - 非 Insecure 模式:CertFile 与 KeyFile 都必须提供。
//   - Insecure 模式:TLS 证书可省略。
//   - Addr 必须非空。
func (c Config) Validate() error {
	if c.Addr == "" {
		return errors.New("host: Addr must be set")
	}
	if !c.Insecure {
		if c.CertFile == "" {
			return errors.New("host: --cert required (or pass --insecure for plaintext local-only dev)")
		}
		if c.KeyFile == "" {
			return errors.New("host: --key required (or pass --insecure for plaintext local-only dev)")
		}
	}
	if c.ICECandidates == "" {
		return errors.New("host: ICECandidates must be set (default \"host\")")
	}
	return nil
}
