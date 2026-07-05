package host

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/guoshun0321/remote-desktop/host/signal"
)

// Server 是 Host 的 HTTPS server。本 slice(#3)在 #2 的健康检查 / 冒烟页面
// 基础上,挂载 WebSocket 信令端点 /ws(signal.Handler)。
type Server struct {
	cfg     Config
	srv     *http.Server
	logger  *log.Logger
	signal  *signal.Handler
}

// NewServer 构造一个 Host Server。Config 必须已通过 Validate。
//
// 路由:
//
//	GET  /healthz — 健康检查,返回 200 "ok"
//	GET  /        — 冒烟页面,让浏览器能确认 Host 已就绪
//	WS   /ws      — 信令端点(#3):WebSocket 升级 + 单密码认证 +
//	                SDP offer/answer + trickle ICE(answerer 角色)
func NewServer(cfg Config, logger *log.Logger) (*Server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if logger == nil {
		logger = log.Default()
	}

	signalHandler := signal.NewHandler(
		cfg.Password,
		cfg.EffectiveOrigins(),
		signal.ICEConfig{Mode: cfg.ICECandidates},
		logger,
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 冒烟页面:让浏览器加载后能确认 Host 已就绪。
		// 真实 Client SPA 与 Host 独立部署(CONTEXT.md TLS Topology),
		// 本页面只是占位。
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintln(w, `<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><title>remote-desktop Host</title></head>
<body>
<p>Host ready.</p>
<p>Client SPA is deployed separately; see <code>/healthz</code>.</p>
</body>
</html>`)
	})
	mux.Handle("/ws", signalHandler)

	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: mux,
	}

	return &Server{cfg: cfg, srv: srv, logger: logger, signal: signalHandler}, nil
}

// Start 启动 HTTPS server(或 --insecure 模式下的 HTTP server)。
// 阻塞直至 ListenAndServe 返回(始终返回非 nil error)。
//
// CONTEXT.md TLS Topology:
//   - 默认:HTTPS,信令是 wss://(因前端 HTTPS)。
//   - --insecure:跳过 TLS,仅纯本地开发便利,启动前已打印 INSECURE 警告。
func (s *Server) Start() error {
	if s.cfg.Insecure {
		s.logger.Printf("HOST STARTING on http://%s (INSECURE MODE — plaintext, no TLS)", s.cfg.Addr)
		return s.srv.ListenAndServe()
	}
	s.logger.Printf("HOST STARTING on https://%s", s.cfg.Addr)
	return s.srv.ListenAndServeTLS(s.cfg.CertFile, s.cfg.KeyFile)
}

// Shutdown 优雅关闭 server。
func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}
