package signal

import "testing"

func TestOriginAllowed(t *testing.T) {
	allowed := []string{
		"https://app.example.com",
		"http://localhost:*",
		"http://127.0.0.1:*",
	}

	cases := []struct {
		name   string
		origin string
		want   bool
	}{
		// 精确匹配
		{"exact match allowed", "https://app.example.com", true},

		// localhost 端口通配
		{"localhost wildcard port 5173", "http://localhost:5173", true},
		{"localhost wildcard port 3000", "http://localhost:3000", true},
		{"127.0.0.1 wildcard port", "http://127.0.0.1:8080", true},

		// 拒绝
		{"wrong scheme", "http://app.example.com", false},
		{"wrong host", "https://evil.example.com", false},
		{"evil localhost path traversal", "http://localhost.evil.com:5173", false},
		{"empty origin", "", false},
		{"unrelated origin", "https://other.example.org", false},
		{"file origin", "file:///etc/passwd", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := originAllowed(allowed, tc.origin)
			if got != tc.want {
				t.Fatalf("originAllowed(%q) = %v, want %v", tc.origin, got, tc.want)
			}
		})
	}
}

func TestOriginAllowed_EmptyList(t *testing.T) {
	// 空允许列表:任意 origin 一律拒绝(包括非空 origin)。这是 Handler 不会被
	// 用空列表构造的(TLS 模式 Validate 拦截;insecure 模式 EffectiveOrigins
	// 兜底),但纯函数本身要安全。
	if originAllowed(nil, "https://example.com") {
		t.Fatal("empty allowed list should reject all origins")
	}
	if originAllowed(nil, "") {
		t.Fatal("empty allowed list should reject empty origin")
	}
}
