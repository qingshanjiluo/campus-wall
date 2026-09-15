"""HTTP 响应辅助：jsonify / 错误处理 / 请求解析。

API 兼容 Flask 的 jsonify 语义，返回统一 JSON 结构。
"""
import json
from http import HTTPMethod
from urllib.parse import urlparse, parse_qs

from workers import Response


def jsonify(data, status=200, headers=None):
    return Response.from_json(data, status=status, headers=headers or {})


def ok(data=None):
    return jsonify(data if data is not None else {'success': True})


def error(msg, status=400, extra=None):
    body = {'error': msg}
    if extra:
        body.update(extra)
    return jsonify(body, status=status)


def method_value(request):
    m = request.method
    return m.value if isinstance(m, HTTPMethod) else m


def get_headers(request):
    return request.headers


def get_auth_token(request):
    h = request.headers
    raw = h.get('Authorization') or ''
    if raw.startswith('Bearer '):
        return raw[len('Bearer '):]
    return raw


async def get_json_body(request):
    """安全解析 JSON body，失败返回 None。"""
    try:
        text = await request.text()
        return json.loads(text) if text else None
    except Exception:
        return None


def get_query_params(request):
    """返回 dict[str, str]（parse_qs 取首个值）。"""
    url = urlparse(request.url)
    return {k: v[0] for k, v in parse_qs(url.query).items()}


def get_path(request):
    return urlparse(request.url).path


def get_query(request):
    return urlparse(request.url).query


def get_env_secret(name, default=''):
    """读取环境变量（含 wrangler 的 [vars]，它们以字符串形式注入 env）。"""
    from context import env
    if env is None:
        return default
    val = getattr(env, name, None)
    if val is None:
        return default
    return str(val)


def env_binding(name):
    """获取 Workers binding（D1/R2/KV 等），未声明返回 None。"""
    from context import env
    if env is None:
        return None
    return getattr(env, name, None)