import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { VitePWA } from 'vite-plugin-pwa'

// 开发时:Vite dev server(默认 5173)把 /api 请求代理到后端 :8000
// 生产时:构建产物输出到 ../server/static,由 Go server 一并托管(同源)。
// (安卓端已拆分为原生工程 android-app/,不再打包网页)

const pwaPlugin = VitePWA({
  registerType: 'autoUpdate',
  injectRegister: 'inline', // 自动注入 SW 注册,无需改 main.js
  manifest: {
    name: '抖音下载器',
    short_name: '抖音下载',
    description: '粘贴抖音链接,直存无水印视频',
    display: 'standalone',
    orientation: 'portrait',
    background_color: '#ffffff',
    theme_color: '#42b983',
    lang: 'zh-CN',
    icons: [
      { src: '/icons/pwa-192.png', sizes: '192x192', type: 'image/png' },
      { src: '/icons/pwa-512.png', sizes: '512x512', type: 'image/png' },
      {
        src: '/icons/maskable-512.png',
        sizes: '512x512',
        type: 'image/png',
        purpose: 'maskable',
      },
    ],
  },
  // 应用壳可离线缓存;但 /api 永不缓存(实时数据)
  workbox: {
    navigateFallbackDenylist: [/^\/api\//],
    // 不拦截 /api/v1/stream 的下载
    maximumFileSizeToCacheInBytes: 5 * 1024 * 1024,
  },
})

export default defineConfig({
  plugins: [vue(), pwaPlugin],
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
