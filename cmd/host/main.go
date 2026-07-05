// Command host 启动 remote-desktop 的 Host 程序(被控端)。
//
// Host 捕获桌面、音频并接收键鼠 / 剪贴板输入,经 WebRTC 推送给 Client(见
// CONTEXT.md)。本 slice(#2 骨架)只启动 HTTPS server 与健康检查端点;
// 信令、媒体、输入逻辑在后续 slice(#3 起)加入。
//
// 用法:
//
//	host --addr :443 --cert cert.pem --key key.pem
//	host --addr :8080 --insecure      # 仅纯本地开发
//
// 退出码:
//
//	0  正常关闭
//	1  flag 解析失败 / Config 校验失败 / server 启动错误
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/guoshun0321/remote-desktop/host"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("host", flag.ContinueOnError)
	addr := fs.String("addr", ":443", "HTTPS server listen address")
	cert := fs.String("cert", "", "TLS certificate path (required unless --insecure)")
	key := fs.String("key", "", "TLS key path (required unless --insecure)")
	insecure := fs.Bool("insecure", false, "skip TLS — plaintext local-only dev (prints INSECURE warning)")
	iceCandidates := fs.String("ice.candidates", "host", `ICE candidate mode: "host" (default, LAN-only) | "stun" | "turn"`)

	if err := fs.Parse(args); err != nil {
		fs.Usage()
		return 1
	}

	cfg := host.Config{
		Addr:          *addr,
		CertFile:      *cert,
		KeyFile:       *key,
		Insecure:      *insecure,
		ICECandidates: *iceCandidates,
	}

	// --insecure 必须在启动前打印显眼警告(CONTEXT.md --insecure mode)。
	if cfg.Insecure {
		printInsecureWarning()
	}

	srv, err := host.NewServer(cfg, log.Default())
	if err != nil {
		fmt.Fprintf(os.Stderr, "host: invalid config: %v\n", err)
		fs.Usage()
		return 1
	}

	// 监听 SIGINT/SIGTERM 优雅关闭。
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Printf("received %s, shutting down...", sig)
		_ = srv.Shutdown(context.Background())
	}()

	if err := srv.Start(); err != nil {
		// http.ErrServerClosed 是 Shutdown 触发的正常退出,不视为错误。
		if errors.Is(err, http.ErrServerClosed) {
			return 0
		}
		fmt.Fprintf(os.Stderr, "host: server error: %v\n", err)
		return 1
	}
	return 0
}

// printInsecureWarning 打印显眼 INSECURE MODE 警告(CONTEXT.md --insecure mode)。
func printInsecureWarning() {
	const banner = `
!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
!! INSECURE MODE — TLS is DISABLED
!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
!! - WS handshake & password first-frame are sent in PLAINTEXT.
!!   Anyone on the same network segment can sniff them.
!! - Non-localhost access: navigator.clipboard is UNAVAILABLE
!!   (Secure Context not satisfied) -> clipboard sync auto-disabled.
!! Use cases: localhost-only local debugging ONLY.
!! For any real cross-machine deployment, use TLS (--cert/--key).
!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
`
	fmt.Fprint(os.Stderr, banner)
}
