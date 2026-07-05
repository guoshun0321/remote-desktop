import { createApp } from 'vue'
import App from './App.vue'

// Client 入口。本 slice(#2 骨架)只渲染占位 UI;信令 / WebRTC / 输入逻辑在
// 后续 slice(#3 起)引入。CONTEXT.md 词汇:Client = 控制端浏览器 SPA。
createApp(App).mount('#app')
