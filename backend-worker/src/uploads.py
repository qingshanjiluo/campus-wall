"""图片上传存储：优先 D1（base64），R2 启用后切换到 R2。
设计要点：
- R2 账户未启用时，用 D1 表 uploads 存 base64，URL 为 /api/uploads/<key>
- R2 可用时，STORAGE_BACKEND 改为 'r2'，直接写对象存储
- 两种实现共用 save_upload / serve_upload 接口，路由层无需感知。"""
import base64
import mimetypes
import os
import uuid

import db as _db
import web as httpmod

STORAGE_BACKEND = 'd1'  # 'r2' 需要先开通 R2 并配置 wrangler.toml

ALLOWED_EXTS = ('png', 'jpg', 'jpeg', 'gif', 'webp')

# 前端使用 /static/uploads/<filename> 形式的 URL，这里返回同一形式保持兼容
PUBLIC_BASE = '/static/uploads'


def _ext_of(filename):
    return filename.rsplit('.', 1)[-1].lower() if '.' in filename else ''


async def save_upload(request, prefix='file', subdir=''):
    """解析 multipart 请求并保存文件。
    返回 (url, error)，url 为 None 表示失败。"""
    try:
        fd = await request.form_data()
    except Exception:
        return None, '请选择文件'

    file_val = fd.get('file')
    if file_val is None:
        return None, '请选择文件'
    if isinstance(file_val, str):
        return None, '请选择文件'

    f = file_val
    filename = getattr(f, 'name', '') or ''
    if not filename:
        return None, '请选择文件'

    ext = _ext_of(filename)
    if ext not in ALLOWED_EXTS:
        return None, '不支持的格式，请上传 png/jpg/gif/webp'

    try:
        data = await f.bytes()
    except Exception:
        return None, '读取文件失败'

    if not data:
        return None, '文件为空'
    if len(data) > 5 * 1024 * 1024:  # 5MB 上限
        return None, '文件过大（最大 5MB）'

    key = f'{prefix}_{uuid.uuid4().hex[:12]}.{ext}'
    ct = f'image/{ext}' if ext != 'jpg' else 'image/jpeg'

    if STORAGE_BACKEND == 'r2':
        bucket = httpmod.env_binding('UPLOADS')
        if bucket is None:
            return None, '存储服务未就绪'
        await bucket.put(key, data, httpMetadata={'contentType': ct})
    else:
        b64 = base64.b64encode(data).decode()
        try:
            await _db.execute(
                'INSERT INTO uploads (key, filename, content_type, data, size) VALUES (?, ?, ?, ?, ?)',
                (key, filename, ct, b64, len(data)))
        except Exception:
            return None, '存储失败，请重试'

    return f'{PUBLIC_BASE}/{subdir}/{key}' if subdir else f'{PUBLIC_BASE}/{key}', None


async def serve_upload(request, params):
    """GET /api/uploads/<path> 或 /static/uploads/<path>：回传上传的图片。
    path 可能形如 "posts/xxx.png" / "avatars/xxx" 或直接 "xxx.png"。"""
    path = params.get('path', '')
    if not path or '..' in path or '\\' in path:
        return httpmod.error('文件不存在', 404)
    # 支持可选的 subdir 前缀，key 取末段文件名
    key = path.rsplit('/', 1)[-1]
    if not key:
        return httpmod.error('文件不存在', 404)

    if STORAGE_BACKEND == 'r2':
        bucket = httpmod.env_binding('UPLOADS')
        if bucket is None:
            return httpmod.error('存储服务未就绪', 503)
        obj = await bucket.get(key)
        if obj is None:
            return httpmod.error('文件不存在', 404)
        data = (await obj.arrayBuffer()).to_bytes()
        ct = 'application/octet-stream'
        meta = getattr(obj, 'httpMetadata', None)
        if meta is not None:
            ct = meta.get('contentType', 'application/octet-stream') if hasattr(meta, 'get') else 'application/octet-stream'
    else:
        row = await _db.query('SELECT data, content_type FROM uploads WHERE key = ?', (key,), one=True)
        if not row:
            return httpmod.error('文件不存在', 404)
        data = base64.b64decode(row['data'])
        ct = row['content_type']

    return _bytes_response(data, ct)


def _bytes_response(data, content_type):
    from context import env
    from http import HTTPMethod  # noqa: F401
    try:
        from workers import Response
    except Exception:
        return httpmod.error('无法发送文件', 500)
    return Response(data, status=200, headers={
        'content-type': content_type,
        'cache-control': 'public, max-age=31536000',
    })
