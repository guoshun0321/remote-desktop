package signal

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
)

// Handler 是 /ws 信令端点的 http.Handler。持有 Config 关键字段(密码、origin
// 白名单、ICE 模式)与 session registry。
//
// 一个 Handler 实例可并发服务多个 Client(每条 WS 一个 goroutine);registry
// 保证单 active session 语义。
type Handler struct {
	password  string
	origins   []string
	ice      ICEConfig
	registry *registry
	logger   *log.Logger
	upgrader websocket.Upgrader
}

// ICEConfig 描述 Host 侧 PeerConnection 的 ICE 配置(ADR-0002)。
type ICEConfig struct {
	Mode string // "host" | "stun" | "turn"(本 slice 仅实现 host)
}

// NewHandler 构造信令 Handler。
func NewHandler(password string, origins []string, ice ICEConfig, logger *log.Logger) *Handler {
	if logger == nil {
		logger = log.Default()
	}
	return &Handler{
		password: password,
		origins:  origins,
		ice:      ice,
		registry: newRegistry(),
		logger:   logger,
		upgrader: websocket.Upgrader{
			// Origin 校验由 handler 自身在升级前完成(upgrader.CheckOrigin 不
			// 足以表达 ":*" 通配,故这里放行 true,真正的判断在 ServeHTTP)。
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

// ActiveSessionID 返回当前活跃 Session 的 ID(无则 0)。
// 供 host.Server 观察与 #8 抢占逻辑用(比对 sid 决定 teardown 归属)。
func (h *Handler) ActiveSessionID() int64 { return h.registry.activeSessionID() }

// ServeHTTP 处理 /ws 升级 + 信令编排。
//
// 流程(CONTEXT.md Signal / Offerer-Answerer / Trickle ICE / Signal Origin Check):
//  1. Origin check(不符 → 403,不升级)
//  2. WS 升级
//  3. 读首帧 → 必须 auth + 密码比对(错 → error 帧 + close)
//  4. 建 pion PC(answerer)
//  5. newSession 拿 sid → 回 auth_ok
//  6. 循环读帧:offer → SetRemoteDescription + CreateAnswer + SetLocalDescription
//     + 回 answer;candidate → AddICECandidate(trickle in)
//  7. OnICECandidate → 回 candidate 帧(trickle out)
//  8. ICE state 变化 → log
//  9. defer:close conn + pc.Close + clearIfActive
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 1. Signal Origin Check(CONTEXT.md)。
	if !originAllowed(h.origins, r.Header.Get("Origin")) {
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return
	}

	// 2. WS 升级。
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		// Upgrade 已写过 HTTP 响应;只 log。
		h.logger.Printf("signal: ws upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	// 3. 读首帧,必须 auth。
	if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		h.sendError(conn, "internal error")
		return
	}
	_, data, err := conn.ReadMessage()
	if err != nil {
		h.logger.Printf("signal: read auth frame failed: %v", err)
		return
	}
	frame, err := readFrame(data)
	if err != nil {
		h.sendError(conn, "invalid frame")
		return
	}
	if frame.Type != FrameTypeAuth || frame.Password != h.password {
		h.sendError(conn, "auth failed")
		return
	}
	// 后续无 deadline(信令交互可能等 ICE 收集)。
	_ = conn.SetReadDeadline(time.Time{})

	// 4. 建 PC(answerer)。
	pc, err := h.defaultPC()
	if err != nil {
		h.sendError(conn, "internal error")
		h.logger.Printf("signal: create peer connection failed: %v", err)
		return
	}
	defer pc.Close()

	// 5. 注册 session(分配 sid),回 auth_ok。
	sid := h.registry.newSession(pc)
	defer h.registry.clearIfActive(sid)
	if err := h.writeFrame(conn, Frame{Type: FrameTypeAuthOK, SessionID: sid}); err != nil {
		h.logger.Printf("signal: write auth_ok failed: %v", err)
		return
	}

	// 7. trickle out:把 Host 收集到的 ICE 候选推给 Client。
	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			// gathering complete
			return
		}
		init := c.ToJSON()
		raw, _ := json.Marshal(init)
		if err := h.writeFrame(conn, Frame{Type: FrameTypeCandidate, Candidate: string(raw)}); err != nil {
			h.logger.Printf("signal: write candidate failed: %v", err)
		}
	})

	// 8. ICE 状态变化日志(本 slice 的"成功"可见点是 connected 日志)。
	// 后续 #8 抢占 / #9 重连会在此 hook PC failed → teardown / 降级状态。
	pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		h.logger.Printf("signal: sid=%d ICE state=%s", sid, state.String())
	})

	// 6. 信令主循环:读 Client 帧。
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				h.logger.Printf("signal: sid=%d read loop ended: %v", sid, err)
			}
			return
		}
		frame, err := readFrame(data)
		if err != nil {
			h.sendError(conn, "invalid frame")
			return
		}
		switch frame.Type {
		case FrameTypeOffer:
			if err := h.handleOffer(conn, pc, frame); err != nil {
				h.logger.Printf("signal: sid=%d handle offer: %v", sid, err)
				h.sendError(conn, "offer handling failed")
				return
			}
		case FrameTypeCandidate:
			if err := h.handleCandidate(pc, frame); err != nil {
				h.logger.Printf("signal: sid=%d handle candidate: %v", sid, err)
				// 单个 candidate 失败不致命,继续。
			}
		default:
			h.sendError(conn, "unexpected frame type after auth")
			return
		}
	}
}

// handleOffer 处理 Client 的 SDP offer(Client offerer / Host answerer)。
//	SetRemoteDescription(offer) → CreateAnswer → SetLocalDescription(answer)
//	→ 回 answer 帧。answer 的 ICE 候选由 OnICECandidate trickle 推送。
//
// wire 上的 SDP 是 JSON 对象 {"type":"offer|answer","sdp":"<SDP 串>"} —— 与
// 浏览器 RTCPeerConnection 的 setLocalDescription/setRemoteDescription 直接
// 兼容,也便于 pion 双向用。
func (h *Handler) handleOffer(conn *websocket.Conn, pc *webrtc.PeerConnection, f Frame) error {
	var sd struct {
		Type string `json:"type"`
		SDP  string `json:"sdp"`
	}
	if err := json.Unmarshal([]byte(f.SDP), &sd); err != nil {
		return err
	}
	offer := webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: sd.SDP}
	if err := pc.SetRemoteDescription(offer); err != nil {
		return err
	}
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		return err
	}
	if err := pc.SetLocalDescription(answer); err != nil {
		return err
	}
	// 回 answer:同形 JSON 对象。
	raw, err := json.Marshal(map[string]string{
		"type": answer.Type.String(),
		"sdp":  answer.SDP,
	})
	if err != nil {
		return err
	}
	return h.writeFrame(conn, Frame{Type: FrameTypeAnswer, SDP: string(raw)})
}

// handleCandidate 处理 Client trickle 推来的 ICE 候选。
func (h *Handler) handleCandidate(pc *webrtc.PeerConnection, f Frame) error {
	var init webrtc.ICECandidateInit
	if err := json.Unmarshal([]byte(f.Candidate), &init); err != nil {
		return err
	}
	return pc.AddICECandidate(init)
}

// defaultPC 是生产用的 pion PeerConnection 工厂(answerer)。
//
// ICE 配置(ADR-0002 host-candidate-only):空 ICEServers + 默认
// ICETransportPolicyAll —— pion 无 STUN/TURN 时只收集 host candidate。
//
// h.ice.Mode 已在 Config.Validate 阶段保证 == "host"(stun/turn 在后续 slice
// 落地前被显式拒绝,见 host/config.go)。本方法在 host 模式下配置 pion;
// stun/turn 落地时在此 switch 注入 ICEServers。
func (h *Handler) defaultPC() (*webrtc.PeerConnection, error) {
	cfg := webrtc.Configuration{
		ICEServers:         nil, // host-only:无 STUN/TURN
		ICETransportPolicy: webrtc.ICETransportPolicyAll,
	}
	switch h.ice.Mode {
	case "host", "":
		// 首期唯一支持:空 ICEServers → 只收集 host candidate。
	case "stun", "turn":
		// 后续 slice 注入 ICEServers(ADR-0002 "放开只改配置")。今天由
		// Config.Validate 拒绝,不会走到这里。
	}
	return webrtc.NewPeerConnection(cfg)
}

// writeFrame 序列化并发送一帧。
func (h *Handler) writeFrame(conn *websocket.Conn, f Frame) error {
	data, err := writeFrame(f)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, data)
}

// sendError 发 error 帧并关闭连接。
func (h *Handler) sendError(conn *websocket.Conn, msg string) {
	_ = h.writeFrame(conn, Frame{Type: FrameTypeError, Message: msg})
	_ = conn.WriteMessage(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.ClosePolicyViolation, msg))
}
