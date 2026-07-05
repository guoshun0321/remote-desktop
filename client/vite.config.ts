import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// Vite 配置。本 slice 用默认设置;后续 slice(#3 信令)会加 dev server
// proxy 把 /ws 代理到本地 Host,便于本地联调。
export default defineConfig({
  plugins: [vue()],
  server: {
    // dev server 端口;Vite 默认 5173,显式写出便于 Host 端 Origin 校验白名单。
    port: 5173,
  },
})
