#!/usr/bin/env bash
# 一键启动抖音下载器(在服务器上执行),参照 calendar-site 模式:
#   1) npm run build 打包前端 → server/static
#   2) 用 conda env `dogs` 在宿主机后台启动后端(run.py --serve),监听 127.0.0.1:8000
#   3) docker compose 启动官方 nginx 镜像(127.0.0.1:8083,纯静态托管前端)
# 对外 HTTPS 由 edge-nginx 负责(见 docker/edge-proxy/douyin.xuziyue.work.conf)。
#
# 通常由 docker/deploy.sh 经 SSH 调用;也可在服务器上直接 `bash docker/start.sh`。
set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_DIR"

CONDA_ENV="${CONDA_ENV:-dogs}"
SERVE_PORT="${SERVE_PORT:-8000}"

echo "== [1/3] 构建前端 (npm run build) =="
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
  # 如 vite-plugin-pwa / @capacitor/*),避免旧目录缺包导致构建失败。
  echo "  同步依赖(npm install)..."
  npm install
fi
npm run build
cd "$REPO_DIR"
echo "  前端产物: $REPO_DIR/server/static"

echo "== [2/3] 启动后端 (conda env: $CONDA_ENV, 127.0.0.1:$SERVE_PORT) =="
if [[ -f .backend.pid ]] && kill -0 "$(cat .backend.pid)" 2>/dev/null; then
  echo "  停止旧后端 PID $(cat .backend.pid)"
  kill "$(cat .backend.pid)" || true
  sleep 1
fi

# conda 常不在非交互 shell 的 PATH 里(.bashrc 的 conda init 被交互检查挡住),
# 故显式 source conda.sh。覆盖常见安装位置(含本机的 ~/software/miniconda3)。
CONDA_SH=""
if command -v conda >/dev/null 2>&1; then
  CONDA_SH="$(conda info --base 2>/dev/null)/etc/profile.d/conda.sh"
fi
for cand in "$CONDA_SH" \
            "$HOME/software/miniconda3/etc/profile.d/conda.sh" \
            "$HOME/miniconda3/etc/profile.d/conda.sh" \
            "$HOME/anaconda3/etc/profile.d/conda.sh" \
            "/opt/conda/etc/profile.d/conda.sh"; do
  if [[ -n "$cand" && -f "$cand" ]]; then CONDA_SH="$cand"; break; fi
done
if [[ -z "$CONDA_SH" ]]; then
  echo "  找不到 conda.sh(确认 conda 已安装)"; exit 1
fi
set +u  # conda.sh 同样不耐 set -u
# shellcheck disable=SC1090
source "$CONDA_SH"
set -u

conda activate "$CONDA_ENV"
mkdir -p logs
# 仅绑 loopback:edge-nginx(host 网络)可达,公网不可达
nohup python run.py --serve --serve-host 127.0.0.1 --serve-port "$SERVE_PORT" -c config.yml \
  > logs/backend.log 2>&1 &
echo $! > .backend.pid
disown 2>/dev/null || true
echo "  后端 PID $(cat .backend.pid) (日志: logs/backend.log)"

echo "== [3/3] 启动前端 nginx 容器 (127.0.0.1:8083) =="
docker compose -f docker/docker-compose.yml up -d

echo
echo "✓ 启动完成。"
echo "  - 前端容器: douyin-web @ 127.0.0.1:8083"
echo "  - 后端 API: 127.0.0.1:$SERVE_PORT"
echo "  - 对外(经 edge-nginx): https://douyin.xuziyue.work/"
echo "  - 停止: docker compose -f docker/docker-compose.yml down ; kill \$(cat .backend.pid)"
