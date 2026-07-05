package signal

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
)

// 这些测试覆盖 #3 的核心信令契约(Seam 1 的 Go in-process 版,Playwright e2e
// 留作 follow-up)。所有测试都用真实的 gorilla/websocket + 真实 pion,在同机
// loopback 上跑真实 ICE(host candidate 直连),不用 mock。

const (
	testPassword = "s3cret"
	testOrigin   = "http://localhost:5173"
)

// newTestServer 起一个 httptest server,挂 signal.Handler,返回 server 与
// ws:// URL。origin 默认放 localhost。
func newTestServer(t *testing.T, allowedOrigins []string) (*httptest.Server, *Handler) {
	t.Helper()
	if allowedOrigins == nil {
		allowedOrigins = []string{testOrigin, "http://127.0.0.1:*"}
	}
	h := NewHandler(testPassword, allowedOrigins, ICEConfig{Mode: "host"}, nil)
	ts := httptest.NewServer(h)
	return ts, h
}

// dialWS 打开一条 WS,带指定 Origin header(模拟浏览器跨域 WS 握手)。
func dialWS(t *testing.T, url, origin string) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	d := websocket.Dialer{}
	return d.Dial(url, http.Header{"Origin": {origin}})
}

// --- Origin 校验 ---

func TestHandler_Origin_Rejects(t *testing.T) {
	ts, _ := newTestServer(t, []string{"https://allowed.example.com"})
	defer ts.Close()
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"

	conn, resp, err := dialWS(t, wsURL, "https://evil.example.com")
	if err == nil {
		conn.Close()
		t.Fatal("expected upgrade to be rejected for disallowed origin")
	}
	if resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got resp=%v", resp)
	}
}

func TestHandler_Origin_Allows(t *testing.T) {
	ts, _ := newTestServer(t, nil)
	defer ts.Close()
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"

	conn, _, err := dialWS(t, wsURL, testOrigin)
	if err != nil {
		t.Fatalf("expected upgrade to succeed for allowed origin, got: %v", err)
	}
	conn.Close()
}

// --- 密码认证 ---

func TestHandler_Password_Rejects(t *testing.T) {
	ts, _ := newTestServer(t, nil)
	defer ts.Close()
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"

	conn, _, err := dialWS(t, wsURL, testOrigin)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// 发错密码。
	bad, _ := writeFrame(Frame{Type: FrameTypeAuth, Password: "wrong"})
	if err := conn.WriteMessage(websocket.TextMessage, bad); err != nil {
		t.Fatalf("write auth: %v", err)
	}

	// 应收 error 帧 + close。
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read error frame: %v", err)
	}
	f, err := readFrame(data)
	if err != nil {
		t.Fatalf("parse frame: %v", err)
	}
	if f.Type != FrameTypeError {
		t.Fatalf("expected error frame, got %q", f.Type)
	}
}

func TestHandler_Password_Accepts(t *testing.T) {
	ts, _ := newTestServer(t, nil)
	defer ts.Close()
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"

	conn, _, err := dialWS(t, wsURL, testOrigin)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// 发正确密码。
	auth, _ := writeFrame(Frame{Type: FrameTypeAuth, Password: testPassword})
	if err := conn.WriteMessage(websocket.TextMessage, auth); err != nil {
		t.Fatalf("write auth: %v", err)
	}

	// 应收 auth_ok,sessionId > 0。
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read auth_ok: %v", err)
	}
	f, err := readFrame(data)
	if err != nil {
		t.Fatalf("parse frame: %v", err)
	}
	if f.Type != FrameTypeAuthOK {
		t.Fatalf("expected auth_ok, got %q", f.Type)
	}
	if f.SessionID <= 0 {
		t.Fatalf("expected positive sessionId, got %d", f.SessionID)
	}
}

func TestHandler_SessionID_Monotonic(t *testing.T) {
	// 两次连接的 sid 应单调递增(为 #8 抢占的 sid 比对铺路)。
	ts, h := newTestServer(t, nil)
	defer ts.Close()
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"

	sid1 := authAndGetSID(t, wsURL)
	sid2 := authAndGetSID(t, wsURL)
	if sid2 <= sid1 {
		t.Fatalf("expected sid2 > sid1 (monotonic), got sid1=%d sid2=%d", sid1, sid2)
	}
	_ = h // registry 内部已验证
}

// authAndGetSID 完成 WS 握手 + 认证,返回分配的 sid。
func authAndGetSID(t *testing.T, wsURL string) int64 {
	t.Helper()
	conn, _, err := dialWS(t, wsURL, testOrigin)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	auth, _ := writeFrame(Frame{Type: FrameTypeAuth, Password: testPassword})
	if err := conn.WriteMessage(websocket.TextMessage, auth); err != nil {
		t.Fatalf("write auth: %v", err)
	}
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read auth_ok: %v", err)
	}
	f, _ := readFrame(data)
	if f.Type != FrameTypeAuthOK {
		t.Fatalf("expected auth_ok, got %q", f.Type)
	}
	return f.SessionID
}

// --- Seam 1 核心:SDP/ICE 端到端,断言 ICE connected ---

// TestHandler_SDP_RoundTrip 是 #3 的决定性测试。
//
// 测试侧用一个真实 pion PC 当作 "Client"(offerer),模拟浏览器:
//  1. 起 Host(insecure httptest server)
//  2. WS 握手 + auth → auth_ok
//  3. 测试 PC CreateOffer → 发 offer 帧
//  4. 收 Host answer → SetRemoteDescription
//  5. 双向 trickle candidate 交换
//  6. 断言测试 PC 的 ICEConnectionState 到达 connected(超时 30s)
//
// 这是真 WS + 真 pion + 真 ICE(同机 loopback,host candidate 直连)。
func TestHandler_SDP_RoundTrip(t *testing.T) {
	// ICE 在纯 loopback 上偶有抖动;给宽松超时。CI 上 host candidate 直连
	// 通常 <5s,30s 是安全余量。
	const iceTimeout = 30 * time.Second

	ts, _ := newTestServer(t, nil)
	defer ts.Close()
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"

	// 测试侧 Client PC(offerer)。空 ICEServers + 默认 All = host-only(ADR-0002)。
	clientPC, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatalf("client PC: %v", err)
	}
	defer clientPC.Close()

	// 需要一个 media/datachannel 让 ICE 真正协商 —— 否则 pion 可能不收集
	// 候选(无 transceiver)。#3 不开 datachannel,但测试需要一个触发器。
	// 用一个未发送的 transceiver 足以让 SDP 含 m-line,从而驱动 ICE。
	if _, err := clientPC.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo); err != nil {
		t.Fatalf("add transceiver: %v", err)
	}

	// WS 连接 + auth。
	conn, _, err := dialWS(t, wsURL, testOrigin)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	auth, _ := writeFrame(Frame{Type: FrameTypeAuth, Password: testPassword})
	if err := conn.WriteMessage(websocket.TextMessage, auth); err != nil {
		t.Fatalf("write auth: %v", err)
	}
	if _, data, err := conn.ReadMessage(); err != nil {
		t.Fatalf("read auth_ok: %v", err)
	} else if f, _ := readFrame(data); f.Type != FrameTypeAuthOK {
		t.Fatalf("expected auth_ok, got %q", f.Type)
	}

	// 测试 PC 候选 → 推给 Host(trickle out)。
	clientPC.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}
		raw, _ := json.Marshal(c.ToJSON())
		f, _ := writeFrame(Frame{Type: FrameTypeCandidate, Candidate: string(raw)})
		_ = conn.WriteMessage(websocket.TextMessage, f)
	})

	// 创建 offer 并发送。
	offer, err := clientPC.CreateOffer(nil)
	if err != nil {
		t.Fatalf("create offer: %v", err)
	}
	if err := clientPC.SetLocalDescription(offer); err != nil {
		t.Fatalf("set local desc: %v", err)
	}
	offerJSON, _ := json.Marshal(map[string]string{
		"type": offer.Type.String(),
		"sdp":  offer.SDP,
	})
	of, _ := writeFrame(Frame{Type: FrameTypeOffer, SDP: string(offerJSON)})
	if err := conn.WriteMessage(websocket.TextMessage, of); err != nil {
		t.Fatalf("write offer: %v", err)
	}

	// 读 Host 来帧(answer / candidate),驱动测试 PC 到 connected。
	connectedCh := make(chan struct{})
	clientPC.OnICEConnectionStateChange(func(s webrtc.ICEConnectionState) {
		t.Logf("client PC ICE state: %s", s.String())
		if s == webrtc.ICEConnectionStateConnected || s == webrtc.ICEConnectionStateCompleted {
			select {
			case connectedCh <- struct{}{}:
			default:
			}
		}
	})

	// 读循环:处理 answer + remote candidate。
	go func() {
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			f, err := readFrame(data)
			if err != nil {
				continue
			}
			switch f.Type {
			case FrameTypeAnswer:
				var sd map[string]string
				_ = json.Unmarshal([]byte(f.SDP), &sd)
				ans := webrtc.SessionDescription{SDP: sd["sdp"]}
				switch sd["type"] {
				case "answer":
					ans.Type = webrtc.SDPTypeAnswer
				case "pranswer":
					ans.Type = webrtc.SDPTypePranswer
				}
				if err := clientPC.SetRemoteDescription(ans); err != nil {
					t.Logf("client SetRemoteDescription: %v", err)
				}
			case FrameTypeCandidate:
				var init webrtc.ICECandidateInit
				if err := json.Unmarshal([]byte(f.Candidate), &init); err == nil {
					_ = clientPC.AddICECandidate(init)
				}
			}
		}
	}()

	select {
	case <-connectedCh:
		t.Log("✅ ICE reached connected")
	case <-time.After(iceTimeout):
		t.Fatalf("ICE did not reach connected within %v", iceTimeout)
	}
}
