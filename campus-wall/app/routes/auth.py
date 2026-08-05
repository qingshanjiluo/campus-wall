import os
import uuid
from datetime import datetime, timedelta
import jwt as pyjwt
from flask import Blueprint, request, jsonify, current_app, g
from app.models import create_user, get_user_by_username, get_user_by_email, verify_password, update_user, get_user_by_id, get_user_stats, change_password
from app.utils.auth import generate_token, token_required

auth_bp = Blueprint('auth', __name__)


@auth_bp.route('/register', methods=['POST'])
def register():
    data = request.get_json(silent=True) or {}
    username = (data.get('username') or '').strip()
    email = (data.get('email') or '').strip()
    password = data.get('password') or ''

    if not username or len(username) < 2:
        return jsonify({'error': '用户名至少2个字符'}), 400
    if len(username) > 30:
        return jsonify({'error': '用户名最多30个字符'}), 400
    if not email or '@' not in email:
        return jsonify({'error': '请输入有效的邮箱'}), 400
    if not password or len(password) < 6:
        return jsonify({'error': '密码至少6个字符'}), 400
    if len(password) > 128:
        return jsonify({'error': '密码过长'}), 400

    uid = create_user(username, email, password)
    if not uid:
        return jsonify({'error': '用户名或邮箱已被使用'}), 409

    token = generate_token(uid)
    user = get_user_by_id(uid)
    return jsonify({
        'message': '注册成功',
        'token': token,
        'user': _user_dict(user)
    }), 201


@auth_bp.route('/login', methods=['POST'])
def login():
    data = request.get_json(silent=True) or {}
    username = (data.get('username') or '').strip()
    password = data.get('password') or ''

    if not username or not password:
        return jsonify({'error': '请提供用户名和密码'}), 400

    user = get_user_by_username(username) or get_user_by_email(username)
    if not user or not verify_password(password, user['password_hash']):
        return jsonify({'error': '用户名或密码错误'}), 401

    token = generate_token(user['id'])
    return jsonify({
        'message': '登录成功',
        'token': token,
        'user': _user_dict(user)
    })


@auth_bp.route('/me', methods=['GET'])
@token_required
def get_me():
    user = g.current_user
    stats = get_user_stats(user['id'])
    return jsonify({**_user_dict(user), 'stats': stats})


@auth_bp.route('/me', methods=['PUT'])
@token_required
def update_me():
    data = request.get_json(silent=True) or {}
    uid = g.current_user['id']

    if 'username' in data:
        existing = get_user_by_username(data['username'])
        if existing and existing['id'] != uid:
            return jsonify({'error': '用户名已被使用'}), 409
    if 'email' in data:
        existing = get_user_by_email(data['email'])
        if existing and existing['id'] != uid:
            return jsonify({'error': '邮箱已被使用'}), 409

    update_user(uid, **{k: data[k] for k in ('username', 'email', 'avatar', 'bio') if k in data})
    user = get_user_by_id(uid)
    return jsonify({'message': '更新成功', 'user': _user_dict(user)})


@auth_bp.route('/user/<int:uid>', methods=['GET'])
def get_user_profile(uid):
    user = get_user_by_id(uid)
    if not user:
        return jsonify({'error': '用户不存在'}), 404
    stats = get_user_stats(uid)
    return jsonify({**_user_dict(user), 'stats': stats})


@auth_bp.route('/avatar', methods=['POST'])
@token_required
def upload_avatar():
    if 'file' not in request.files:
        return jsonify({'error': '请选择文件'}), 400
    file = request.files['file']
    if not file.filename:
        return jsonify({'error': '请选择文件'}), 400

    ext = file.filename.rsplit('.', 1)[-1].lower() if '.' in file.filename else ''
    if ext not in ('png', 'jpg', 'jpeg', 'gif', 'webp'):
        return jsonify({'error': '不支持的格式，请上传 png/jpg/gif/webp'}), 400

    filename = f"avatar_{g.current_user['id']}_{uuid.uuid4().hex[:8]}.{ext}"
    filepath = os.path.join(current_app.config['UPLOAD_FOLDER'], filename)
    file.save(filepath)

    avatar_url = f'/static/uploads/{filename}'
    update_user(g.current_user['id'], avatar=avatar_url)
    return jsonify({'url': avatar_url, 'message': '头像更新成功'})


@auth_bp.route('/change-password', methods=['POST'])
@token_required
def change_pwd():
    data = request.get_json(silent=True) or {}
    old_pw = data.get('old_password', '')
    new_pw = data.get('new_password', '')

    if not old_pw or not new_pw:
        return jsonify({'error': '请填写完整'}), 400
    if len(new_pw) < 6:
        return jsonify({'error': '新密码至少6个字符'}), 400
    if len(new_pw) > 128:
        return jsonify({'error': '密码过长'}), 400

    user = get_user_by_id(g.current_user['id'])
    if not verify_password(old_pw, user['password_hash']):
        return jsonify({'error': '原密码错误'}), 400

    change_password(g.current_user['id'], new_pw)
    return jsonify({'message': '密码修改成功'})


# ── 忘记密码 / 重置密码 ──

@auth_bp.route('/forgot-password', methods=['POST'])
def forgot_password():
    """请求重置密码：输入邮箱，返回重置token（开发模式）或发送邮件"""
    data = request.get_json(silent=True) or {}
    email = (data.get('email') or '').strip()
    if not email or '@' not in email:
        return jsonify({'error': '请输入有效的邮箱'}), 400

    user = get_user_by_email(email)
    # 不泄露邮箱是否存在
    if not user:
        return jsonify({'message': '如果该邮箱已注册，重置链接已发送'})

    reset_token = pyjwt.encode({
        'user_id': user['id'],
        'purpose': 'password_reset',
        'exp': datetime.utcnow() + timedelta(minutes=30)
    }, current_app.config['JWT_SECRET'], algorithm='HS256')

    # 邮件发送：可配置 SMTP；开发模式返回 token 便于联调
    smtp_host = os.environ.get('SMTP_HOST', '')
    if smtp_host:
        try:
            import smtplib
            from email.mime.text import MIMEText
            msg = MIMEText(f'你的重置链接（30分钟内有效）：\n{cwd_base_url()}/reset-password?token={reset_token}\n\n如果你没有请求重置密码，请忽略此邮件。', 'plain', 'utf-8')
            msg['Subject'] = '校园墙 · 密码重置'
            msg['From'] = os.environ.get('SMTP_USER', '')
            msg['To'] = email
            with smtplib.SMTP(smtp_host, int(os.environ.get('SMTP_PORT', 587))) as server:
                if os.environ.get('SMTP_TLS', '1') == '1':
                    server.starttls()
                server.login(os.environ.get('SMTP_USER', ''), os.environ.get('SMTP_PASSWORD', ''))
                server.send_message(msg)
        except Exception as e:
            current_app.logger.warning(f'邮件发送失败: {e}')
            return jsonify({'message': '邮件发送失败，请稍后再试'}), 500
    else:
        # 开发模式：直接返回 token
        current_app.logger.info(f'重置密码 token: {reset_token}')
        return jsonify({'message': '（开发模式）重置token已生成', 'reset_token': reset_token})

    return jsonify({'message': '如果该邮箱已注册，重置链接已发送'})


def cwd_base_url():
    return os.environ.get('SITE_BASE_URL', 'http://localhost:5000')


@auth_bp.route('/reset-password', methods=['POST'])
def reset_password():
    """使用重置token设置新密码"""
    data = request.get_json(silent=True) or {}
    token = (data.get('token') or '').strip()
    new_password = data.get('new_password', '')

    if not token:
        return jsonify({'error': '缺少重置令牌'}), 400
    if not new_password or len(new_password) < 6:
        return jsonify({'error': '新密码至少6个字符'}), 400
    if len(new_password) > 128:
        return jsonify({'error': '密码过长'}), 400

    try:
        payload = pyjwt.decode(token, current_app.config['JWT_SECRET'], algorithms=['HS256'])
        if payload.get('purpose') != 'password_reset':
            return jsonify({'error': '无效的重置令牌'}), 400
        user_id = payload['user_id']
    except pyjwt.ExpiredSignatureError:
        return jsonify({'error': '重置令牌已过期'}), 400
    except pyjwt.InvalidTokenError:
        return jsonify({'error': '无效的重置令牌'}), 400

    change_password(user_id, new_password)
    return jsonify({'message': '密码已重置，请使用新密码登录'})


def _user_dict(user):
    return {
        'id': user['id'],
        'username': user['username'],
        'email': user.get('email', ''),
        'avatar': user['avatar'],
        'bio': user['bio'],
        'role': user['role'],
        'created_at': user.get('created_at', ''),
        # 扩展字段（虚拟资产 / 身份组）
        'coins': user.get('coins', 0),
        'points': user.get('points', 0),
        'level': user.get('level', 1),
        'exp': user.get('exp', 0),
        'checkin_streak': user.get('checkin_streak', 0),
        'identity_group': user.get('identity_group', ''),
        'title': user.get('title', ''),
        'mood': user.get('mood', '')
    }
