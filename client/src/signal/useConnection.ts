// Client 连接状态机 composable。封装 WebSocket 信令 + WebRTC PeerConnection
// 的完整建连流程(Client 永远是 SDP offerer,Host 永远是 answerer —— CONTEXT.md
// Offerer/Answerer Roles)。
//
// 状态机(CONTEXT.md "连接状态显示" 四态):
//   disconnected → connecting → connected → disconnected/failed
//
// 本 slice(#3)只跑通建连,不挂 media/datachannel。reconnect 退避留 #9。
import { ref, type Ref } from 'vue'
import { type Frame, serializeSDP, parseSDP } from './protocol'

export type ConnectionState = 'connecting' | 'connected' | 'disconnected' | 'failed'

export interface UseConnection {
  state: Ref<ConnectionState>
  sessionId: Ref<number | null>
  error: Ref<string | null>
  connect: (hostUrl: string, password: string) => Promise<void>
  disconnect: () => void
}

/**
 * 建立连接。hostUrl 是 Host 的信令 WS 地址(如 ws://localhost:18443/ws 或
 * wss://host.example.com/ws)。password 是单密码(WebSocket 握手首帧认证)。
 *
 * 流程:
 *  1. 开 WebSocket
 *  2. 发 auth 帧 → 等 auth_ok(回写 sessionId)
 *  3. 建 RTCPeerConnection(iceServers: [] —— host candidate only,ADR-0002)
 *  4. createOffer → setLocalDescription → 发 offer 帧
 *  5. OnICECandidate → 发 candidate 帧(trickle out)
 *  6. 收 answer → setRemoteDescription;收 candidate → addIceCandidate
 *  7. iceconnectionstate connected → 状态机进 connected
 */
export function useConnection(): UseConnection {
  const state = ref<ConnectionState>('disconnected')
  const sessionId = ref<number | null>(null)
  const error = ref<string | null>(null)

  let ws: WebSocket | null = null
  let pc: RTCPeerConnection | null = null

  function setState(s: ConnectionState) {
    state.value = s
  }
  function fail(msg: string) {
    error.value = msg
    setState('failed')
  }

  function send(frame: Frame) {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify(frame))
    }
  }

  async function connect(hostUrl: string, password: string) {
    error.value = null
    setState('connecting')
    try {
      await openAndAuth(hostUrl, password)
      await negotiate()
    } catch (e) {
      fail(e instanceof Error ? e.message : String(e))
      teardown()
    }
  }

  // 1-2:开 WS + 发 auth + 等 auth_ok。
  function openAndAuth(hostUrl: string, password: string): Promise<void> {
    return new Promise((resolve, reject) => {
      ws = new WebSocket(hostUrl)
      ws.onopen = () => {
        send({ type: 'auth', password })
      }
      ws.onmessage = (ev) => {
        let frame: Frame
        try {
          frame = JSON.parse(ev.data) as Frame
        } catch {
          reject(new Error('invalid frame from Host'))
          return
        }
        if (frame.type === 'auth_ok') {
          sessionId.value = frame.sessionId ?? null
          // 切到通用 onmessage 处理后续帧(answer/candidate)。
          ws!.onmessage = handleSignalFrame
          resolve()
        } else if (frame.type === 'error') {
          reject(new Error(frame.message ?? 'auth failed'))
        } else {
          reject(new Error(`expected auth_ok, got ${frame.type}`))
        }
      }
      ws.onerror = () => reject(new Error('WebSocket error'))
      ws.onclose = () => {
        // 如果还没 connected,任何 close 都是失败信号(重连在 #9)。
        if (state.value === 'connecting') {
          reject(new Error('WebSocket closed during handshake'))
        }
      }
    })
  }

  // 3-4:建 PC + createOffer + 发 offer。
  async function negotiate() {
    // iceServers: [] —— 浏览器只用 host candidate(ADR-0002 首期 host-only)。
    pc = new RTCPeerConnection({ iceServers: [] })

    // trickle out:本地候选一产生就推给 Host。
    pc.onicecandidate = (ev) => {
      if (ev.candidate) {
        send({ type: 'candidate', candidate: JSON.stringify(ev.candidate.toJSON()) })
      }
    }

    // ICE 状态推进状态机。
    pc.oniceconnectionstatechange = () => {
      if (!pc) return
      const s = pc.iceConnectionState
      if (s === 'connected' || s === 'completed') {
        setState('connected')
      } else if (s === 'failed') {
        fail('ICE failed')
      } else if (s === 'disconnected') {
        // 短暂 disconnected 不立刻降级(PC 可能自愈);#9 接入超时降级。
        // 本 slice 仅观察。
      }
    }

    // #3 不挂 media/datachannel。但需要一个 m-line 让 ICE 真正协商 —— 否则
    // 浏览器可能不收集候选。后续 #4–#7 会用真实 transceiver 替换。
    pc.addTransceiver('video', { direction: 'recvonly' })

    const offer = await pc.createOffer()
    await pc.setLocalDescription(offer)
    send({ type: 'offer', sdp: serializeSDP(offer) })
  }

  // 6:处理 Host 来帧(answer / candidate)。
  function handleSignalFrame(ev: MessageEvent) {
    let frame: Frame
    try {
      frame = JSON.parse(ev.data) as Frame
    } catch {
      return
    }
    if (!pc) return
    switch (frame.type) {
      case 'answer': {
        const desc = parseSDP(frame.sdp ?? '')
        pc.setRemoteDescription(new RTCSessionDescription(desc)).catch((e) =>
          fail(`setRemoteDescription: ${e}`)
        )
        break
      }
      case 'candidate': {
        const init = JSON.parse(frame.candidate ?? '{}') as RTCIceCandidateInit
        pc.addIceCandidate(init).catch(() => {
          /* 单个 candidate 失败不致命 */
        })
        break
      }
      case 'error':
        fail(frame.message ?? 'Host error')
        break
    }
  }

  function teardown() {
    if (pc) {
      try {
        pc.close()
      } catch {
        /* ignore */
      }
      pc = null
    }
    if (ws) {
      try {
        ws.close()
      } catch {
        /* ignore */
      }
      ws = null
    }
  }

  function disconnect() {
    teardown()
    sessionId.value = null
    setState('disconnected')
  }

  return { state, sessionId, error, connect, disconnect }
}
