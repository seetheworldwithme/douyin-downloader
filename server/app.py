"""FastAPI REST 服务入口。

HTTP 层薄封装：
- 接收 URL，创建 job，返回 job_id
- 实际下载委托给 cli.main.download_url 的简化复用

fastapi/uvicorn 是**可选**依赖。若未安装，导入本模块会 ImportError。
"""

from __future__ import annotations

import pathlib
from contextlib import asynccontextmanager
from typing import Any, Dict, List, Optional

from fastapi import FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import FileResponse
from fastapi.staticfiles import StaticFiles
from pydantic import BaseModel

from auth import CookieManager
from config import ConfigLoader
from control import QueueManager, RateLimiter, RetryHandler
from core import DouyinAPIClient, DownloaderFactory, URLParser
from server.jobs import DownloadJob, JobManager
from server.progress import ServerProgressReporter
from storage import FileManager
from utils.logger import setup_logger
from utils.validators import is_short_url, normalize_short_url

logger = setup_logger("REST")

# web 前端的默认保存目录（仅作用于 web，不影响 CLI 全局默认）
WEB_DEFAULT_SAVE_DIR = "Video/douyin"

# 允许按请求覆盖的内容类型开关。视频 mp4 本体不在其中——它是核心产物，始终下载。
_OVERRIDABLE_CONTENT_KEYS = ("music", "cover", "avatar", "json")


class _RequestConfig:
    """按请求覆盖若干内容类型开关的 config 视图。

    下载链路对 config 只调用 ``.get(key)``（见 core/），所以只需转发 get，
    其余属性通过 ``__getattr__`` 回落到基础 ConfigLoader。
    """

    def __init__(self, base: ConfigLoader, overrides: Dict[str, bool]):
        self._base = base
        self._overrides = overrides

    def get(self, key: str, default: Any = None) -> Any:
        if key in self._overrides:
            return self._overrides[key]
        return self._base.get(key, default)

    def __getattr__(self, name: str) -> Any:
        # 只在 _base 已设置后触发；交由基础 ConfigLoader 提供其余能力。
        return getattr(self._base, name)


class DownloadRequest(BaseModel):
    url: str
    save_dir: Optional[str] = None
    # 可选附件内容类型开关；未提供（None）的字段沿用 config 默认
    content: Optional[Dict[str, bool]] = None


class JobResponse(BaseModel):
    job_id: str
    status: str
    url: str


class _ServerDeps:
    """跨请求复用的重量级依赖。

    REST 服务在进程生命周期内只需要一份 FileManager / RateLimiter / RetryHandler /
    QueueManager / CookieManager；每个请求重新构造既浪费又会触发文件系统 mkdir。
    DouyinAPIClient 由于持有 aiohttp.ClientSession，依旧按请求创建，避免跨请求泄漏
    连接状态或触发 "Session is closed" 错误。
    """

    def __init__(self, config: ConfigLoader):
        self.config = config
        # Resolve the cookie file path relative to the config file's directory
        # so the sidecar can find it regardless of its working directory (which
        # on macOS is often '/' when launched by Electron).
        if config.config_path:
            from pathlib import Path

            cookie_file = str(Path(config.config_path).resolve().parent / ".cookies.json")
        else:
            cookie_file = ".cookies.json"
        self.cookie_manager = CookieManager(cookie_file=cookie_file)
        # Load cookies from the config (env var / YAML cookie key) first, then
        # fall back to whatever is already on disk in the cookie file. This
        # ensures that cookies saved by a previous session are picked up on
        # restart even when the config doesn't embed them inline.
        initial_cookies = config.get_cookies()
        if initial_cookies:
            self.cookie_manager.set_cookies(initial_cookies)
        else:
            # Trigger a load from disk so get_cookies() returns the persisted
            # session without requiring a fresh login on every app restart.
            self.cookie_manager.get_cookies()
        self.file_manager = FileManager(config.get("path"))
        self.rate_limiter = RateLimiter(max_per_second=float(config.get("rate_limit", 2) or 2))
        self.retry_handler = RetryHandler(max_retries=int(config.get("retry_times", 3) or 3))
        self.queue_manager = QueueManager(max_workers=int(config.get("thread", 5) or 5))


async def _execute_download(job: DownloadJob, deps: "_ServerDeps") -> Dict[str, int]:
    """简化版 download_url：执行并把成功/失败计数返回给 job。

    有意不复用 cli.main.download_url —— 后者绑定了 progress_display 的 rich 状态。
    API client 仍按请求创建（aiohttp session 不跨请求复用）；其余重量级依赖从
    _ServerDeps 共享。

    每请求按 ``job.save_dir``（或 config 默认）构造一个 FileManager，使前端可
    指定保存目录；构造一个 ServerProgressReporter 把进度实时写回 job。
    """
    # 解析本次实际保存目录：优先用前端传入的 save_dir，否则回落 config.path
    save_dir = job.save_dir or deps.config.get("path")
    job.save_dir = save_dir  # 回填实际值供前端展示
    # FileManager 构造成本可接受（仅 mkdir，exist_ok=True）；按请求构造以支持
    # 前端自定义目录。_ServerDeps 里那份共享实例仅作为没有 save_dir 时的 fallback。
    file_manager = FileManager(save_dir) if save_dir else deps.file_manager
    reporter = ServerProgressReporter(job)
    # 按请求覆盖内容类型开关（白名单过滤，避免前端塞入无关键）
    overrides = {
        k: bool(v)
        for k, v in job.content.items()
        if k in _OVERRIDABLE_CONTENT_KEYS
    }
    config = _RequestConfig(deps.config, overrides) if overrides else deps.config
    url = job.url

    # proxy 与 cli.main.download_url 对齐:API 请求、短链解析和 CDN 媒体
    # 下载(downloader_base 读 api_client.proxy)统一走配置代理。
    async with DouyinAPIClient(
        deps.cookie_manager.get_cookies(),
        proxy=deps.config.get("proxy"),
    ) as api_client:
        if is_short_url(url):
            resolved = await api_client.resolve_short_url(normalize_short_url(url))
            if not resolved:
                raise RuntimeError(f"Failed to resolve short URL: {url}")
            url = resolved

        parsed = URLParser.parse(url)
        if not parsed:
            raise RuntimeError(f"Unsupported URL: {url}")

        downloader = DownloaderFactory.create(
            parsed["type"],
            config,
            api_client,
            file_manager,
            deps.cookie_manager,
            None,  # database 不在 server 场景里启用，避免单例冲突
            deps.rate_limiter,
            deps.retry_handler,
            deps.queue_manager,
            progress_reporter=reporter,
        )
        if downloader is None:
            raise RuntimeError(f"No downloader for url_type={parsed['type']}")

        result = await downloader.download(parsed)
        return {
            "total": result.total,
            "success": result.success,
            "failed": result.failed,
            "skipped": result.skipped,
        }


def build_app(config: ConfigLoader) -> FastAPI:
    deps = _ServerDeps(config)

    async def executor(job: DownloadJob) -> Dict[str, int]:
        return await _execute_download(job, deps)

    server_cfg = config.get("server") or {}
    if not isinstance(server_cfg, dict):
        server_cfg = {}
    manager = JobManager(
        executor=executor,
        max_concurrency=int(config.get("thread", 2) or 2),
        max_jobs=int(server_cfg.get("max_jobs") or JobManager.DEFAULT_MAX_JOBS),
        job_ttl_seconds=float(
            server_cfg.get("job_ttl_seconds") or JobManager.DEFAULT_JOB_TTL_SECONDS
        ),
    )

    @asynccontextmanager
    async def lifespan(app: FastAPI):
        yield
        await manager.shutdown()

    app = FastAPI(
        title="Douyin Downloader API",
        version="1.0",
        description="REST API for dispatching Douyin download jobs.",
        lifespan=lifespan,
    )
    app.state.job_manager = manager
    app.state.deps = deps

    @app.get("/api/v1/health")
    async def health() -> Dict[str, str]:
        return {"status": "ok"}

    @app.post("/api/v1/download", response_model=JobResponse)
    async def create_job(req: DownloadRequest) -> JobResponse:
        if not req.url:
            raise HTTPException(status_code=400, detail="url is required")
        job = await manager.submit(
            req.url,
            save_dir=(req.save_dir or None),
            content=(req.content or None),
        )
        return JobResponse(job_id=job.job_id, status=job.status, url=job.url)

    @app.get("/api/v1/defaults")
    async def defaults() -> Dict[str, str]:
        """返回 web 前端默认/当前保存目录，供 UI 预填与展示。"""
        return {
            "default_path": WEB_DEFAULT_SAVE_DIR,
            "current_path": config.get("path") or "",
        }

    @app.get("/api/v1/jobs/{job_id}")
    async def get_job(job_id: str) -> Dict[str, Any]:
        job = await manager.get(job_id)
        if job is None:
            raise HTTPException(status_code=404, detail="job not found")
        return job.to_dict()

    @app.get("/api/v1/jobs")
    async def list_jobs() -> Dict[str, List[Dict[str, Any]]]:
        jobs = await manager.list_jobs()
        return {"jobs": [j.to_dict() for j in jobs]}

    # 开发期:Vite dev server(默认 5173)跨域访问后端;生产同源时这条规则无害
    app.add_middleware(
        CORSMiddleware,
        allow_origins=[
            "http://localhost:5173",
            "http://127.0.0.1:5173",
        ],
        allow_methods=["*"],
        allow_headers=["*"],
    )

    # 托管前端构建产物(web/ 经 `npm run build` 输出到 server/static)
    static_dir = pathlib.Path(__file__).resolve().parent / "static"
    index_html = static_dir / "index.html"
    if index_html.exists():
        assets_dir = static_dir / "assets"
        if assets_dir.exists():
            app.mount(
                "/assets", StaticFiles(directory=str(assets_dir)), name="assets"
            )

        @app.get("/", include_in_schema=False)
        async def root() -> FileResponse:
            return FileResponse(str(index_html))

        # SPA 兜底:非 /api 路径一律回退到 index.html
        @app.get("/{path:path}", include_in_schema=False)
        async def spa_fallback(path: str) -> FileResponse:
            candidate = static_dir / path
            if path and candidate.is_file():
                return FileResponse(str(candidate))
            return FileResponse(str(index_html))
    else:
        logger.info(
            "前端未构建(server/static/index.html 不存在),仅运行 API。"
            "如需 Web 控制台:cd web && npm install && npm run build"
        )

    return app


async def run_server(config: ConfigLoader, *, host: str, port: int) -> None:
    import uvicorn

    app = build_app(config)
    uv_config = uvicorn.Config(app, host=host, port=port, log_level="info")
    server = uvicorn.Server(uv_config)
    await server.serve()
