import os
import uuid
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


def _user_dict(user):
    return {
        'id': user['id'],
        'username': user['username'],
        'email': user.get('email', ''),
        'avatar': user['avatar'],
        'bio': user['bio'],
        'role': user['role'],
        'created_at': user.get('created_at', '')
    }
