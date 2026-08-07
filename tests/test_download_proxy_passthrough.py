"""Regression: 解析路径必须透传 ``config['proxy']`` 给 ``DouyinAPIClient``。

历史上 ``server.app._execute_download`` 构造 ``DouyinAPIClient(cookies)`` 时漏传
proxy,导致在需要代理的网络里 API/CDN 请求全走直连而失败。``_execute_download``
已被流式 ``_resolve_video`` 取代,这里把同一契约钉在新调用点上。

core 的 ``retry_executor`` 调用点一并无关本次重构,继续保留覆盖。
"""

from __future__ import annotations

import asyncio
from typing import Any, Dict, List, Optional

import pytest
from fastapi import HTTPException

from config.config_loader import ConfigLoader


class _RecordingAPIClient:
    """替代 DouyinAPIClient,记录构造时的 ``proxy``。"""

    seen_proxies: List[Optional[str]] = []

    def __init__(self, cookies, proxy: Optional[str] = None, **_kw):
        _RecordingAPIClient.seen_proxies.append(proxy)

    @classmethod
    def reset(cls):
        cls.seen_proxies = []

    async def __aenter__(self):
        return self

    async def __aexit__(self, *_a):
        return None

    async def _ensure_session(self):
        return None

    async def close(self):
        return None

    async def resolve_short_url(self, _url):
        return None

    async def get_video_detail(self, _aweme_id):
        # 返回 None 让 _resolve_video 在校验阶段抛 502,proxy 此时已记录
        return None


def _run_resolve_video(monkeypatch, tmp_path, config_updates: Dict[str, Any]):
    from server import app as server_app
    from server.app import _resolve_video, _ServerDeps

    deps = _ServerDeps(ConfigLoader(None))
    deps.config.update(path=str(tmp_path), **config_updates)

    _RecordingAPIClient.reset()
    monkeypatch.setattr(server_app, "DouyinAPIClient", _RecordingAPIClient)
    monkeypatch.setattr(server_app, "is_short_url", lambda _u: False)
    monkeypatch.setattr(
        server_app.URLParser,
        "parse",
        staticmethod(lambda _u: {"type": "video", "aweme_id": "1"}),
    )

    with pytest.raises(HTTPException):
        asyncio.run(
            _resolve_video(
                "https://www.douyin.com/video/7000000000000000001", deps
            )
        )
    return _RecordingAPIClient.seen_proxies


def test_resolve_video_passes_configured_proxy(monkeypatch, tmp_path):
    seen = _run_resolve_video(
        monkeypatch, tmp_path, {"proxy": "http://127.0.0.1:7890"}
    )
    assert seen == ["http://127.0.0.1:7890"]


def test_resolve_video_without_proxy_stays_direct(monkeypatch, tmp_path):
    seen = _run_resolve_video(monkeypatch, tmp_path, {})
    assert len(seen) == 1
    assert not seen[0]  # None or "" — DouyinAPIClient 两都归一


def test_retry_executor_passes_configured_proxy(monkeypatch, tmp_path):
    from core import retry_executor as retry_mod
    from storage.file_manager import FileManager

    class _StubCookieManager:
        def get_cookies(self):
            return {}

    config = ConfigLoader(None)
    config.update(path=str(tmp_path), proxy="http://127.0.0.1:7890")

    _RecordingAPIClient.reset()
    monkeypatch.setattr(retry_mod, "DouyinAPIClient", _RecordingAPIClient)
    # 工厂返回 None 立即中止,proxy 捕获已足够
    monkeypatch.setattr(
        retry_mod.DownloaderFactory,
        "create",
        staticmethod(lambda *_a, **_kw: None),
    )

    with pytest.raises(RuntimeError, match="No downloader available for retry"):
        asyncio.run(
            retry_mod.retry_failed_awemes(
                "https://www.douyin.com/video/7000000000000000001",
                aweme_ids=["7000000000000000001"],
                config=config,
                file_manager=FileManager(str(tmp_path)),
                cookie_manager=_StubCookieManager(),
            )
        )

    assert _RecordingAPIClient.seen_proxies == ["http://127.0.0.1:7890"]
