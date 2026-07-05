<script setup lang="ts">
// Client SPA 根组件。本 slice(#2 骨架)只显示"连接状态:就绪"占位。
//
// 后续 slice 在此挂载:
//   - #3 信令 + WebRTC 连接:connecting/connected/disconnected/failed 四态机
//   - #4 视频:<video> 元素渲染 Host 主屏画面
//   - #5 音频:<audio> 元素播放 Host 系统音频
//   - #6 键鼠输入:采集本地事件 → input datachannel
//   - #7 剪贴板:navigator.clipboard 同步 + --insecure 降级提示
//
// CONTEXT.md 词汇:Connection state 显示词为 connecting / connected /
// disconnected / failed(见 CONTEXT.md "连接状态显示")。
import { ref } from 'vue'

const connectionState = ref<'connecting' | 'connected' | 'disconnected' | 'failed'>('disconnected')
// 占位:骨架 slice 下 Client 尚未连接任何 Host,显示"就绪"等待用户操作。
const ready = ref(true)
</script>

<template>
  <main class="client">
    <h1>Remote Desktop Client</h1>
    <p class="status" :data-state="connectionState">
      连接状态:
      <strong>{{ ready ? '就绪(等待连接)' : connectionState }}</strong>
    </p>
    <p class="hint">骨架 slice #2 — 信令 / WebRTC / 媒体 / 输入将在后续 slice 接入。</p>
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
.status {
  font-size: 1.1rem;
  padding: 0.75rem 1rem;
  border-radius: 6px;
  background: #f0f4f8;
}
.status[data-state='connected'] {
  background: #d4f4dd;
}
.status[data-state='disconnected'],
.status[data-state='failed'] {
  background: #fde4e4;
}
.hint {
  color: #677076;
  font-size: 0.9rem;
  margin-top: 1.5rem;
}
</style>
