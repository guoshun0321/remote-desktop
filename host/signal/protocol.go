// Package signal 实现 Host 侧的 WebSocket 信令端点与 WebRTC PeerConnection
// 编排(answerer 角色)。
//
// CONTEXT.md 词汇:
//   - Signal:Host 与 Client 之间经 WebSocket 交换 SDP offer/answer 与 ICE
//     candidate 的过程,只用于建连协商,不承载媒体或输入。
//   - Offerer/Answerer Roles:Client 永远是 SDP offerer,Host 永远是 answerer。
//   - Trickle ICE:候选边收集边经 signal 推送。
//   - Signal Origin Check:Host 在 WS 握手阶段校验 Origin header。
//
// 本 slice(#3)只建 PeerConnection,不挂任何 media track / datachannel
// (那些是 #4–#7)。Session ID 在此 slice mint 并回写给 Client;抢占 teardown
// 与 stale-input-drop 留给 #6/#8。
package signal

import (
	"encoding/json"
	"fmt"
)

// FrameType 是信令帧类型。wire format 是 JSON,每帧带 type 字段。
type FrameType string

const (
	// Client → Host
	FrameTypeAuth      FrameType = "auth"      // 握手后第一帧:{"type":"auth","password":"..."}
	FrameTypeOffer     FrameType = "offer"     // Client 主动发起的 SDP offer
	FrameTypeCandidate FrameType = "candidate" // trickle ICE 候选

	// Host → Client
	FrameTypeAuthOK FrameType = "auth_ok" // 认证通过 + 回写 Session ID
	FrameTypeAnswer FrameType = "answer"  // Host 回的 SDP answer
	// FrameTypeCandidate 复用(双向)
	FrameTypeError FrameType = "error" // 密码错 / Origin 拒绝 / 协议错
)

// Frame 是 WebSocket 上传输的信令帧统一信封。
//
// 不同 type 用不同字段:
//
//	auth      → Password
//	auth_ok   → SessionID
//	offer     → SDP(webrtc.SessionDescription.Serialize 的字符串形式)
//	answer    → SDP
//	candidate → Candidate(webrtc.ICECandidateInit 的 JSON 字符串)
//	error     → Message
type Frame struct {
	Type      FrameType `json:"type"`
	Password  string    `json:"password,omitempty"`
	SessionID int64     `json:"sessionId,omitempty"`
	SDP       string    `json:"sdp,omitempty"`
	Candidate string    `json:"candidate,omitempty"`
	Message   string    `json:"message,omitempty"`
}

// readFrame 从 JSON 字节解析一个 Frame。
func readFrame(data []byte) (Frame, error) {
	var f Frame
	if err := json.Unmarshal(data, &f); err != nil {
		return Frame{}, fmt.Errorf("signal: invalid frame json: %w", err)
	}
	return f, nil
}

// writeFrame 序列化 Frame 为 JSON 字节。
func writeFrame(f Frame) ([]byte, error) {
	return json.Marshal(f)
}
