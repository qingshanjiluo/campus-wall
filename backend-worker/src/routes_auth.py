"""认证路由：注册 / 登录 / 个人资料 / 密码管理。
handler 签名统一为 async fn(request, params)，由 entry.Router 分发。"""
import json
import re
import uuid

import web as httpmod
import auth as authmod
import db as _db
from models import (
    create_user, get_user_by_username, get_user_by_email, get_user_by_id,
    verify_password, update_user, get_user_stats, change_password,
    public_user,
)
from uploads import save_upload

from http import HTTPMethod


async def _require_user(request):
    """强制登录，返回 (user, response)。response 非 None 表示应直接返回。"""
    user = await authmod.require_user(request)
    if not user:
        return None, httpmod.error('缺少认证令牌', 401)
    return user, None


def _user_dict(user):
    return public_user(user)


async def register(request, params):
    data = await httpmod.get_json_body(request) or {}
    username = (data.get('username') or '').strip()
    email = (data.get('email') or '').strip()
    password = data.get('password') or ''

    if not username or len(username) < 2:
        return httpmod.error('用户名至少2个字符', 400)
    if len(username) > 30:
        return httpmod.error('用户名最大30个字符', 400)
    if not email or '@' not in email:
        return httpmod.error('请输入有效的邮箱', 400)
    if not password or len(password) < 6:
        return httpmod.error('密码至少6个字符', 400)
    if len(password) > 128:
        return httpmod.error('密码过长', 400)

    uid = await create_user(username, email, password)
    if not uid:
        return httpmod.error('用户名或邮箱已被使用', 409)

    token = authmod.generate_token(uid)
    user = await get_user_by_id(uid)
    return httpmod.jsonify({
        'message': '注册成功',
        'token': token,
        'user': _user_dict(user),
    }, status=201)


async def login(request, params):
    data = await httpmod.get_json_body(request) or {}
    username = (data.get('username') or '').strip()
    password = data.get('password') or ''

    if not username or not password:
        return httpmod.error('请提供用户名和密码', 400)

    user = await get_user_by_username(username) or await get_user_by_email(username)
    if not user or not authmod.verify_password(password, user['password_hash']):
        return httpmod.error('用户名或密码错误', 401)

    token = authmod.generate_token(user['id'])
    return httpmod.jsonify({
        'message': '登录成功',
        'token': token,
        'user': _user_dict(user),
    })


async def get_me(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    stats = await get_user_stats(user['id'])
    return httpmod.jsonify({**_user_dict(user), 'stats': stats})


async def update_me(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    data = await httpmod.get_json_body(request) or {}
    uid = user['id']

    if 'username' in data:
        existing = await get_user_by_username(data['username'])
        if existing and existing['id'] != uid:
            return httpmod.error('用户名已被使用', 409)
    if 'email' in data:
        existing = await get_user_by_email(data['email'])
        if existing and existing['id'] != uid:
            return httpmod.error('邮箱已被使用', 409)

    fields = {k: data[k] for k in ('username', 'email', 'avatar', 'bio', 'mood', 'title') if k in data}
    await update_user(uid, **fields)
    user = await get_user_by_id(uid)
    return httpmod.jsonify({'message': '更新成功', 'user': _user_dict(user)})


async def get_user_profile(request, params):
    uid = int(params['uid'])
    user = await get_user_by_id(uid)
    if not user:
        return httpmod.error('用户不存在', 404)
    stats = await get_user_stats(uid)
    return httpmod.jsonify({**_user_dict(user), 'stats': stats})


async def upload_avatar(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    url, err = await save_upload(request, prefix='avatar', subdir='avatars')
    if err:
        return httpmod.error(err, 400)
    await update_user(user['id'], avatar=url)
    return httpmod.jsonify({'url': url, 'message': '头像更新成功'})


async def change_pwd(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    data = await httpmod.get_json_body(request) or {}
    old_pw = data.get('old_password', '')
    new_pw = data.get('new_password', '')

    if not old_pw or not new_pw:
        return httpmod.error('请填写完整', 400)
    if len(new_pw) < 6:
        return httpmod.error('新密码至少6个字符', 400)
    if len(new_pw) > 128:
        return httpmod.error('密码过长', 400)

    full = await get_user_by_id(user['id'])
    if not authmod.verify_password(old_pw, full['password_hash']):
        return httpmod.error('原密码错误', 400)

    await change_password(user['id'], new_pw)
    return httpmod.jsonify({'message': '密码修改成功'})


async def forgot_password(request, params):
    data = await httpmod.get_json_body(request) or {}
    email = (data.get('email') or '').strip()
    if not email or '@' not in email:
        return httpmod.error('请输入有效的邮箱', 400)

    user = await get_user_by_email(email)
    # 不泄露邮箱是否存在
    if not user:
        return httpmod.jsonify({'message': '如果该邮箱已注册，重置链接已发送'})

    reset_token = authmod.generate_reset_token(user['id'])
    smtp_host = httpmod.get_env_secret('SMTP_HOST')
    if smtp_host:
        try:
            await _send_reset_email(smtp_host, email, reset_token)
        except Exception:
            return httpmod.error('邮件发送失败，请稍后再试', 500)
        return httpmod.jsonify({'message': '如果该邮箱已注册，重置链接已发送'})
    # 开发模式：直接返回 token
    return httpmod.jsonify({'message': '（开发模式）重置token已生成', 'reset_token': reset_token})


async def _send_reset_email(smtp_host, email, reset_token):
    import smtplib
    from email.mime.text import MIMEText
    base_url = httpmod.get_env_secret('SITE_BASE_URL', 'http://localhost:5000')
    msg = MIMEText(
        f'你的重置链接（30分钟内有效）：\n{base_url}/reset-password?token={reset_token}\n\n'
        f'如果你没有请求重置密码，请忽略此邮件。', 'plain', 'utf-8')
    msg['Subject'] = '校园墙 · 密码重置'
    msg['From'] = httpmod.get_env_secret('SMTP_USER', '')
    msg['To'] = email
    with smtplib.SMTP(smtp_host, int(httpmod.get_env_secret('SMTP_PORT', '587'))) as server:
        if httpmod.get_env_secret('SMTP_TLS', '1') == '1':
            server.starttls()
        server.login(httpmod.get_env_secret('SMTP_USER', ''),
                     httpmod.get_env_secret('SMTP_PASSWORD', ''))
        server.send_message(msg)


async def reset_password(request, params):
    data = await httpmod.get_json_body(request) or {}
    token = (data.get('token') or '').strip()
    new_password = data.get('new_password', '')

    if not token:
        return httpmod.error('缺少重置令牌', 400)
    if not new_password or len(new_password) < 6:
        return httpmod.error('新密码至少6个字符', 400)
    if len(new_password) > 128:
        return httpmod.error('密码过长', 400)

    payload = authmod.decode_reset_token(token)
    if not payload:
        return httpmod.error('无效或过期的重置令牌', 400)
    await change_password(payload['user_id'], new_password)
    return httpmod.jsonify({'message': '密码已重置，请使用新密码登录'})


ROUTES = [
    (HTTPMethod.POST, r'^/api/auth/register$', register),
    (HTTPMethod.POST, r'^/api/auth/login$', login),
    (HTTPMethod.GET, r'^/api/auth/me$', get_me),
    (HTTPMethod.PUT, r'^/api/auth/me$', update_me),
    (HTTPMethod.GET, r'^/api/auth/user/(?P<uid>\d+)$', get_user_profile),
    (HTTPMethod.POST, r'^/api/auth/avatar$', upload_avatar),
    (HTTPMethod.POST, r'^/api/auth/change-password$', change_pwd),
    (HTTPMethod.POST, r'^/api/auth/forgot-password$', forgot_password),
    (HTTPMethod.POST, r'^/api/auth/reset-password$', reset_password),
]
