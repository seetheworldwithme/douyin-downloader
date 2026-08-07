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
cd web
if [[ ! -d node_modules ]]; then
  echo "  node_modules 不存在,执行 npm ci..."
  npm ci
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

if command -v conda >/dev/null 2>&1; then
  source "$(conda info --base)/etc/profile.d/conda.sh"
elif [[ -f "$HOME/miniconda3/etc/profile.d/conda.sh" ]]; then
  source "$HOME/miniconda3/etc/profile.d/conda.sh"
elif [[ -f "$HOME/anaconda3/etc/profile.d/conda.sh" ]]; then
  source "$HOME/anaconda3/etc/profile.d/conda.sh"
else
  echo "  找不到 conda(deploy.sh 用 bash -lc 调用本脚本以加载 conda 初始化)"
  exit 1
fi

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
