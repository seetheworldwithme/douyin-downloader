#!/usr/bin/env bash
# 一键启动抖音下载器(在服务器上直接执行:`bash start.sh`):
#   1) npm run build 打包前端 → server-go/static
#   2) 编译 Go 后端(server-go)并在宿主机后台启动,监听 127.0.0.1:8000
# 对外由宿主机 nginx 直接托管:/(静态,直接读 server-go/static)+ /api/(反代 8000),
# 站点配置见 nginx/douyin.xuziyue.work.conf(改配置后 sudo systemctl reload nginx)。
# 访问:https://douyin.xuziyue.work/
set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$REPO_DIR"

SERVE_PORT="${SERVE_PORT:-8000}"
GO_VERSION_MIN="1.26"  # go.mod 要求 go 1.26.4

echo "== [1/2] 构建前端 (npm run build) =="
# PWA 的 service worker 压缩需要 Node 20+(全局 crypto);非交互 shell 不会自动激活 nvm,
# 这里显式 source nvm 并切到 LTS。无 nvm 则回退系统 node(若 <20 会构建失败)。
export NVM_DIR="$HOME/.nvm"
if [ -s "$NVM_DIR/nvm.sh" ]; then
  # nvm.sh 引用未定义变量,与 set -u 冲突,临时关闭
  set +u
  # shellcheck disable=SC1091
  source "$NVM_DIR/nvm.sh"
  nvm use --lts >/dev/null 2>&1 || true
  set -u
fi
echo "  node: $(node -v 2>&1)"
cd web
if [[ ! -d node_modules ]]; then
  echo "  node_modules 不存在,执行 npm ci..."
  npm ci
else
  # node_modules 已存在:用 npm install 增量同步(补齐 package.json 新增依赖,
  # 如 vite-plugin-pwa),避免旧目录缺包导致构建失败。
  echo "  同步依赖(npm install)..."
  npm install
fi
npm run build
cd "$REPO_DIR"
echo "  前端产物: $REPO_DIR/server-go/static"

echo "== [2/2] 编译并启动 Go 后端 (127.0.0.1:$SERVE_PORT) =="
if [[ -f .backend.pid ]] && kill -0 "$(cat .backend.pid)" 2>/dev/null; then
  echo "  停止旧后端 PID $(cat .backend.pid)"
  kill "$(cat .backend.pid)" || true
  sleep 1
fi

# 需要 go ≥ 1.26(go.mod: go 1.26.4)。非交互 shell 可能不在 PATH,
# 兼容常见安装位置(/usr/local/go、~/go、snap、homebrew)。
ensure_go() {
  if command -v go >/dev/null 2>&1; then return 0; fi
  for cand in /usr/local/go/bin/go /usr/lib/go/bin/go \
              "$HOME/go/bin/go" /snap/go/current/bin/go \
              /opt/homebrew/bin/go /usr/local/bin/go; do
    if [[ -x "$cand" ]]; then export PATH="$(dirname "$cand"):$PATH"; return 0; fi
  done
  return 1
}
if ! ensure_go; then
  echo "  找不到 go(需 ≥$GO_VERSION_MIN,见 https://go.dev/dl/)"; exit 1
fi
echo "  go: $(go version 2>&1)"

# 编译后端为静态二进制(modernc.org/sqlite 纯 Go,无需 CGO)
mkdir -p .bin
( cd server-go && CGO_ENABLED=0 go build -trimpath -o ../.bin/douyin-server ./cmd/server )

# 图集「合成视频」功能依赖宿主机 ffmpeg;缺失只告警不阻断启动(下载图片不受影响)
if ! command -v ffmpeg >/dev/null 2>&1; then
  echo "  ⚠ 未找到 ffmpeg:图集「合成视频下载」将不可用。安装:apt install ffmpeg"
fi

# 仅绑 loopback:宿主机 nginx 可达,公网不可达
mkdir -p logs
nohup ./.bin/douyin-server -config config.yml -host 127.0.0.1 -port "$SERVE_PORT" \
  > logs/backend.log 2>&1 &
echo $! > .backend.pid
disown 2>/dev/null || true
echo "  后端 PID $(cat .backend.pid) (日志: logs/backend.log)"

echo
echo "✓ 启动完成。"
echo "  - 前端静态: $REPO_DIR/server-go/static(宿主机 nginx 直接托管)"
echo "  - 后端 API: 127.0.0.1:$SERVE_PORT (日志: logs/backend.log)"
echo "  - 对外: https://douyin.xuziyue.work/  (nginx 站点: nginx/douyin.xuziyue.work.conf)"
echo "  - 停止后端: kill \$(cat .backend.pid)"
