#!/usr/bin/env bash
#
# build-android.sh —— 一键把 web 前端打成安卓安装包(debug APK)
#
#   bash build-android.sh                          # debug 包(可直接安装)
#   BUILD_VARIANT=release bash build-android.sh    # release(需自配签名)
#
# 产出:.bin/douyin-downloader.apk
# 安装:~/Library/Android/sdk/platform-tools/adb install -r .bin/douyin-downloader.apk
#
# 幂等:可重复执行;首次会 cap add android + 下载 Gradle/AGP 依赖(较慢)。
# 已内置国内适配:阿里云 Maven 镜像 + 腾讯云 android-35 平台镜像 + 自动挑兼容 JDK。
#
set -euo pipefail

cd "$(dirname "$0")"
ROOT="$(pwd)"
WEB="$ROOT/web"
ANDROID_DIR="$WEB/android"
PKG_DOT="io.github.nick.dydl"            # capacitor.config.json 的 appId
JAVA_DIR_REL="io/github/nick/dydl"
PLUGIN_JAVA="$ROOT/mobile/native/SaveToGalleryPlugin.java"
APK_OUT="$ROOT/.bin/douyin-downloader.apk"
DEFAULT_SDK="$HOME/Library/Android/sdk"
AS_JBR="/Applications/Android Studio.app/Contents/jbr/Contents/Home"
MIRROR_INIT="$ANDROID_DIR/mirror.init.gradle"
PLATFORM_ZIP_URL="https://mirrors.cloud.tencent.com/AndroidSDK/platform-35_r01.zip"
TMP=""

log()  { printf '\033[1;36m▶ %s\033[0m\n' "$*"; }
ok()   { printf '\033[1;32m✓ %s\033[0m\n' "$*"; }
warn() { printf '\033[1;33m⚠ %s\033[0m\n' "$*" >&2; }
die()  { printf '\033[1;31m✗ %s\033[0m\n' "$*" >&2; exit 1; }
trap '[ -n "${TMP:-}" ] && rm -rf "$TMP"' EXIT

BUILD_VARIANT="${BUILD_VARIANT:-debug}"
case "$BUILD_VARIANT" in
  debug)   GRADLE_TASK="assembleDebug" ;;
  release) GRADLE_TASK="assembleRelease" ;;
  *) die "BUILD_VARIANT 只支持 debug / release" ;;
esac

command -v node >/dev/null || die "未找到 node(先装 Node)"
command -v npx  >/dev/null || die "未找到 npx"
command -v curl >/dev/null || die "未找到 curl"
command -v unzip >/dev/null || die "未找到 unzip"

# ── 1. 选 Gradle 兼容的 JDK(17~23;跳过 24/25,如 AS 自带 JBR 25) ──
jdk_major() { "$1/bin/java" -version 2>&1 | awk -F'"' 'NR==1{split($2,a,".");print a[1]}'; }
hb() { command -v brew >/dev/null 2>&1 && brew --prefix "$1" 2>/dev/null || true; }
pick=""
for c in \
  "${JAVA_HOME:-}" \
  "$(hb openjdk@17)/libexec/openjdk.jdk/Contents/Home" \
  "$(hb openjdk@21)/libexec/openjdk.jdk/Contents/Home" \
  "$(hb openjdk)/libexec/openjdk.jdk/Contents/Home" \
  "$(/usr/libexec/java_home -v17 2>/dev/null || true)" \
  "$(/usr/libexec/java_home -v21 2>/dev/null || true)" \
  "$AS_JBR" ; do
  [ -n "$c" ] && [ -x "$c/bin/java" ] || continue
  m="$(jdk_major "$c")"
  if [ "${m:-0}" -ge 17 ] 2>/dev/null && [ "${m:-0}" -le 23 ] 2>/dev/null; then pick="$c"; break; fi
done
export JAVA_HOME="$pick"
[ -n "$JAVA_HOME" ] || die "找不到 JDK 17~23(本机 Android Studio 的 JBR 是 25,Gradle 8.11 不支持)。装一个:brew install openjdk@17"
ok "JAVA_HOME=$JAVA_HOME (Java $(jdk_major "$JAVA_HOME"))"

# ── 2. Android SDK ──
if [ -z "${ANDROID_HOME:-}" ] && [ -z "${ANDROID_SDK_ROOT:-}" ]; then
  [ -d "$DEFAULT_SDK" ] && export ANDROID_HOME="$DEFAULT_SDK"
fi
{ [ -n "${ANDROID_HOME:-}" ] && [ -d "$ANDROID_HOME" ]; } || \
  die "找不到 Android SDK(期望 $DEFAULT_SDK;装 Android Studio 即可)"
ok "ANDROID_HOME=$ANDROID_HOME"

# ── 3. web 依赖 ──
log "安装 web 依赖(若需要)"
[ -d "$WEB/node_modules" ] || (cd "$WEB" && npm install)

# ── 4. 构建前端(native 变体,无 PWA Service Worker) ──
log "构建前端(BUILD_TARGET=capacitor)"
(cd "$WEB" && npm run build:native)

# ── 5. 创建 android 工程(仅首次) ──
if [ ! -d "$ANDROID_DIR" ]; then
  log "cap add android(首次)"
  (cd "$WEB" && npx cap add android)
fi
# local.properties 指向 SDK
LP="$ANDROID_DIR/local.properties"
grep -q '^sdk.dir=' "$LP" 2>/dev/null || printf 'sdk.dir=%s\n' "$ANDROID_HOME" > "$LP"
ok "android 工程就绪"

# ── 6. 缺 android-35 平台则从腾讯云镜像装(Capacitor 7 需 compileSdk 35) ──
if [ ! -d "$ANDROID_HOME/platforms/android-35" ]; then
  log "下载 android-35 平台(国内镜像,约 60MB)"
  TMP="$(mktemp -d)"
  curl -sL --fail -m 300 -o "$TMP/p35.zip" "$PLATFORM_ZIP_URL" || \
    die "下载 $PLATFORM_ZIP_URL 失败;请改在 Android Studio 里装 Android SDK Platform 35 后重试"
  unzip -q -o "$TMP/p35.zip" -d "$ANDROID_HOME/platforms/" || die "解压平台失败"
  [ -f "$ANDROID_HOME/platforms/android-35/android.jar" ] || die "平台安装后仍缺 android.jar"
  ok "已安装 platforms/android-35"
fi

# ── 7. 应用补丁(全部幂等) ──
# 7a. minSdkVersion = 29(MediaStore RELATIVE_PATH 需要)
VG="$ANDROID_DIR/variables.gradle"
if [ -f "$VG" ] && ! grep -qE 'minSdkVersion[[:space:]]*=[[:space:]]*29' "$VG"; then
  sed -i '' -E 's/minSdkVersion[[:space:]]*=[[:space:]]*[0-9]+/minSdkVersion = 29/' "$VG"
  ok "variables.gradle: minSdkVersion -> 29"
fi

# 7b. OkHttp 依赖(SaveToGalleryPlugin 拉流用)
BG="$(ls "$ANDROID_DIR/app/build.gradle" "$ANDROID_DIR/app/build.gradle.kts" 2>/dev/null | head -1 || true)"
if [ -n "$BG" ] && ! grep -q 'okhttp3' "$BG"; then
  case "$BG" in
    *.kts) DEP='    implementation("com.squareup.okhttp3:okhttp:4.12.0")' ;;
    *)     DEP="    implementation 'com.squareup.okhttp3:okhttp:4.12.0'" ;;
  esac
  awk -v dep="$DEP" '/dependencies[[:space:]]*{/ && !d {print; print dep; d=1; next} {print}' "$BG" > "$BG.tmp" && mv "$BG.tmp" "$BG"
  ok "加入 OkHttp 依赖"
fi

# 7c. 复制 Java 原生插件(Capacitor 模板不带 Kotlin,故用 Java)
DST_DIR="$ANDROID_DIR/app/src/main/java/$JAVA_DIR_REL"
mkdir -p "$DST_DIR"
cp "$PLUGIN_JAVA" "$DST_DIR/SaveToGalleryPlugin.java"
rm -f "$DST_DIR/SaveToGalleryPlugin.kt"
ok "复制 SaveToGalleryPlugin.java"

# 7d. 注册本地插件(关键!app 内插件不会自动注册,必须 registerPlugin)
MA_FILE="$(find "$ANDROID_DIR/app/src/main/java" -name 'MainActivity.*' 2>/dev/null | head -1 || true)"
if [ -n "$MA_FILE" ] && ! grep -q 'registerPlugin' "$MA_FILE"; then
  case "$MA_FILE" in
    *.kt) cat > "$MA_FILE" <<KOTLIN
package $PKG_DOT

import android.os.Bundle
import com.getcapacitor.BridgeActivity

class MainActivity : BridgeActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        registerPlugin(SaveToGalleryPlugin::class.java)
        super.onCreate(savedInstanceState)
    }
}
KOTLIN
      ;;
    *) cat > "$MA_FILE" <<JAVA
package $PKG_DOT;

import android.os.Bundle;
import com.getcapacitor.BridgeActivity;

public class MainActivity extends BridgeActivity {
    @Override
    public void onCreate(Bundle savedInstanceState) {
        registerPlugin(SaveToGalleryPlugin.class);
        super.onCreate(savedInstanceState);
    }
}
JAVA
      ;;
  esac
  ok "MainActivity 注册 SaveToGalleryPlugin"
fi

# ── 8. 阿里云镜像 init.gradle(国内访问 dl.google.com 易 TLS 失败) ──
mkdir -p "$ANDROID_DIR"
cat > "$MIRROR_INIT" <<'INITGRADLE'
def mirror = { r ->
    try {
        if (r != null && r.metaClass.respondsTo(r, "getUrl") && r.url) {
            def u = r.url.toString()
            if (u.contains("dl.google.com") || u.contains("maven.google.com")) r.url = "https://maven.aliyun.com/repository/google"
            else if (u.contains("repo1.maven.org/maven2") || u.contains("repo.maven.apache.org")) r.url = "https://maven.aliyun.com/repository/central"
            else if (u.contains("plugins.gradle.org")) r.url = "https://maven.aliyun.com/repository/gradle-plugin"
        }
    } catch (Exception ignore) {}
}
allprojects {
    buildscript { repositories { all { mirror(it) } } }
    repositories { all { mirror(it) } }
}
settingsEvaluated { s ->
    try { s.pluginManagement.repositories.all { mirror(it) } } catch (Exception ignore) {}
    try { s.dependencyResolutionManagement.repositories.all { mirror(it) } } catch (Exception ignore) {}
}
INITGRADLE
ok "写入阿里云 Maven 镜像"

# ── 9. 安卓图标(失败则用默认图标,不影响构建) ──
log "生成安卓图标"
(cd "$WEB" && npm run assets:android) || warn "图标生成跳过(将用默认图标)"

# ── 10. 同步 web 产物 + 插件 ──
log "cap sync android"
(cd "$WEB" && npx cap sync android)

# ── 11. Gradle 构建 APK ──
chmod +x "$ANDROID_DIR/gradlew" 2>/dev/null || true
log "Gradle $GRADLE_TASK(首次会下载依赖,请耐心等待)"
(cd "$ANDROID_DIR" && ./gradlew "$GRADLE_TASK" --init-script "$MIRROR_INIT")

# ── 12. 定位并拷贝 APK ──
APK="$(find "$ANDROID_DIR/app/build/outputs/apk" -name "*.apk" 2>/dev/null | head -1 || true)"
[ -n "$APK" ] && [ -f "$APK" ] || die "构建完成但没找到 APK"
mkdir -p "$(dirname "$APK_OUT")"
cp "$APK" "$APK_OUT"
echo
ok "打包完成:$APK_OUT ($(du -h "$APK_OUT" | cut -f1))"
echo
echo "安装到手机(开启 USB 调试并连电脑):"
echo "  \"$ANDROID_HOME/platform-tools/adb\" install -r \"$APK_OUT\""
