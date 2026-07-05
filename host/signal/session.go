package signal

import (
	"sync"

	"github.com/pion/webrtc/v4"
)

// session 是一次 Host↔Client 的活跃会话(CONTEXT.md Session)。
// Host 同时只维护一个活跃 Session(ADR-0003 单 Client 模型)。
//
// 只持有 PC —— 信令 WS conn 由 handler 的局部变量管理,无需 registry 关心。
type session struct {
	id int64                  // Session ID,单调递增(CONTEXT.md Session ID)
	pc *webrtc.PeerConnection // Host 侧 answerer
}

// registry 维护 Host 的活跃 Session 槽位。
//
// 单 Client 模型(ADR-0003):同一时刻只有一个 active session。新 Client
// 建连时 newSession 把 active 替换为新的、并 close 旧的 PC,为"后来者抢占
// 前者"留 hook。本 slice(#3)单 Client 即可,但抢占语义的骨架在此就位,
// #8 接入完整抢占逻辑(旧 PC teardown + stale-input-drop)。
//
// 所有方法 goroutine-safe。
type registry struct {
	mu     sync.Mutex
	next   int64 // 下一个 Session ID(从 1 开始)
	active *session
}

func newRegistry() *registry {
	return &registry{next: 1}
}

// newSession 注册一个新 Session,返回分配的 Session ID。
//
// 如果当前已有 active session(本 slice 单 Client 模型下,理论上不会发生
// 除非 Client 重连或第二个 Client 接入),close 其 PC 让位 —— 这是 ADR-0003
// "后来者抢占前者"的最小实现;#8 会在此扩展完整的 teardown 通知。
//
// 调用方负责把 sid 经 auth_ok 帧回写给 Client。
func (r *registry) newSession(pc *webrtc.PeerConnection) int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	// 让位:若已有 active,关其 PC(其 WS 由各自的 handler defer 清理)。
	if r.active != nil && r.active.pc != nil && r.active.pc != pc {
		_ = r.active.pc.Close()
	}
	sid := r.next
	r.next++
	r.active = &session{id: sid, pc: pc}
	return sid
}

// clearIfActive 当 session 仍是 active 时清空槽位(用于 handler 退出时)。
// 传 sid 而非指针,避免清错被抢占后的新 session。
func (r *registry) clearIfActive(sid int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.active != nil && r.active.id == sid {
		r.active = nil
	}
}

// activeSessionID 返回当前 active 的 sid(无则 0)。供测试与后续 slice 用。
func (r *registry) activeSessionID() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.active == nil {
		return 0
	}
	return r.active.id
}
