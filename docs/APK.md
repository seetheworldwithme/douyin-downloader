# 安卓 APK 构建指南

安卓端是**原生应用**(Kotlin + Jetpack Compose),工程在仓库根目录的 `android-app/`,
与 `web/` 前端完全独立,共用同一套 REST API(`/api/v1`)。下载由 OkHttp 拉取
`/api/v1/stream` 后直接写入系统媒体库(`MediaStore`),不经服务器磁盘、不进通用下载夹。

```
手机 APP(Kotlin + Compose 原生) ──HTTPS──> 远程部署的 Go 服务(server-go)
   └ MediaStoreDownloader OkHttp 拉 /api/v1/stream → MediaStore → 相册可见
```

服务器地址内置在 `android-app/app/build.gradle.kts` 的 `SERVER_BASE`
(默认 `https://douyin.xuziyue.work`,改后重新构建即可)。

## 前置条件

- **JDK 17~23**(本仓库已确认有 JDK 21;JDK 24/25 与当前 Gradle/AGP 不兼容)。
- **Android SDK**(compileSdk 35 + Build-Tools)+ platform-tools。
  装 Android Studio 即可,或设置 `ANDROID_HOME` 指向已有 SDK。
- 一台 **Android 10(API 29)+** 的手机(开启 USB 调试),或 Android 模拟器。

> 下载器只实现 API 29+ 的 `MediaStore` 分区存储路径,故 `minSdk = 29`。
> 如需兼容 Android 9 及以下,需追加 `WRITE_EXTERNAL_STORAGE` 权限与公共目录直写分支。

## 一、一键打包(推荐,Git Bash)

```bash
bash build-android-app.sh                          # debug 包(可直接安装)
BUILD_VARIANT=release bash build-android-app.sh    # release(需自配签名)
```

脚本自动完成:JDK/SDK 探测 → 阿里云 Maven 镜像(国内加速)→ `gradlew test`(单测)
→ `assembleDebug` → 拷贝产物到 `release/apk/douyin-downloader.apk` 并打印 adb 安装命令。

## 二、用 Android Studio

直接打开 `android-app/` 工程:连上手机 → Run ▶,或 `Build > Build APK(s)`。

## 三、命令行(手动)

```bash
cd android-app
./gradlew test assembleDebug    # 单测 + debug APK
adb install -r app/build/outputs/apk/debug/app-debug.apk
```

### Release(自签名,可选)

```bash
# 1) 生成一次 keystore(妥善保管,勿提交进仓库)
keytool -genkey -v -keystore dydl.keystore -alias dydl -keyalg RSA -keysize 2048 -validity 10000

# 2) 在 android-app/app/build.gradle.kts 配置 signingConfigs + 把 release 指向它
# 3) 构建
cd android-app && ./gradlew assembleRelease
```

## 四、首启使用流程

1. 装好 APK 打开 → 登录(用户名/密码来自服务端 `config.yml` 的 `auth`)。
2. 粘贴抖音视频/图集链接或整段分享文案 → 下载 → 进度条走完。
3. 视频出现在 **相册 / 文件管理 → Movies/抖音下载器**;图集 ZIP 在
   **Download/抖音下载器**;单图在 **Pictures/抖音下载器**。

## 工程结构速览

| 路径(app/src/main/java/io/github/nick/dydl/) | 职责 |
|---|---|
| `MainActivity.kt` | 唯一 Activity;顶栏(健康点/登出)+ SnackBar 容器 |
| `AppViewModel.kt` | 登录态 / 健康轮询 / 解析 / 下载状态流 |
| `api/ApiClient.kt`、`api/Models.kt` | REST 客户端(OkHttp + kotlinx.serialization) |
| `download/MediaStoreDownloader.kt` | 流式下载写入 MediaStore,进度 `Flow<Int>` |
| `ui/LoginScreen.kt`、`ui/SubmitScreen.kt` | 登录页 / 下载页(图集双方式弹窗) |
| `util/UrlExtractor.kt` | 从分享文案提取抖音链接(与网页版同正则) |

## 已知限制 / 排错

- **自签 HTTPS**:OkHttp 默认不信任自签证书,会连不上。
  - 推荐:用反代(Caddy / Nginx)+ Let's Encrypt 合法证书。
  - 或:仅内网使用时改用 HTTP 地址。
- **后台下载**:当前在 APP 前台时下载并显示应用内进度条;切到后台长任务可能被系统暂停。
  后续可加前台 Service 提升健壮性。
- **进不了相册**:确认手机系统版本 ≥ Android 10;部分厂商 ROM 的「相册」需手动刷新或
  从「文件管理 → Movies」查看;`adb shell ls Movies/抖音下载器/` 可核对文件确实落盘。
- **覆盖安装失败**(INSTALL_FAILED_UPDATE_INCOMPATIBLE):新旧 APK 签名不同
  (如 debug 与 release keystore 混用),先卸载旧版再装。
