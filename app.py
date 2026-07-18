from flask import Flask, render_template, jsonify, request, session
from flask_cors import CORS
import sqlite3
from datetime import datetime, timedelta
import json
import jwt
from functools import wraps
from database import init_db, get_user_by_id, get_user_by_username, get_user_by_email, verify_password, create_user

app = Flask(__name__)
app.secret_key = 'your-secret-key-change-in-production'  # 生产环境请更换
CORS(app, supports_credentials=True)

# JWT 配置
JWT_SECRET = 'your-jwt-secret-key-change-in-production'
JWT_EXPIRATION = 7  # 7天

# ===== 应用启动时初始化数据库 =====
with app.app_context():
    init_db()

def token_required(f):
    """JWT Token 验证装饰器"""
    @wraps(f)
    def decorated(*args, **kwargs):
        token = request.headers.get('Authorization')
        if not token:
            return jsonify({'error': '缺少认证令牌'}), 401
        
        try:
            # 移除 'Bearer ' 前缀
            if token.startswith('Bearer '):
                token = token[7:]
            data = jwt.decode(token, JWT_SECRET, algorithms=['HS256'])
            current_user = get_user_by_id(data['user_id'])
            if not current_user:
                return jsonify({'error': '用户不存在'}), 401
        except jwt.ExpiredSignatureError:
            return jsonify({'error': '令牌已过期'}), 401
        except jwt.InvalidTokenError:
            return jsonify({'error': '无效的令牌'}), 401
        
        return f(current_user, *args, **kwargs)
    return decorated

def generate_token(user_id):
    """生成 JWT Token"""
    payload = {
        'user_id': user_id,
        'exp': datetime.utcnow() + timedelta(days=JWT_EXPIRATION)
    }
    return jwt.encode(payload, JWT_SECRET, algorithm='HS256')

# ===== 认证路由 =====
@app.route('/api/auth/register', methods=['POST'])
def register():
    """用户注册"""
    data = request.get_json()
    username = data.get('username', '').strip()
    email = data.get('email', '').strip()
    password = data.get('password', '')
    
    # 验证输入
    if not username or len(username) < 3:
        return jsonify({'error': '用户名至少3个字符'}), 400
    if not email or '@' not in email:
        return jsonify({'error': '请输入有效的邮箱'}), 400
    if not password or len(password) < 6:
        return jsonify({'error': '密码至少6个字符'}), 400
    
    # 创建用户
    user_id = create_user(username, email, password)
    if not user_id:
        return jsonify({'error': '用户名或邮箱已被使用'}), 409
    
    # 生成Token
    token = generate_token(user_id)
    user = get_user_by_id(user_id)
    
    return jsonify({
        'message': '注册成功',
        'token': token,
        'user': {
            'id': user['id'],
            'username': user['username'],
            'email': user['email'],
            'avatar': user['avatar'],
            'bio': user['bio'],
            'role': user['role']
        }
    }), 201

@app.route('/api/auth/login', methods=['POST'])
def login():
    """用户登录"""
    data = request.get_json()
    username = data.get('username', '').strip()
    password = data.get('password', '')
    
    if not username or not password:
        return jsonify({'error': '请提供用户名和密码'}), 400
    
    # 查找用户（支持用户名或邮箱登录）
    user = get_user_by_username(username)
    if not user:
        user = get_user_by_email(username)
    
    if not user:
        return jsonify({'error': '用户名或密码错误'}), 401
    
    # 验证密码
    if not verify_password(password, user['password_hash']):
        return jsonify({'error': '用户名或密码错误'}), 401
    
    # 生成Token
    token = generate_token(user['id'])
    
    return jsonify({
        'message': '登录成功',
        'token': token,
        'user': {
            'id': user['id'],
            'username': user['username'],
            'email': user['email'],
            'avatar': user['avatar'],
            'bio': user['bio'],
            'role': user['role']
        }
    })

@app.route('/api/auth/me', methods=['GET'])
@token_required
def get_current_user(current_user):
    """获取当前用户信息"""
    return jsonify({
        'id': current_user['id'],
        'username': current_user['username'],
        'email': current_user['email'],
        'avatar': current_user['avatar'],
        'bio': current_user['bio'],
        'role': current_user['role'],
        'created_at': current_user['created_at']
    })

@app.route('/api/auth/logout', methods=['POST'])
def logout():
    """登出（客户端删除token即可）"""
    return jsonify({'message': '已登出'})

# ===== 原有路由 =====
@app.route('/')
def index():
    return render_template('index.html')

@app.route('/api/stations')
def get_stations():
    conn = sqlite3.connect('instance/campushub.db')
    c = conn.cursor()
    c.execute('SELECT id, name, description, cover, tags, user_count, post_count, created_at FROM sub_stations ORDER BY created_at DESC')
    stations = []
    for row in c.fetchall():
        stations.append({
            'id': row[0],
            'name': row[1],
            'description': row[2],
            'cover': row[3] or '/static/images/default-cover.jpg',
            'tags': json.loads(row[4]) if row[4] else [],
            'user_count': row[5],
            'post_count': row[6],
            'created_at': row[7]
        })
    conn.close()
    return jsonify(stations)

@app.route('/api/posts')
def get_posts():
    limit = request.args.get('limit', 6, type=int)
    conn = sqlite3.connect('instance/campushub.db')
    c = conn.cursor()
    c.execute('''SELECT p.id, p.title, p.content, p.author, p.views, p.likes, p.comments, p.created_at, s.name 
                 FROM posts p JOIN sub_stations s ON p.station_id = s.id 
                 ORDER BY p.created_at DESC LIMIT ?''', (limit,))
    posts = []
    for row in c.fetchall():
        posts.append({
            'id': row[0],
            'title': row[1],
            'content': row[2][:100] + ('...' if len(row[2]) > 100 else ''),
            'author': row[3],
            'views': row[4],
            'likes': row[5],
            'comments': row[6],
            'created_at': row[7],
            'station_name': row[8]
        })
    conn.close()
    return jsonify(posts)

@app.route('/api/search')
def search_stations():
    keyword = request.args.get('q', '').strip()
    if not keyword:
        return jsonify([])
    
    conn = sqlite3.connect('instance/campushub.db')
    c = conn.cursor()
    c.execute('''SELECT id, name, description, cover, tags, user_count, post_count 
                 FROM sub_stations 
                 WHERE name LIKE ? OR description LIKE ? 
                 ORDER BY user_count DESC''', 
              (f'%{keyword}%', f'%{keyword}%'))
    stations = []
    for row in c.fetchall():
        stations.append({
            'id': row[0],
            'name': row[1],
            'description': row[2],
            'cover': row[3] or '/static/images/default-cover.jpg',
            'tags': json.loads(row[4]) if row[4] else [],
            'user_count': row[5],
            'post_count': row[6]
        })
    conn.close()
    return jsonify(stations)

if __name__ == '__main__':
    app.run(debug=True, host='0.0.0.0', port=5000)
