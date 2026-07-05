package host

import (
	"context"
	"fmt"
	"log"
	"net/http"
)

// Server 是 Host 的 HTTPS server。本 slice(#2 骨架)只提供健康检查端点
// 与(可选)冒烟页面;WebSocket 信令端点与 WebRTC 媒体端在后续 slice 加入。
type Server struct {
	cfg    Config
	srv    *http.Server
	logger *log.Logger
}

// NewServer 构造一个 Host Server。Config 必须已通过 Validate。
//
// 路由(本 slice):
//
//	GET /healthz   — 健康检查,返回 200 "ok"
//	GET /          — 冒烟页面,让浏览器能确认 Host 已就绪
//
// 后续 slice 会在同一个 mux 上挂 /ws(信令端点,见 #3)。
func NewServer(cfg Config, logger *log.Logger) (*Server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if logger == nil {
		logger = log.Default()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 冒烟页面:让浏览器加载后能确认 Host 已就绪。
		// 真实 Client SPA 与 Host 独立部署(CONTEXT.md TLS Topology),
		// 本页面只是骨架 slice 的占位。
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

	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: mux,
	}

	return &Server{cfg: cfg, srv: srv, logger: logger}, nil
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
