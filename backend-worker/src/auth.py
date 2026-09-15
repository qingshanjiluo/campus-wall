"""认证模块：PBKDF2 密码哈希 + HS256 JWT。

bcrypt 在 Python Workers (Pyodide/WASM) 中无法加载 C 扩展，
故改用标准库 hashlib.pbkdf2_hmac 实现 PBKDF2-SHA256，参数与
浏览器 WebCrypto deriveKey('PBKDF2') 完全一致（可互操作）。

哈希格式：pbkdf2_sha256$迭代次数$盐(hex)$哈希(hex)
"""
import hashlib
import hmac
import base64
import json
import os
import time
from context import env, auth_header
import db as _db


PBKDF2_ITERATIONS = 100_000


# ── 密码哈希（PBKDF2-SHA256）──

def _b64(b):
    return base64.urlsafe_b64encode(b).rstrip(b'=').decode()


def _b64d(s):
    s += '=' * (-len(s) % 4)
    return base64.urlsafe_b64decode(s)


def hash_password(password):
    salt = os.urandom(16)
    dk = hashlib.pbkdf2_hmac('sha256', password.encode('utf-8'), salt, PBKDF2_ITERATIONS)
    return f"pbkdf2_sha256${PBKDF2_ITERATIONS}${salt.hex()}${dk.hex()}"


def verify_password(password, stored):
    try:
        _, iters, salt_hex, hash_hex = stored.split('$')
        dk = hashlib.pbkdf2_hmac('sha256', password.encode('utf-8'),
                                 bytes.fromhex(salt_hex), int(iters))
        return hmac.compare_digest(dk.hex(), hash_hex)
    except (ValueError, TypeError):
        return False


def is_legacy_bcrypt(stored):
    return bool(stored) and not stored.startswith('pbkdf2_sha256$')


# ── JWT (HS256) ──

def _jwt_secret():
    import context
    # 用 getattr 防止绑定缺失时直接抛异常；生产通过 wrangler secret 注入
    secret = getattr(context.env, 'JWT_SECRET', None)
    return secret or 'dev-only-insecure-secret'


def generate_token(user_id, days=None):
    import context
    days = days or int(context.env.JWT_EXPIRATION_DAYS or 30)
    header = _b64(json.dumps({'alg': 'HS256', 'typ': 'JWT'}, separators=(',', ':')).encode())
    payload = _b64(json.dumps({
        'user_id': user_id,
        'exp': int(time.time()) + days * 86400,
        'iat': int(time.time()),
    }, separators=(',', ':')).encode())
    sig = _b64(hmac.new(_jwt_secret().encode(), f'{header}.{payload}'.encode(),
                        hashlib.sha256).digest())
    return f'{header}.{payload}.{sig}'


def decode_token(token):
    try:
        header_b, payload_b, sig_b = token.split('.')
        data = f'{header_b}.{payload_b}'.encode()
        expected = hmac.new(_jwt_secret().encode(), data, hashlib.sha256).digest()
        if not hmac.compare_digest(_b64d(sig_b), expected):
            return None
        payload = json.loads(_b64d(payload_b))
        if payload.get('exp', 0) < time.time():
            return None
        return payload
    except Exception:
        return None


def generate_reset_token(user_id, minutes=30):
    """生成短期密码重置令牌（含 purpose 标记）。"""
    header = _b64(json.dumps({'alg': 'HS256', 'typ': 'JWT'}, separators=(',', ':')).encode())
    payload = _b64(json.dumps({
        'user_id': user_id,
        'purpose': 'password_reset',
        'exp': int(time.time()) + minutes * 60,
        'iat': int(time.time()),
    }, separators=(',', ':')).encode())
    sig = _b64(hmac.new(_jwt_secret().encode(), f'{header}.{payload}'.encode(),
                        hashlib.sha256).digest())
    return f'{header}.{payload}.{sig}'


def decode_reset_token(token):
    """解码重置令牌，非法/过期/用途不符返回 None，否则返回 payload。"""
    data = decode_token(token)
    if not data:
        return None
    if data.get('purpose') != 'password_reset':
        return None
    return data


async def get_token_user(token):
    """解析 token 并返回用户 dict（不存在返回 None）。"""
    data = decode_token(token)
    if not data:
        return None
    return await _db.query('SELECT * FROM users WHERE id = ?', (data['user_id'],), one=True)


async def require_user(request):
    """从请求头取 token 并返回用户，未认证返回 None。"""
    token = auth_header(request)
    if not token:
        return None
    return await get_token_user(token)


async def optional_user(request):
    """可选认证：有合法 token 返回用户，否则 None（不报错）。"""
    token = auth_header(request)
    if not token:
        return None
    return await get_token_user(token)