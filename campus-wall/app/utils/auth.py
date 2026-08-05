"""JWT 认证工具"""
import jwt
from datetime import datetime, timedelta
from functools import wraps
from flask import request, jsonify, current_app, g


def generate_token(user_id):
    payload = {
        'user_id': user_id,
        'exp': datetime.utcnow() + timedelta(days=current_app.config['JWT_EXPIRATION_DAYS'])
    }
    return jwt.encode(payload, current_app.config['JWT_SECRET'], algorithm='HS256')


def decode_token(token):
    try:
        return jwt.decode(token, current_app.config['JWT_SECRET'], algorithms=['HS256'])
    except jwt.ExpiredSignatureError:
        return None
    except jwt.InvalidTokenError:
        return None


def token_required(f):
    @wraps(f)
    def decorated(*args, **kwargs):
        token = request.headers.get('Authorization', '').replace('Bearer ', '')
        if not token:
            return jsonify({'error': '缺少认证令牌'}), 401
        data = decode_token(token)
        if not data:
            return jsonify({'error': '令牌无效或已过期'}), 401
        from app.models import get_user_by_id
        user = get_user_by_id(data['user_id'])
        if not user:
            return jsonify({'error': '用户不存在'}), 401
        g.current_user = user
        return f(*args, **kwargs)
    return decorated


def optional_auth(f):
    """可选认证：有 token 则解析，没有则 g.current_user = None"""
    @wraps(f)
    def decorated(*args, **kwargs):
        g.current_user = None
        token = request.headers.get('Authorization', '').replace('Bearer ', '')
        if token:
            data = decode_token(token)
            if data:
                from app.models import get_user_by_id
                g.current_user = get_user_by_id(data['user_id'])
        return f(*args, **kwargs)
    return decorated
