import type { CapacitorConfig } from '@capacitor/cli'

// Capacitor 配置。`cap add android` / `cap sync` 在本目录(web/)执行。
//
// appId:安卓包名,发布前改成你自己的(反转域名,全小写)。
// webDir:Vue 构建产物(npm run build → ../server/static),APK 与网页共用同一份。
// server.androidScheme:'https' → WebView origin = https://localhost(已加进后端 CORS 默认列表)。
const config: CapacitorConfig = {
  appId: 'io.github.nick.dydl',
  appName: '抖音下载器',
  webDir: '../server/static',
  server: {
    androidScheme: 'https',
  },
  android: {
    allowMixedContent: false,
  },
}

export default config
