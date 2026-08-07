#!/usr/bin/env bash
# 一键启动前后端(在服务器上执行):
#   1) npm run build 打包前端 → server/static
#   2) 用 conda env `dogs` 在宿主机后台启动后端(run.py --serve),监听 127.0.0.1:8000
#   3) docker compose 启动官方 nginx 镜像,托管前端 + 反代 /api
#
# 通常由 docker/deploy.sh 经 SSH 调用;也可在服务器上直接 `bash docker/start.sh`。
set -euo pipefail

# 定位仓库根(start.sh 位于 docker/)
REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_DIR"

CONDA_ENV="${CONDA_ENV:-dogs}"
SERVE_PORT="${SERVE_PORT:-8000}"

echo "== [1/3] 构建前端 (npm run build) =="
cd web
if [[ ! -d node_modules ]]; then
  echo "  node_modules 不存在,执行 npm ci..."
  npm ci
fi
npm run build
cd "$REPO_DIR"
echo "  前端产物: $REPO_DIR/server/static"

echo "== [2/3] 启动后端 (conda env: $CONDA_ENV) =="
# 停掉旧的后端进程(如有)
if [[ -f .backend.pid ]] && kill -0 "$(cat .backend.pid)" 2>/dev/null; then
  echo "  停止旧后端 PID $(cat .backend.pid)"
  kill "$(cat .backend.pid)" || true
  sleep 1
fi

# 非交互 shell 需手动初始化 conda
if command -v conda >/dev/null 2>&1; then
  source "$(conda info --base)/etc/profile.d/conda.sh"
elif [[ -f "$HOME/miniconda3/etc/profile.d/conda.sh" ]]; then
  source "$HOME/miniconda3/etc/profile.d/conda.sh"
elif [[ -f "$HOME/anaconda3/etc/profile.d/conda.sh" ]]; then
  source "$HOME/anaconda3/etc/profile.d/conda.sh"
else
  echo "  找不到 conda,请确认已安装并在 PATH(deploy.sh 用 bash -lc 调用本脚本)"
  exit 1
fi

conda activate "$CONDA_ENV"
mkdir -p logs
# 后端只绑 127.0.0.1,外网不可达;nginx 通过 host 网络反代
nohup python run.py --serve --serve-host 127.0.0.1 --serve-port "$SERVE_PORT" -c config.yml \
  > logs/backend.log 2>&1 &
echo $! > .backend.pid
disown 2>/dev/null || true
echo "  后端 PID $(cat .backend.pid) (日志: logs/backend.log)"

echo "== [3/3] 启动 nginx (docker compose) =="
docker compose -f docker/docker-compose.yml up -d

echo
echo "✓ 启动完成。"
echo "  - 前端/nginx: http://<服务器IP或域名>/  (容器 dydl-nginx,host 网络 80)"
echo "  - 后端 API:   127.0.0.1:$SERVE_PORT (仅本机,经 nginx /api 对外)"
echo "  - 停止: docker compose -f docker/docker-compose.yml down ; kill \$(cat .backend.pid)"
