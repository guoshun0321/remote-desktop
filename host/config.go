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
	//   - "stun" / "turn":后续放开(本 slice 消费 host,stun/turn 留给后续)。
	ICECandidates string

	// Password 是 WebSocket 握手首帧校验的单密码(CONTEXT.md 单密码认证)。
	// TLS 与 --insecure 模式都必填 —— 认证不依赖 TLS。--insecure 下明文可嗅探
	// 是已记录的代价,不是放空密码的理由。
	Password string

	// AllowedOrigins 是 WebSocket 握手 Signal Origin Check 的允许来源白名单
	// (CONTEXT.md Signal Origin Check)。每项可以是精确 origin(如
	// "https://app.example.com")或带 "*" 端口通配(如 "http://localhost:*")。
	//
	//   - TLS 模式:必填(防止任意网页连入 Host)。
	//   - --insecure 模式:可省略,省略时 EffectiveOrigins() 返回 localhost 默认值。
	AllowedOrigins []string
}

// Validate 校验 Config 在当前 flag 组合下是否自洽。
//
// 规则(CONTEXT.md TLS Topology + --insecure mode + 单密码认证 +
// Signal Origin Check):
//   - 非 Insecure 模式:CertFile 与 KeyFile 都必须提供。
//   - Insecure 模式:TLS 证书可省略。
//   - Addr 必须非空。
//   - Password 必填(认证不依赖 TLS)。
//   - TLS 模式下 AllowedOrigins 必填(insecure 模式可省略,EffectiveOrigins 兜底)。
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
		if len(c.AllowedOrigins) == 0 {
			return errors.New("host: --origin required in TLS mode (Signal Origin Check; pass --insecure to default to localhost)")
		}
	}
	if c.ICECandidates == "" {
		return errors.New("host: ICECandidates must be set (default \"host\")")
	}
	// ADR-0002 的承诺是"放开 STUN/TURN 只改配置不改代码"。但 stun/turn 的
	// 真实实现(ICEServers 注入)在后续 slice。在落地前,显式拒绝而非静默
	// 退化为 host —— 否则用户改了配置以为生效,实则没。
	switch c.ICECandidates {
	case "host":
		// 首期唯一支持的模式。
	case "stun", "turn":
		return errors.New("host: ICECandidates \"" + c.ICECandidates + "\" not implemented yet (only \"host\" in first phase; see ADR-0002)")
	default:
		return errors.New("host: ICECandidates must be \"host\" | \"stun\" | \"turn\" (got \"" + c.ICECandidates + "\")")
	}
	if c.Password == "" {
		return errors.New("host: --password required (single-password auth, independent of TLS)")
	}
	return nil
}

// EffectiveOrigins 返回 Signal Origin Check 实际生效的来源白名单。
//
// 显式配置优先;--insecure 模式下未配置则用 localhost 默认值(开发便利,
// CONTEXT.md --insecure mode 的"纯本地开发"语义)。TLS 模式下未配置应由
// Validate 拦下,本方法仍兜底返回 localhost 以防误调用。
func (c Config) EffectiveOrigins() []string {
	if len(c.AllowedOrigins) > 0 {
		return c.AllowedOrigins
	}
	return []string{
		"http://localhost:*",
		"http://127.0.0.1:*",
		"http://[::1]:*",
	}
}
