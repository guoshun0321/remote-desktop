// Client 侧信令帧定义。与 Host 的 host/signal/protocol.go 严格对齐。
//
// wire format 是 JSON,每帧带 type 字段。详见 host/signal/protocol.go 的注释
// 与 CONTEXT.md 词汇:Signal / Offerer-Answerer / Trickle ICE / 单密码认证 /
// Signal Origin Check。

export type FrameType =
  | 'auth' // C→H:握手首帧,带 password
  | 'auth_ok' // H→C:认证通过,回写 sessionId
  | 'offer' // C→H:Client 主动发起的 SDP offer
  | 'answer' // H→C:Host 回的 SDP answer
  | 'candidate' // 双向:trickle ICE 候选
  | 'error' // H→C:密码错 / Origin 拒绝 / 协议错

export interface Frame {
  type: FrameType
  password?: string
  sessionId?: number
  /** SDP 帧:JSON 对象 {"type":"offer|answer","sdp":"<SDP 串>"} 的字符串化形式。 */
  sdp?: string
  /** candidate 帧:RTCICECandidateInit 的 JSON 字符串。 */
  candidate?: string
  message?: string
}

// SDPDescription 是 sdp 字段里塞的 JSON 对象。与浏览器 RTCSessionDescription
// 的 {type, sdp} 形状一致,可直接喂给 setLocalDescription / setRemoteDescription。
export interface SDPDescription {
  type: 'offer' | 'answer' | 'pranswer' | 'rollback'
  sdp: string
}

/** 序列化一个 SDPDescription 为字符串(放进 Frame.sdp)。 */
export function serializeSDP(desc: RTCSessionDescriptionInit | SDPDescription): string {
  return JSON.stringify({ type: desc.type, sdp: desc.sdp })
}

/** 反序列化 Frame.sdp 字段为 SDPDescription。 */
export function parseSDP(raw: string): SDPDescription {
  return JSON.parse(raw) as SDPDescription
}
