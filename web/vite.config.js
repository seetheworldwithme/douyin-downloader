import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// 开发时:Vite dev server(默认 5173)把 /api 请求代理到后端 :8000
// 生产时:构建产物输出到 ../server/static,由 FastAPI 一并托管(同源)
export default defineConfig({
  plugins: [vue()],
  base: '/',
  build: {
    outDir: '../server/static',
    emptyOutDir: true,
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8000',
        changeOrigin: true,
      },
    },
  },
})
