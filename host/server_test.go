package host

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 本 slice(#2 骨架)的 host 包只有 Config 校验 + 健康检查端点是 Linux 上
// 可测的真实外部行为(信令/媒体/输入都在后续 slice)。这里测这两块。
// 平台相关测试(screen capture / input inject)是 Windows-only,在 Windows
// runner 上跑(见 ADR-0004 + PRD #1 Testing Decisions)。

func TestConfig_Validate(t *testing.T) {
	t.Run("TLS mode requires both cert and key", func(t *testing.T) {
		cases := []struct {
			name string
			cfg  Config
			want string // 期望错误子串
		}{
			{
				name: "missing addr",
				cfg:  Config{CertFile: "c", KeyFile: "k", ICECandidates: "host"},
				want: "Addr must be set",
			},
			{
				name: "tls mode missing cert",
				cfg:  Config{Addr: ":443", KeyFile: "k", ICECandidates: "host"},
				want: "--cert required",
			},
			{
				name: "tls mode missing key",
				cfg:  Config{Addr: ":443", CertFile: "c", ICECandidates: "host"},
				want: "--key required",
			},
			{
				name: "missing ice candidates default",
				cfg:  Config{Addr: ":443", CertFile: "c", KeyFile: "k", ICECandidates: ""},
				want: "ICECandidates must be set",
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				err := tc.cfg.Validate()
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.want)
				}
				if !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("expected error containing %q, got %q", tc.want, err.Error())
				}
			})
		}
	})

	t.Run("insecure mode does not require cert/key", func(t *testing.T) {
		cfg := Config{Addr: ":8080", Insecure: true, ICECandidates: "host"}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("insecure mode should not require cert/key, got error: %v", err)
		}
	})

	t.Run("valid TLS config passes", func(t *testing.T) {
		cfg := Config{Addr: ":443", CertFile: "cert.pem", KeyFile: "key.pem", ICECandidates: "host"}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("valid TLS config should pass, got error: %v", err)
		}
	})
}

// TestServer_HealthEndpoint 启动真实的 httptest server 验证健康检查端点。
// 这是骨架 slice 在 Linux 上可测的最高 seam:Config -> NewServer -> mux 路由 ->
// HTTP 请求/响应,全程真实无 mock。
func TestServer_HealthEndpoint(t *testing.T) {
	// Insecure 配置让 NewServer 不卡 TLS 校验;httptest 起的是 HTTP server。
	cfg := Config{Addr: "127.0.0.1:0", Insecure: true, ICECandidates: "host"}
	srv, err := NewServer(cfg, nil)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	ts := httptest.NewServer(srv.srv.Handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if strings.TrimSpace(string(body)) != "ok" {
		t.Fatalf("body = %q, want %q", body, "ok")
	}
}

// TestServer_SmokePage 验证冒烟页面返回 200 + HTML。
func TestServer_SmokePage(t *testing.T) {
	cfg := Config{Addr: "127.0.0.1:0", Insecure: true, ICECandidates: "host"}
	srv, err := NewServer(cfg, nil)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	ts := httptest.NewServer(srv.srv.Handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html*", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Host ready") {
		t.Fatalf("body does not contain 'Host ready': %q", body)
	}
}

// TestNewServer_RejectsInvalidConfig 验证 NewServer 在 Config 校验失败时返回错误。
func TestNewServer_RejectsInvalidConfig(t *testing.T) {
	// TLS mode without cert/key should fail at NewServer.
	cfg := Config{Addr: ":443", ICECandidates: "host"} // missing cert+key, not insecure
	_, err := NewServer(cfg, nil)
	if err == nil {
		t.Fatal("expected error for invalid config, got nil")
	}
}
