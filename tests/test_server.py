"""FastAPI 服务测试:验证 login / resolve / stream 的 HTTP 层。

不触达真实 Douyin API(用 monkeypatch 在解析早期就拦截)。
"""

import pytest

try:
    from fastapi.testclient import TestClient  # type: ignore
except ImportError:  # pragma: no cover
    pytest.skip("fastapi not installed", allow_module_level=True)

from fastapi import HTTPException

from config import ConfigLoader
from server.app import build_app, issue_token


def _make_app(tmp_path, *, auth=None):
    config = ConfigLoader(None)
    config.update(path=str(tmp_path))
    if auth:
        config.update(auth=auth)
    return build_app(config)


def _token(app):
    """用 app 自身 deps 签发合法 token,绕过登录端点。"""
    return issue_token("tester", app.state.deps.auth_secret)


def test_health_endpoint(tmp_path):
    app = _make_app(tmp_path)
    with TestClient(app) as client:
        resp = client.get("/api/v1/health")
        assert resp.status_code == 200
        assert resp.json() == {"status": "ok"}


def test_login_success(tmp_path):
    app = _make_app(tmp_path, auth={"username": "u", "password": "p", "secret": "s"})
    with TestClient(app) as client:
        resp = client.post("/api/v1/login", json={"username": "u", "password": "p"})
        assert resp.status_code == 200
        assert "token" in resp.json()


def test_login_wrong_password(tmp_path):
    app = _make_app(tmp_path, auth={"username": "u", "password": "p", "secret": "s"})
    with TestClient(app) as client:
        resp = client.post("/api/v1/login", json={"username": "u", "password": "wrong"})
        assert resp.status_code == 401


def test_stream_requires_auth(tmp_path):
    app = _make_app(tmp_path, auth={"username": "u", "password": "p", "secret": "s"})
    with TestClient(app) as client:
        resp = client.get("/api/v1/stream", params={"url": "https://www.douyin.com/video/1"})
        assert resp.status_code == 401


def test_resolve_requires_auth(tmp_path):
    app = _make_app(tmp_path, auth={"username": "u", "password": "p", "secret": "s"})
    with TestClient(app) as client:
        resp = client.post("/api/v1/resolve", json={"url": "https://www.douyin.com/video/1"})
        assert resp.status_code == 401


def test_resolve_rejects_non_video(tmp_path, monkeypatch):
    app = _make_app(tmp_path, auth={"username": "u", "password": "p", "secret": "s"})
    from server import app as server_app

    monkeypatch.setattr(server_app, "is_short_url", lambda _u: False)
    monkeypatch.setattr(
        server_app.URLParser,
        "parse",
        staticmethod(lambda _u: {"type": "gallery", "aweme_id": "1"}),
    )
    with TestClient(app) as client:
        resp = client.post(
            "/api/v1/resolve",
            json={"url": "https://www.douyin.com/note/1"},
            headers={"Authorization": f"Bearer {_token(app)}"},
        )
        assert resp.status_code == 400
        assert "video" in resp.json()["detail"]


def test_deps_loads_auth_credentials(tmp_path):
    app = _make_app(
        tmp_path, auth={"username": "u", "password": "p", "secret": "fixed-secret"}
    )
    assert app.state.deps.auth_username == "u"
    assert app.state.deps.auth_password == "p"
    assert app.state.deps.auth_secret == "fixed-secret"


def test_deps_generates_ephemeral_secret_when_missing(tmp_path):
    """secret 留空时应生成临时密钥(重启即失效),凭据其余字段仍读配置。"""
    app = _make_app(tmp_path, auth={"username": "u", "password": "p", "secret": ""})
    assert app.state.deps.auth_secret  # 临时生成,非空
    assert app.state.deps.auth_username == "u"
    assert app.state.deps.auth_password == "p"
