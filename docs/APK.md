# 安卓 APK 构建指南

本仓库的网页端同时也是安卓 APP 的 UI。APP 用 **Capacitor** 把 Vue 站点包进 WebView,
再用一个原生插件(`SaveToGalleryPlugin`)把视频直接写入系统相册(`Movies/抖音下载器`),
不经服务器磁盘、不进通用下载夹。

```
手机 APP(Capacitor WebView,本地 UI) ──HTTPS──> 远程部署的 Python 服务(--serve)
   └ 原生插件 OkHttp 拉 /api/v1/stream → MediaStore.Video → 相册可见
```

## 前置条件

- **JDK 17+**(本仓库已确认有 JDK 21)。
- **Android Studio**(带 Android SDK 34 + Build-Tools)+ platform-tools。
  首次打开工程时让它自动下载所需 SDK 组件。
- **Node 18+**(本仓库已确认有 Node 24)。
- 一台 **Android 10(API 29)+** 的手机(开启 USB 调试),或 Android 模拟器。

> 原生插件只实现了 API 29+ 的 `MediaStore` 分区存储路径,故 `minSdkVersion = 29`。
> 如需兼容 Android 9 及以下,需追加 `WRITE_EXTERNAL_STORAGE` 权限与公共目录直写分支
> (见 `mobile/native/SaveToGalleryPlugin.kt` 注释)。

## 一、首次搭建(只做一次)

```bash
# 1) 安装前端依赖并构建(产物输出到 ../server/static)
cd web
npm install
npm run build

# 2) 生成安卓工程(在 web/ 目录执行,产物为 web/android/)
npx cap add android
```

生成后,把原生插件与配置接进去:

```bash
# 3) 复制插件到安卓工程源码目录(包名路径需与 capacitor.config.ts 的 appId 一致)
#    appId = io.github.nick.dydl → 路径 io/github/nick/dydl/
mkdir -p android/app/src/main/java/io/github/nick/dydl
cp ../mobile/native/SaveToGalleryPlugin.kt \
   android/app/src/main/java/io/github/nick/dydl/SaveToGalleryPlugin.kt
```

- **OkHttp 依赖**:打开 `web/android/app/build.gradle`,在 `dependencies { ... }` 里加
  `implementation 'com.squareup.okhttp3:okhttp:4.12.0'`(参考 `mobile/native/app.build.gradle.snippet`)。
- **minSdk**:编辑 `web/android/variables.gradle`,把 `minSdkVersion` 改为 `29`。
- **包名**:若改了 `capacitor.config.ts` 的 `appId`,把插件文件的 `package` 行与目录路径一并对齐。
- **应用名**:在 Android Studio 里改 `android/app/src/main/res/values/strings.xml` 的 `app_name`(默认取 `capacitor.config.ts` 的 `appName`)。

> Capacitor 会自动扫描带 `@CapacitorPlugin` 注解的类完成注册,**无需改 MainActivity**。
> `AndroidManifest.xml` 也不用加权限:Capacitor 默认已含 `INTERNET`,而 API 29+ 写 `MediaStore`
> 不需要任何存储权限。

## 二、每次更新前端后

```bash
cd web
npm run build          # 重新构建 → ../server/static
npx cap sync android   # 把最新 web 资源拷进安卓工程 + 同步插件
```

## 三、构建 / 安装 APK

**用 Android Studio(推荐)**:打开 `web/android/`,连上手机 → Run ▶,或
`Build > Build APK(s)`。

**或命令行**:

```bash
cd web/android
./gradlew assembleDebug      # debug APK:android/app/build/outputs/apk/debug/app-debug.apk
adb install -r app/build/outputs/apk/debug/app-debug.apk
```

### Release(自签名,可选)

```bash
# 1) 生成一次 keystore(妥善保管,勿提交进仓库)
keytool -genkey -v -keystore dydl.keystore -alias dydl -keyalg RSA -keysize 2048 -validity 10000

# 2) 在 android/app/build.gradle 配置 signingConfigs + 把 release 指向它
# 3) 构建
cd web/android && ./gradlew assembleRelease
```

## 四、首启使用流程

1. 装好 APK 打开 → 因为没配服务器地址,首屏显示 **设置** 卡。
2. 填入远程服务地址,如 `https://your-nas.example.com:8000` → 保存并测试。
3. 登录(用户名/密码来自服务端 `config.yml` 的 `auth`)。
4. 粘贴抖音视频链接 → 下载 → 进度条走完 → 视频出现在 **相册 / 文件管理 → Movies/抖音下载器**。

## 已知限制 / 排错

- **自签 HTTPS**:WebView 默认不信任自签证书,会连不上。
  - 推荐:用反代(Caddy / Nginx)+ Let's Encrypt 合法证书。
  - 或:在内网用 HTTP(给 `capacitor.config.ts` 加 `server: { cleartext: true }` 并在
    manifest 加 `android:usesCleartextTraffic="true"`,**仅限内网**)。
- **后台下载**:v1 在 APP 在前台时下载并显示应用内进度条;切到后台长任务可能被系统暂停。
  后续可加前台 Service 提升健壮性。
- **进不了相册**:确认手机系统版本 ≥ Android 10;部分厂商 ROM 的「相册」需手动刷新或
  从「文件管理 → Movies」查看;`adb shell ls Movies/抖音下载器/` 可核对文件确实落盘。
- **插件未生效**(调用报 not implemented):确认插件包名路径与 `appId` 一致、`@CapacitorPlugin`
  注解存在、改过原生代码后执行了 `npx cap sync android` 并重新构建 APK。
