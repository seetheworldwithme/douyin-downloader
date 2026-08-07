"""FastAPI 服务测试:验证 login / resolve / stream 的 HTTP 层。

不触达真实 Douyin API(用 monkeypatch 在解析早期就拦截)。
"""

import pytest

try:
    from fastapi.testclient import TestClient  # type: ignore
except ImportError:  # pragma: no cover
    pytest.skip("fastapi not installed", allow_module_level=True)


from config import ConfigLoader
from server.app import build_app, issue_token


def _make_app(tmp_path, *, auth=None, server=None):
    config = ConfigLoader(None)
    config.update(path=str(tmp_path))
    if auth:
        config.update(auth=auth)
    if server:
        config.update(server=server)
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


def test_login_with_users_list(tmp_path):
    """auth.users 列表里的账号可登录(与单账号共存,共享同一 secret)。"""
    app = _make_app(
        tmp_path,
        auth={
            "username": "u",
            "password": "p",
            "secret": "s",
            "users": [{"username": "alice", "password": "alice-pwd"}],
        },
    )
    with TestClient(app) as client:
        # 列表账号
        r1 = client.post("/api/v1/login", json={"username": "alice", "password": "alice-pwd"})
        assert r1.status_code == 200
        assert "token" in r1.json()
        # 单账号仍可用
        r2 = client.post("/api/v1/login", json={"username": "u", "password": "p"})
        assert r2.status_code == 200
        # 列表账号密码错
        r3 = client.post("/api/v1/login", json={"username": "alice", "password": "x"})
        assert r3.status_code == 401
        # 未知账号
        r4 = client.post("/api/v1/login", json={"username": "bob", "password": "x"})
        assert r4.status_code == 401


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


def test_cors_allows_capacitor_origin(tmp_path):
    """默认来源列表应放行 Capacitor WebView origin(https://localhost)。"""
    app = _make_app(tmp_path)
    with TestClient(app) as client:
        resp = client.options(
            "/api/v1/resolve",
            headers={
                "Origin": "https://localhost",
                "Access-Control-Request-Method": "POST",
            },
        )
        assert resp.status_code == 200
        assert resp.headers["access-control-allow-origin"] == "https://localhost"


def test_cors_config_override_to_wildcard(tmp_path):
    """server.cors_origins=['*'] 时应全放开(任意 origin 回显 *)。"""
    app = _make_app(tmp_path, server={"cors_origins": ["*"]})
    with TestClient(app) as client:
        resp = client.options(
            "/api/v1/resolve",
            headers={
                "Origin": "https://random.example.com",
                "Access-Control-Request-Method": "POST",
            },
        )
        assert resp.status_code == 200
        assert resp.headers["access-control-allow-origin"] == "*"


def test_cors_rejects_unknown_origin_by_default(tmp_path):
    """默认列表不含的 origin 不应回显(非通配模式下严格限制)。"""
    app = _make_app(tmp_path)
    with TestClient(app) as client:
        # 普通请求(非预检)带未授权 Origin:CORS 中间件不拦截响应,但不回显头
        resp = client.get("/api/v1/health", headers={"Origin": "https://evil.example.com"})
        assert resp.status_code == 200
        assert "access-control-allow-origin" not in resp.headers
