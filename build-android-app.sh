#!/usr/bin/env bash
#
# build-android-app.sh —— Windows(Git Bash)一键构建原生安卓 App(Kotlin + Compose)
#
#   bash build-android-app.sh                          # debug 包(可直接安装)
#   BUILD_VARIANT=release bash build-android-app.sh    # release(需自配签名)
#
# 产出:release/apk/douyin-downloader.apk
# 安装:adb install -r release/apk/douyin-downloader.apk
#
# 前置:Git Bash 里运行(不要用 WSL);需已装 Android Studio(SDK + JDK 17~23)。
# 已内置国内适配:阿里云 Maven 镜像 + 腾讯云 android-35 平台镜像。
#
set -euo pipefail

cd "$(dirname "$0")"
ROOT="$(pwd)"
APP_DIR="$ROOT/android-app"
APK_OUT="$ROOT/release/apk/douyin-downloader.apk"
AS_JBR="/c/Program Files/Android/Android Studio/jbr"
MIRROR_INIT="$APP_DIR/mirror.init.gradle"
PLATFORM_ZIP_URL="https://mirrors.cloud.tencent.com/AndroidSDK/platform-35_r01.zip"
DL_TMP=""   # 平台包临时下载目录;绝不叫 TMP,避免与环境变量 TMP 冲突(trap 会 rm -rf 它!)

log()  { printf '\033[1;36m▶ %s\033[0m\n' "$*"; }
ok()   { printf '\033[1;32m✓ %s\033[0m\n' "$*"; }
warn() { printf '\033[1;33m⚠ %s\033[0m\n' "$*" >&2; }
die()  { printf '\033[1;31m✗ %s\033[0m\n' "$*" >&2; exit 1; }
trap '[ -n "${DL_TMP:-}" ] && rm -rf "$DL_TMP"' EXIT

# winpath: 转成 Windows 风格路径(C:/...)给 local.properties / powershell 用
winpath() { cygpath -m "$1" 2>/dev/null || echo "$1"; }

BUILD_VARIANT="${BUILD_VARIANT:-debug}"
case "$BUILD_VARIANT" in
  debug)   GRADLE_TASK="assembleDebug" ;;
  release) GRADLE_TASK="assembleRelease" ;;
  *) die "BUILD_VARIANT 只支持 debug / release" ;;
esac

[ -d "$APP_DIR" ] || die "未找到 android-app/ 工程"

# WSL 检测:WSL 里路径/工具链不通用,应改用 Git Bash
if grep -qi microsoft /proc/version 2>/dev/null; then
  die "检测到 WSL,本脚本面向 Git Bash;请在 Git Bash(或 VS Code 的 Git Bash 终端)里运行"
fi

# ── 1. 选 Gradle 兼容的 JDK(17~23;跳过 24/25,如新版 AS 自带 JBR 25) ──
jdk_major() { "$1/bin/java" -version 2>&1 | awk -F'"' 'NR==1{split($2,a,".");print a[1]}'; }
norm_home() { [ -d "$1" ] && echo "$1" || cygpath -u "$1" 2>/dev/null || true; }
try_pick() { # <dir> : 版本在 17~23 则写入 $pick 并返回 0
  local c m
  c="$(norm_home "$1")"
  [ -n "$c" ] && [ -x "$c/bin/java" ] || return 1
  m="$(jdk_major "$c")"
  if [ "${m:-0}" -ge 17 ] 2>/dev/null && [ "${m:-0}" -le 23 ] 2>/dev/null; then pick="$c"; return 0; fi
  return 1
}
pick=""
try_pick "${JAVA_HOME:-}"                                   && : # JAVA_HOME 优先
if [ -z "$pick" ]; then
  while IFS= read -r c; do try_pick "$c" && break; done < <({
    ls -d "/c/Program Files/Eclipse Adoptium"/jdk-*  2>/dev/null || true
    ls -d "/c/Program Files/Microsoft"/jdk-*         2>/dev/null || true
    ls -d "/c/Program Files/Java"/jdk-*              2>/dev/null || true
    ls -d "/c/Program Files/Zulu"/zulu-*             2>/dev/null || true
    ls -d "/e/Java"/jdk-*                            2>/dev/null || true
    echo "$AS_JBR"
  } | sort -uV | tac)   # 版本从高到低尝试:AGP 8.7 编译需要 Java 17+,优先选高版本
fi
[ -n "$pick" ] || die "找不到 JDK 17~23。装一个:https://adoptium.net (Temurin 17/21),或设置 JAVA_HOME 指向已有 JDK"
export JAVA_HOME="$pick"
ok "JAVA_HOME=$(winpath "$JAVA_HOME") (Java $(jdk_major "$JAVA_HOME"))"

# ── 2. Android SDK(Windows 默认在 %LOCALAPPDATA%\Android\Sdk,本机在 E:\Android\Sdk) ──
if [ -z "${ANDROID_HOME:-}" ] && [ -z "${ANDROID_SDK_ROOT:-}" ]; then
  [ -n "${LOCALAPPDATA:-}" ] && \
    DEFAULT_SDK="$(cygpath -u "$LOCALAPPDATA" 2>/dev/null || echo "$LOCALAPPDATA")/Android/Sdk"
  [ -d "$DEFAULT_SDK" ] || DEFAULT_SDK="/e/Android/Sdk"
  [ -d "$DEFAULT_SDK" ] && export ANDROID_HOME="$DEFAULT_SDK"
fi
{ [ -n "${ANDROID_HOME:-}" ] && [ -d "$ANDROID_HOME" ]; } || \
  die "找不到 Android SDK(期望 %LOCALAPPDATA%\\Android\\Sdk;装 Android Studio 即可,或设 ANDROID_HOME)"
ok "ANDROID_HOME=$(winpath "$ANDROID_HOME")"

# local.properties 指向 SDK(Windows 路径,正斜杠 Gradle 可识别)
LP="$APP_DIR/local.properties"
grep -q '^sdk.dir=' "$LP" 2>/dev/null || printf 'sdk.dir=%s\n' "$(winpath "$ANDROID_HOME")" > "$LP"

# ── 3. 缺 android-35 平台则从腾讯云镜像装 ──
unzip_one() { # <zip> <dest_dir>
  if command -v unzip >/dev/null; then
    unzip -q -o "$1" -d "$2"
  else
    powershell.exe -NoProfile -Command \
      "Expand-Archive -LiteralPath '$(cygpath -w "$1")' -DestinationPath '$(cygpath -w "$2")' -Force"
  fi
}
if [ ! -d "$ANDROID_HOME/platforms/android-35" ]; then
  log "下载 android-35 平台(国内镜像,约 60MB)"
  DL_TMP="$(mktemp -d)"
  curl -sL --fail -m 300 -o "$DL_TMP/p35.zip" "$PLATFORM_ZIP_URL" || \
    die "下载 $PLATFORM_ZIP_URL 失败;请改在 Android Studio 里装 Android SDK Platform 35 后重试"
  unzip_one "$DL_TMP/p35.zip" "$ANDROID_HOME/platforms/" || die "解压平台失败"
  [ -f "$ANDROID_HOME/platforms/android-35/android.jar" ] || die "平台安装后仍缺 android.jar"
  ok "已安装 platforms/android-35"
fi

# ── 4. 阿里云镜像 init.gradle(国内访问 dl.google.com 易 TLS 失败) ──
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

# ── 5. Gradle 构建(单测 + APK;首次会下载依赖,请耐心等待) ──
# Gradle(java.io.tmpdir)从 TMP/TEMP 读路径;后台/异构环境下可能是无效值,强制归一化
SYS_TMP="$(cygpath -u "${LOCALAPPDATA:-C:\\Windows\\Temp}" 2>/dev/null || echo /tmp)/Temp"
[ -d "$SYS_TMP" ] || SYS_TMP=/tmp
export TMP="$(cygpath -w "$SYS_TMP")"
export TEMP="$TMP"
chmod +x "$APP_DIR/gradlew" 2>/dev/null || true
log "Gradle test + $GRADLE_TASK"
(cd "$APP_DIR" && JAVA_HOME="$(winpath "$JAVA_HOME")" ./gradlew test "$GRADLE_TASK" --init-script "$MIRROR_INIT")

# ── 6. 定位并拷贝 APK ──
APK="$(find "$APP_DIR/app/build/outputs/apk" -name "*.apk" 2>/dev/null | head -1 || true)"
[ -n "$APK" ] && [ -f "$APK" ] || die "构建完成但没找到 APK"
mkdir -p "$(dirname "$APK_OUT")"
cp "$APK" "$APK_OUT"
echo
ok "打包完成:$APK_OUT ($(du -h "$APK_OUT" | cut -f1))"
echo
echo "安装到手机(开启 USB 调试并连电脑):"
echo "  \"$(winpath "$ANDROID_HOME")/platform-tools/adb.exe\" install -r \"$(winpath "$APK_OUT")\""
