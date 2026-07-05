<script setup lang="ts">
// Client SPA 根组件。本 slice(#3)接入 useConnection,显示连接状态四态机
// + 提供极简连接表单(Host URL + 密码 + Connect/Disconnect)。
//
// 后续 slice 在 connected 后挂载:
//   - #4 视频:<video> 渲染 Host 主屏
//   - #5 音频:<audio> 播放 Host 系统音频
//   - #6 键鼠输入:采集本地事件 → input datachannel
//   - #7 剪贴板:navigator.clipboard 同步 + --insecure 降级提示
import { ref } from 'vue'
import { useConnection } from './signal/useConnection'

const { state, sessionId, error, connect, disconnect } = useConnection()

// 默认 Host URL:本地 dev 用 ws:// + Host 默认 insecure 端口。生产是 wss://。
const hostUrl = ref('ws://localhost:18443/ws')
const password = ref('')

function onConnect() {
  if (!hostUrl.value || !password.value) return
  void connect(hostUrl.value, password.value)
}

const stateLabel: Record<string, string> = {
  disconnected: '已断开',
  connecting: '连接中…',
  connected: '已连接',
  failed: '连接失败',
}
</script>

<template>
  <main class="client">
    <h1>Remote Desktop Client</h1>

    <section class="status" :data-state="state">
      <span class="dot" />
      <strong>{{ stateLabel[state] }}</strong>
      <span v-if="sessionId !== null" class="sid">Session #{{ sessionId }}</span>
    </section>

    <form class="connect-form" @submit.prevent="onConnect">
      <label>
        Host
        <input v-model="hostUrl" type="text" placeholder="ws://host:port/ws" :disabled="state === 'connecting'" />
      </label>
      <label>
        Password
        <input v-model="password" type="password" autocomplete="current-password" :disabled="state === 'connecting'" />
      </label>
      <div class="actions">
        <button type="submit" :disabled="state === 'connecting' || !hostUrl || !password">
          {{ state === 'connecting' ? '连接中…' : '连接' }}
        </button>
        <button
          v-if="state === 'connected' || state === 'failed'"
          type="button"
          class="secondary"
          @click="disconnect"
        >
          断开
        </button>
      </div>
    </form>

    <p v-if="error" class="error">⚠ {{ error }}</p>

    <p class="hint">
      骨架 + 信令 slice(#2/#3) — 媒体 / 输入 / 剪贴板将在 #4–#7 接入。
    </p>
  </main>
</template>

<style scoped>
.client {
  font-family: system-ui, -apple-system, 'Segoe UI', Roboto, sans-serif;
  max-width: 640px;
  margin: 4rem auto;
  padding: 0 1.5rem;
  color: #1f2933;
}
h1 {
  font-size: 1.5rem;
  margin: 0 0 1.5rem;
}
.status {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  padding: 0.75rem 1rem;
  border-radius: 6px;
  background: #f0f4f8;
  font-size: 1.05rem;
}
.status[data-state='connected'] {
  background: #d4f4dd;
}
.status[data-state='disconnected'],
.status[data-state='failed'] {
  background: #fde4e4;
}
.dot {
  width: 0.7rem;
  height: 0.7rem;
  border-radius: 50%;
  background: #94a3b8;
}
.status[data-state='connected'] .dot {
  background: #16a34a;
}
.status[data-state='failed'] .dot {
  background: #dc2626;
}
.sid {
  margin-left: auto;
  font-size: 0.85rem;
  color: #677076;
}
.connect-form {
  margin-top: 1.5rem;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}
label {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  font-size: 0.9rem;
  color: #4b5563;
}
input {
  padding: 0.5rem 0.6rem;
  border: 1px solid #cbd5e1;
  border-radius: 4px;
  font-size: 1rem;
}
input:disabled {
  background: #f1f5f9;
}
.actions {
  display: flex;
  gap: 0.5rem;
}
button {
  padding: 0.5rem 1rem;
  border: 0;
  border-radius: 4px;
  background: #1d76db;
  color: white;
  font-size: 0.95rem;
  cursor: pointer;
}
button:disabled {
  background: #94a3b8;
  cursor: not-allowed;
}
button.secondary {
  background: #64748b;
}
.error {
  margin-top: 1rem;
  color: #dc2626;
  font-size: 0.9rem;
}
.hint {
  color: #677076;
  font-size: 0.85rem;
  margin-top: 2rem;
}
</style>
