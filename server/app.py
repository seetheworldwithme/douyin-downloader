"""FastAPI REST 服务入口。

HTTP 层(流式下载到本地,服务器不落盘):
- ``POST /api/v1/login``   校验用户名密码,签发 HMAC token
- ``POST /api/v1/resolve`` 解析视频链接,返回标题/文件名(供前端预览确认)
- ``GET  /api/v1/stream``  流式透传无水印 mp4 给浏览器,触发原生另存为

视频字节流由服务器从抖音 CDN 中转给浏览器(防盗链决定了浏览器无法直连),
但**不写入服务器磁盘**。

fastapi/uvicorn 是**可选**依赖。若未安装,导入本模块会 ImportError。
"""

from __future__ import annotations

import base64
import hashlib
import hmac
import json
import pathlib
import secrets
import time
from typing import Any, Dict, Optional

from urllib.parse import quote

from fastapi import Depends, FastAPI, Header, HTTPException, Query, Request
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import FileResponse, StreamingResponse
from fastapi.staticfiles import StaticFiles
from pydantic import BaseModel

from auth import CookieManager
from config import ConfigLoader
from core import DouyinAPIClient, LoginRequiredError, URLParser
from core.video_downloader import VideoDownloader
from utils.logger import setup_logger
from utils.validators import is_short_url, normalize_short_url, sanitize_filename

logger = setup_logger("REST")

_TOKEN_TTL_SECONDS = 7 * 24 * 3600  # 个人用途,token 有效期 7 天
_DEFAULT_USERNAME = "xuziyue"
_DEFAULT_PASSWORD = "mmjsxu666555"


# --------------------------------------------------------------------------- #
# 认证:零依赖 HMAC 签名 token(类 JWT,HS256)
# --------------------------------------------------------------------------- #
def _b64(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode("ascii")


def _unb64(s: str) -> bytes:
    pad = "=" * (-len(s) % 4)
    return base64.urlsafe_b64decode(s + pad)


def issue_token(username: str, secret: str) -> str:
    header = _b64(json.dumps({"alg": "HS256", "typ": "JWT"}, separators=(",", ":")).encode())
    now = int(time.time())
    payload = _b64(
        json.dumps(
            {"sub": username, "iat": now, "exp": now + _TOKEN_TTL_SECONDS},
            separators=(",", ":"),
        ).encode()
    )
    signing_input = f"{header}.{payload}".encode()
    sig = hmac.new(secret.encode(), signing_input, hashlib.sha256).digest()
    return f"{header}.{payload}.{_b64(sig)}"


def verify_token(token: str, secret: str) -> Optional[dict]:
    try:
        header_b64, payload_b64, sig_b64 = token.split(".")
    except (ValueError, AttributeError):
        return None
    signing_input = f"{header_b64}.{payload_b64}".encode()
    expected = hmac.new(secret.encode(), signing_input, hashlib.sha256).digest()
    try:
        actual = _unb64(sig_b64)
    except Exception:
        return None
    if not hmac.compare_digest(expected, actual):
        return None
    try:
        payload = json.loads(_unb64(payload_b64))
    except Exception:
        return None
    if not isinstance(payload, dict):
        return None
    exp = payload.get("exp")
    if not isinstance(exp, int) or exp < int(time.time()):
        return None
    return payload


class LoginRequest(BaseModel):
    username: str
    password: str


class ResolveRequest(BaseModel):
    url: str


# --------------------------------------------------------------------------- #
# 跨请求复用的依赖
# --------------------------------------------------------------------------- #
class _ServerDeps:
    """流式模式下进程级共享依赖:config + cookie_manager + 认证凭据。"""

    def __init__(self, config: ConfigLoader):
        self.config = config
        # cookie 文件相对 config 解析,便于 sidecar 在任意工作目录找到它
        if config.config_path:
            cookie_file = str(
                pathlib.Path(config.config_path).resolve().parent / ".cookies.json"
            )
        else:
            cookie_file = ".cookies.json"
        self.cookie_manager = CookieManager(cookie_file=cookie_file)
        initial_cookies = config.get_cookies()
        if initial_cookies:
            self.cookie_manager.set_cookies(initial_cookies)
        else:
            # 触发从磁盘加载,使重启后仍持有上次保存的会话
            self.cookie_manager.get_cookies()

        # 认证凭据从 config 读;secret 留空则启动时生成临时密钥(重启后 token 失效)
        auth_cfg = config.get("auth") or {}
        if not isinstance(auth_cfg, dict):
            auth_cfg = {}
        self.auth_username = str(auth_cfg.get("username") or _DEFAULT_USERNAME)
        self.auth_password = str(auth_cfg.get("password") or _DEFAULT_PASSWORD)
        secret = str(auth_cfg.get("secret") or "").strip()
        if not secret:
            secret = secrets.token_urlsafe(32)
            logger.warning("auth.secret 未配置,使用临时密钥(重启后所有 token 失效)")
        self.auth_secret = secret


def _get_deps(request: Request) -> _ServerDeps:
    return request.app.state.deps


def require_user(
    request: Request,
    authorization: Optional[str] = Header(None),
    token: Optional[str] = Query(None),
) -> str:
    """校验 token。

    同时支持两种传输:``Authorization: Bearer``(普通 fetch)和 ``?token=``
    (浏览器导航触发的 GET 下载带不了 header)。任一有效即放行。
    """
    deps = _get_deps(request)
    raw = None
    if authorization and authorization.lower().startswith("bearer "):
        raw = authorization[7:].strip()
    elif token:
        raw = token
    if not raw:
        raise HTTPException(status_code=401, detail="未登录")
    payload = verify_token(raw, deps.auth_secret)
    if payload is None:
        raise HTTPException(status_code=401, detail="登录已过期,请重新登录")
    return payload.get("sub") or ""


# --------------------------------------------------------------------------- #
# 共享解析:resolve 与 stream 复用
# --------------------------------------------------------------------------- #
async def _resolve_video(url: str, deps: _ServerDeps) -> Dict[str, Any]:
    """短链 → 类型校验 → 详情 → 无水印直链 + 文件名。

    手动管理 ``api_client`` 生命周期(**不用 async with**):StreamingResponse
    会惰性消费响应体,若在此处用 ``async with`` 会在端点返回时关闭 session,
    导致流式下载读到已关闭的连接。调用方负责在合适时机 ``close()``。

    所有可预判失败均在此以 ``HTTPException`` 抛出(一旦返回 StreamingResponse,
    状态码即锁定 200,无法再改)。
    """
    api_client = DouyinAPIClient(
        deps.cookie_manager.get_cookies(),
        proxy=deps.config.get("proxy"),
    )
    await api_client._ensure_session()
    try:
        if is_short_url(url):
            resolved = await api_client.resolve_short_url(normalize_short_url(url))
            if not resolved:
                raise HTTPException(status_code=400, detail="短链解析失败")
            url = resolved

        parsed = URLParser.parse(url)
        if not parsed:
            raise HTTPException(status_code=400, detail="无法识别的抖音链接")
        if parsed.get("type") != "video":
            raise HTTPException(
                status_code=400,
                detail=f"仅支持视频(/video/)链接,当前类型: {parsed.get('type')}",
            )
        aweme_id = parsed.get("aweme_id")
        if not aweme_id:
            raise HTTPException(status_code=400, detail="未能从链接提取视频 ID")

        try:
            aweme_data = await api_client.get_video_detail(aweme_id)
        except LoginRequiredError:
            raise HTTPException(
                status_code=401,
                detail="抖音 Cookie 已过期,请更新 config.yml 中的 cookies",
            )
        if not aweme_data:
            raise HTTPException(
                status_code=502,
                detail="获取视频详情失败(Cookie 可能过期或被风控)",
            )

        # 复用 VideoDownloader 的无水印直链选择 + X-Bogus 签名逻辑(纯方法,不碰盘)。
        # 调用其私有方法是可接受的折中:把它改成公开 API 会触及共享 core,需同步桌面端;
        # 此处保持零侵入。file_manager=None 安全,因为该方法不使用它。
        downloader = VideoDownloader(
            config=deps.config,
            api_client=api_client,
            file_manager=None,
            cookie_manager=deps.cookie_manager,
        )
        video_info = downloader._build_no_watermark_url(aweme_data)
        if not video_info:
            raise HTTPException(status_code=404, detail="未找到可播放的视频地址")
        video_url, video_headers = video_info

        title = (aweme_data.get("desc") or "").strip() or str(aweme_id)
        filename = f"{sanitize_filename(title, max_length=80)}.mp4"

        return {
            "api_client": api_client,
            "video_url": video_url,
            "video_headers": video_headers,
            "filename": filename,
            "aweme_id": aweme_id,
            "title": title,
        }
    except HTTPException:
        await api_client.close()
        raise
    except Exception as exc:
        await api_client.close()
        logger.exception("resolve_video failed")
        raise HTTPException(status_code=500, detail=f"内部错误: {exc}")


# --------------------------------------------------------------------------- #
# 应用构建
# --------------------------------------------------------------------------- #
def build_app(config: ConfigLoader) -> FastAPI:
    deps = _ServerDeps(config)

    app = FastAPI(
        title="Douyin Downloader API",
        version="2.0",
        description="Stream Douyin videos to the browser (no server-side storage).",
    )
    app.state.deps = deps

    @app.get("/api/v1/health")
    async def health() -> Dict[str, str]:
        return {"status": "ok"}

    @app.post("/api/v1/login")
    async def login(req: LoginRequest) -> Dict[str, str]:
        ok = hmac.compare_digest(req.username or "", deps.auth_username) and hmac.compare_digest(
            req.password or "", deps.auth_password
        )
        if not ok:
            raise HTTPException(status_code=401, detail="用户名或密码错误")
        return {"token": issue_token(req.username, deps.auth_secret)}

    @app.post("/api/v1/resolve")
    async def resolve(
        req: ResolveRequest, _user: str = Depends(require_user)
    ) -> Dict[str, str]:
        if not req.url:
            raise HTTPException(status_code=400, detail="url is required")
        info = await _resolve_video(req.url, deps)
        try:
            return {
                "title": info["title"],
                "filename": info["filename"],
                "aweme_id": info["aweme_id"],
            }
        finally:
            await info["api_client"].close()

    @app.get("/api/v1/stream")
    async def stream(
        url: str = Query(..., description="Douyin video URL or short link"),
        _user: str = Depends(require_user),
    ) -> StreamingResponse:
        info = await _resolve_video(url, deps)
        api_client = info["api_client"]
        video_url = info["video_url"]
        video_headers = info["video_headers"]
        filename = info["filename"]

        try:
            session = await api_client.get_session()
            # proxy 与 _request_json / _download_with_retry 对齐:per-request 传
            upstream = await session.get(
                video_url,
                headers=video_headers,
                proxy=api_client.proxy or None,
            )
            if upstream.status != 200:
                body = ""
                try:
                    body = (await upstream.text())[:200]
                except Exception:
                    pass
                upstream.release()
                raise HTTPException(
                    status_code=502, detail=f"上游返回 {upstream.status}: {body}"
                )

            # 中文文件名双编码(RFC 5987):ASCII 回退 + filename*
            encoded = quote(filename)
            ascii_fallback = (
                filename.encode("ascii", "ignore").decode("ascii").replace('"', "").strip()
                or "video.mp4"
            )
            content_disposition = (
                f"attachment; filename=\"{ascii_fallback}\"; filename*=UTF-8''{encoded}"
            )
            upstream_ct = upstream.headers.get("Content-Type", "video/mp4")
            upstream_cl = upstream.headers.get("Content-Length")
            response_headers = {"Content-Disposition": content_disposition}
            if upstream_cl:
                response_headers["Content-Length"] = upstream_cl
        except HTTPException:
            await api_client.close()
            raise

        # 生成器持有 upstream + api_client 生命周期:正常完成 / 客户端中途断开 /
        # 异常,三种情况都会进入 finally 释放上游并关闭 api_client。
        async def generate():
            try:
                async for chunk in upstream.content.iter_chunked(256 * 1024):
                    yield chunk
            finally:
                try:
                    upstream.release()
                except Exception:
                    pass
                await api_client.close()

        return StreamingResponse(
            generate(), media_type=upstream_ct, headers=response_headers
        )

    # 开发期:Vite dev server(默认 5173)跨域访问后端;生产同源时这条规则无害
    app.add_middleware(
        CORSMiddleware,
        allow_origins=["http://localhost:5173", "http://127.0.0.1:5173"],
        allow_methods=["*"],
        allow_headers=["*"],
    )

    # 托管前端构建产物(web/ 经 `npm run build` 输出到 server/static)
    static_dir = pathlib.Path(__file__).resolve().parent / "static"
    index_html = static_dir / "index.html"
    if index_html.exists():
        assets_dir = static_dir / "assets"
        if assets_dir.exists():
            app.mount("/assets", StaticFiles(directory=str(assets_dir)), name="assets")

        @app.get("/", include_in_schema=False)
        async def root() -> FileResponse:
            return FileResponse(
                str(index_html),
                headers={"Cache-Control": "no-cache, no-store, must-revalidate"},
            )

        # SPA 兜底:非 /api 路径一律回退到 index.html
        @app.get("/{path:path}", include_in_schema=False)
        async def spa_fallback(path: str) -> FileResponse:
            candidate = static_dir / path
            if path and candidate.is_file():
                return FileResponse(str(candidate))
            return FileResponse(
                str(index_html),
                headers={"Cache-Control": "no-cache, no-store, must-revalidate"},
            )
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
