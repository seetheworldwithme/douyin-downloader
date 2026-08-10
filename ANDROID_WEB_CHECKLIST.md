# 安卓端打包 — Web 侧需修改清单

> 目标:把现有 Vue + Vite 前端(`web/`)用 Capacitor 打成 Android APK。
> 结论:**架构已就绪(90%)**,核心通路(WebView `https://localhost` 跨域请求、原生 OkHttp 落盘相册、token 走 query)都已正确处理。
> 真正会卡住或掉体验的是下面这几项,按优先级排列。

---

## ✅ 本次已实现(代码改动)

| 清单项 | 文件 | 改动 |
|---|---|---|
| **P0-1 安全区 / edge-to-edge** | `web/src/App.vue` | header/main 加 `env(safe-area-inset-*)`,`100vh`→`100dvh`,按钮触摸目标 ≥40px |
| **P0-3 CORS 防覆盖** | `server-go/internal/server/server.go` | `defaultCORS` 改为**合并**(始终保留 `https://localhost` / `capacitor://localhost`),不再被 config 覆盖 |
| **P2-7 PWA 条件化** | `web/vite.config.js` + `package.json` | 新增 `build:native` 脚本(`BUILD_TARGET=capacitor`),APK 构建不注入 Service Worker |
| **P2-9 过时文案** | `web/src/components/SettingsCard.vue` | "Python --serve" → "Go server",并提示 APP 必须 HTTPS |
| **默认服务器地址** | `web/src/api.js` | 原生 APK 首启默认 `https://douyin.xuziyue.work`,**无需配置**(已 curl 验证线上 HTTPS 可达);网页同源仍留空 |
| **P1-4 安卓图标** | `web/public/icon*.svg` + `web/assets/*.png` | 重绘下载图标(渐变+白箭头),生成 1024 源图 + 自适应前景/背景层;`npm run assets:android` 一键生成 mipmap |
| **一键打包脚本** | `build-android.sh` + `mobile/native/SaveToGalleryPlugin.java` + `web/capacitor.config.json` | `bash build-android.sh` 全自动:cap add、minSdk29、OkHttp、注册插件(改 Java——模板无 Kotlin)、装 android-35、阿里云镜像、图标、assembleDebug → `.bin/douyin-downloader.apk`。已本机验证产出 11M debug APK |

验证:`go vet`/`go build`/`TestCORS` 通过;`npm run build:native`(无 SW)与 `npm run build`(含 PWA)均构建成功;线上 `https://douyin.xuziyue.work/api/v1/health` 返回 `{"status":"ok"}`;`rm -rf web/android && bash build-android.sh` 从零产出 APK。

> 仍未做(需手动/环境相关):P0-3 部署侧(线上 `config.yml` 的 `cors_origins` 留空即可——服务端已自动兜底 localhost)、P1-5 gradle(**已由 `build-android.sh` 自动处理**)、P1-6 证书(线上已 HTTPS ✓)、P3 打磨项(状态栏/闪屏/返回键)。
> 打包:直接 `bash build-android.sh`,产物 `.bin/douyin-downloader.apk`。

---

## 🔴 P0 · 必须处理(不处理会直接坏 / 不能发布)

### 1. 边到边(Edge-to-Edge)下的安全区 — 头部会被状态栏盖住
- **现象**:Capacitor 7 的 `compileSdk/targetSdk = 35`,Android 15 起强制 edge-to-edge,WebView 会顶到状态栏底下。当前 CSS 没有任何 `env(safe-area-inset-*)` 处理,`web/src/App.vue` 的 `.app-header`(固定 `height:60px`)会被状态栏压住,底部按钮也可能被导航条遮挡。
- **改 `web/src/App.vue` `<style>`**:
  ```css
  /* 100vh 在移动端 WebView 里不可靠,换 dvh */
  .app { min-height: 100dvh; }

  .app-header {
    /* 左右留出刘海/圆角安全区,顶部留出状态栏 */
    padding-top: env(safe-area-inset-top);
    padding-left: max(24px, env(safe-area-inset-left));
    padding-right: max(24px, env(safe-area-inset-right));
    height: calc(60px + env(safe-area-inset-top));
    box-sizing: border-box;
  }

  /* 底部内容不被手势导航条遮住 */
  .app-main {
    padding-bottom: max(24px, env(safe-area-inset-bottom));
  }
  ```
- `web/index.html` 的 `viewport-fit=cover` ✅ 已具备,无需改。
- 可选(更稳):装 `@capacitor/status-bar`,`MainActivity` 里设 `overlaysWebView: false`,让状态栏不压在 WebView 上,二者搭配最保险。

### 2. 服务器必须 HTTPS(混合内容 + 明文流量双重封锁)
- WebView origin 是 `https://localhost`(`capacitor.config.ts` 里 `androidScheme:'https'`),且 `allowMixedContent:false`。
  - JS 的 `fetch()`(登录/解析/健康检查)走 WebView → 对 `http://` 服务器会被**混合内容策略拦截**。
  - 原生 OkHttp 下载(`SaveToGalleryPlugin.kt`)对 `http://` 会被 **API 29+ 明文流量策略拦截**(`CLEARTEXT communication not permitted`)。
- **结论**:`SettingsCard` 里填的服务器地址必须是 `https://`(如 `https://douyin.xuziyue.work`,走 edge-nginx TLS)。生产环境已满足 ✅。
- 若必须支持自建 http 内网服务器,需另行:① `capacitor.config.ts` 改 `allowMixedContent:true`;② Android `network_security_config.xml` 放行明文;代价是安全性下降,**不推荐**。

### 3. 生产服务器的 CORS 必须放行 `https://localhost`
- WebView 跨域请求完全依赖服务端 CORS。`server-go/internal/server/server.go` 的 `defaultCORS` 已包含 `https://localhost` ✅。
- **但**:`NewServerDeps` 里,**只要 `config.yml` 的 `server.cors_origins` 非空,就会整体覆盖 defaultCORS**(`corsOrigins = cfg.Config.Server.CorsOrigins`)。
- **待办**:确认生产服务器那份 `config.yml` 的 `cors_origins` 要么**留空**(走默认,含 `https://localhost`),要么**显式包含** `https://localhost`。否则 APK 里所有 API 请求被 CORS 拦截、表现就是"一直离线/登录失败"。
- 附带验证:edge-nginx 反向代理要把 `OPTIONS` 预检和 `Origin` 头透传给 Go server,不要在 nginx 层 `add_header` 覆盖 CORS。

---

## 🟠 P1 · 发布前应该处理

### 4. 缺 Android 启动图标(Launcher Icon)
- `web/public/icons/pwa-*.png` 是 **PWA 图标,Capacitor 不会自动用于安卓桌面图标**。`cap sync` 后默认是 Capacitor 自带图标。
- **做法**:准备一张 ≥1024×1024 的源图,用官方工具生成全部 mipmap:
  ```bash
  cd web
  npm i -D @capacitor/assets
  # 把源图放到 web/assets/icon.png(1024x1024)、splash 之类
  npx capacitor-assets generate --android
  ```
- `appId: io.github.nick.dydl` 发布前改成你自己的包名(`capacitor.config.ts` 顶部已注明)。

### 5. minSdkVersion 必须设为 29
- `SaveToGalleryPlugin.kt` 用了 `MediaStore RELATIVE_PATH`(API 29+ 才生效)和 `IS_PENDING`,注释也写明"仅支持 API 29+"。
- **待办**:`cap add android` 后,编辑 `web/android/variables.gradle`,把 `minSdkVersion` 改成 `29`(参见 `mobile/native/app.build.gradle.snippet` 的说明)。
- `app.build.gradle.snippet` 里的 OkHttp 依赖也要加进 `web/android/app/build.gradle` 的 `dependencies {}`。

### 6. 自签名证书不支持
- WebView / OkHttp 都不会接受自签名证书,且 WebView 里**无法弹窗让用户手动信任**。生产域名用合法证书(Let's Encrypt 等)即可 ✅;自托管用自签证书的会直接连不上,需在文案里提示。

---

## 🟡 P2 · 建议优化(体验问题,不卡发布)

### 7. PWA Service Worker 在 WebView 里是多余且可能干扰
- `vite.config.js` 的 `VitePWA` 用 `injectRegister:'inline'` 会在 WebView 的 `https://localhost` origin 上注册 Workbox SW。APK 里资源本来就是本地文件,预缓存毫无意义,且可能拦截请求 / 控制台刷错。
- **建议**:给原生构建关掉 PWA。最简单是用环境变量分支:
  ```js
  // vite.config.js
  const isNative = process.env.BUILD_TARGET === 'capacitor'
  plugins: [
    vue(),
    ...(isNative ? [] : [VitePWA({...})]),
  ],
  ```
  打包 APK 前 `BUILD_TARGET=capacitor npm run build`,普通网页构建不带这个变量。若嫌麻烦,保持现状也能跑,但要实测 SW 不影响首次加载。

### 8. 触摸目标偏小
- Element Plus 是桌面向组件库,默认按钮 ~32px,低于安卓 Material 推荐的 48dp,手指点容易误触。
- 建议:`web/src/App.vue` 全局补一条 `.el-button { min-height: 40px; }`(或针对性放大主操作按钮),`el-input` 行高已够。

### 9. 过时文案
- `web/src/components/SettingsCard.vue` 描述里写的是"部署了下载服务(Python `--serve`)的地址",后端早已换成 Go。改成"部署了下载服务的地址(由 Go server 托管)"之类,避免误导自托管用户。

---

## 🟢 P3 · 锦上添花(打磨,可不急)

| 项 | 说明 |
|---|---|
| **状态栏样式** | 装 `@capacitor/status-bar`,设背景色为 `#ffffff`、深色图标,和白色 header 协调。 |
| **启动闪屏** | 装 `@capacitor/splash-screen`,配一张闪屏图,首启不再是裸白屏。 |
| **Android 返回键** | 默认点返回直接退 App。可在确认框(`ElMessageBox`)打开时拦截返回键先关弹窗,体验更自然(Capacitor `app.addListener('backButton')`)。 |
| **下载通知** | 当前下载只有页面内进度条,切后台看不到。如需后台下载+通知,需加 Foreground Service + `POST_NOTIFICATIONS` 权限(API 33+),工作量较大,看需求。 |
| **`100vh` 全局复查** | 第 1 条已处理 `.app`;若后续加页面,统一用 `100dvh`。 |

---

## ✅ 已经做对的(确认无需动)

- `Capacitor.isNativePlatform()` 分流:Web 走 `<a download>` 另存为,APK 走原生 OkHttp 落 `Movies/抖音下载器` → ✅
- token 走 query(`streamUrl` 拼了 `?token=`):原生 GET 无法带 `Authorization` 头,服务端 `requireUser` 同时支持 header 和 query ✅
- `defaultCORS` 含 `https://localhost` / `capacitor://localhost` ✅
- `localStorage` 持久化 server 地址与 token,Capacitor WebView 会持久化 ✅
- `appId` / `appName` / `webDir` 配置正确(`webDir:'../server/static'` 与 Vite 产物一致)✅
- 文件名中文、相册中文目录在 Android 上正常 ✅
- `viewport-fit=cover`、`user-scalable=no` 已设 ✅

---

## 🛠️ 打包步骤

**推荐:一键脚本**(已把下面的手动步骤 + 国内镜像/JDK/平台全部自动化)

```bash
bash build-android.sh
#   产出:.bin/douyin-downloader.apk(debug,可直接安装)
#   安装到手机(开 USB 调试,连电脑):
~/Library/Android/sdk/platform-tools/adb install -r .bin/douyin-downloader.apk
#   release 包:BUILD_VARIANT=release bash build-android.sh(需自配签名)
```

脚本内部:构建前端 → `cap add android`(仅首次)→ 补丁(minSdk29 / OkHttp / 注册 Java 插件)→ 缺则装 android-35 平台(腾讯云镜像)→ 写阿里云 Maven 镜像 → 生成图标 → `cap sync` → `gradlew assembleDebug`。**幂等,可反复跑**;本机已验证 `rm -rf web/android && bash build-android.sh` 从零产出 11M APK。

<details><summary>手动等价步骤(仅供排查脚本时参考)</summary>

```bash
cd web && npm install && npm run build:native
npx cap add android
# android/variables.gradle:        minSdkVersion = 29
# android/app/build.gradle deps:   implementation 'com.squareup.okhttp3:okhttp:4.12.0'
# 复制 mobile/native/SaveToGalleryPlugin.java → android/app/src/main/java/io/github/nick/dydl/
# MainActivity.onCreate 里:        registerPlugin(SaveToGalleryPlugin.class);  // 在 super.onCreate 前
npx cap sync android
cd android && ./gradlew assembleDebug
```
</details>

---

## 📋 上架前自检

- [ ] APK 首启:默认即用线上服务器,直接进登录页(无需配地址;`api.js` 默认 `https://douyin.xuziyue.work`)
- [ ] 健康检查亮"在线"
- [ ] 登录成功 → 解析链接 → 确认下载 → 相册 `Movies/抖音下载器` 出现 mp4
- [ ] 状态栏不遮挡头部、底部按钮不被手势条遮挡(验证 P0-1)
- [ ] 桌面图标 / 名称正确(验证 P1-4)
- [ ] 切到后台再回前台,状态正常
```
