import sqlite3
import bcrypt
from datetime import datetime
import json

DB_PATH = 'instance/campushub.db'

def get_db_connection():
    conn = sqlite3.connect(DB_PATH)
    conn.row_factory = sqlite3.Row
    return conn

def init_db():
    conn = get_db_connection()
    c = conn.cursor()
    
    # 创建用户表
    c.execute('''CREATE TABLE IF NOT EXISTS users (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        username TEXT UNIQUE NOT NULL,
        email TEXT UNIQUE NOT NULL,
        password_hash TEXT NOT NULL,
        avatar TEXT DEFAULT '/static/images/default-avatar.png',
        bio TEXT DEFAULT '',
        role TEXT DEFAULT 'user',
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )''')
    
    # 为子站表添加 owner_id 字段（如果不存在）
    try:
        c.execute('ALTER TABLE sub_stations ADD COLUMN owner_id INTEGER REFERENCES users(id)')
    except sqlite3.OperationalError:
        pass  # 字段已存在
    
    # 为帖子表添加 author_id 字段（如果不存在）
    try:
        c.execute('ALTER TABLE posts ADD COLUMN author_id INTEGER REFERENCES users(id)')
    except sqlite3.OperationalError:
        pass  # 字段已存在
    
    # 创建关注表
    c.execute('''CREATE TABLE IF NOT EXISTS follows (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        follower_id INTEGER NOT NULL,
        following_id INTEGER NOT NULL,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        FOREIGN KEY (follower_id) REFERENCES users(id),
        FOREIGN KEY (following_id) REFERENCES users(id),
        UNIQUE(follower_id, following_id)
    )''')
    
    # 创建通知表
    c.execute('''CREATE TABLE IF NOT EXISTS notifications (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        user_id INTEGER NOT NULL,
        type TEXT NOT NULL,
        content TEXT NOT NULL,
        is_read BOOLEAN DEFAULT 0,
        link TEXT,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        FOREIGN KEY (user_id) REFERENCES users(id)
    )''')
    
    conn.commit()
    conn.close()

def create_user(username, email, password):
    """创建新用户"""
    conn = get_db_connection()
    c = conn.cursor()
    
    # 检查用户名或邮箱是否已存在
    c.execute('SELECT id FROM users WHERE username = ? OR email = ?', (username, email))
    if c.fetchone():
        conn.close()
        return None
    
    # 加密密码
    salt = bcrypt.gensalt()
    password_hash = bcrypt.hashpw(password.encode('utf-8'), salt)
    
    c.execute('''INSERT INTO users (username, email, password_hash) 
                 VALUES (?, ?, ?)''', (username, email, password_hash.decode('utf-8')))
    conn.commit()
    
    user_id = c.lastrowid
    conn.close()
    return user_id

def get_user_by_username(username):
    """根据用户名获取用户"""
    conn = get_db_connection()
    c = conn.cursor()
    c.execute('SELECT * FROM users WHERE username = ?', (username,))
    user = c.fetchone()
    conn.close()
    return dict(user) if user else None

def get_user_by_email(email):
    """根据邮箱获取用户"""
    conn = get_db_connection()
    c = conn.cursor()
    c.execute('SELECT * FROM users WHERE email = ?', (email,))
    user = c.fetchone()
    conn.close()
    return dict(user) if user else None

def get_user_by_id(user_id):
    """根据ID获取用户"""
    conn = get_db_connection()
    c = conn.cursor()
    c.execute('SELECT * FROM users WHERE id = ?', (user_id,))
    user = c.fetchone()
    conn.close()
    return dict(user) if user else None

def verify_password(password, password_hash):
    """验证密码"""
    return bcrypt.checkpw(password.encode('utf-8'), password_hash.encode('utf-8'))

def update_user_profile(user_id, data):
    """更新用户资料"""
    conn = get_db_connection()
    c = conn.cursor()
    
    fields = []
    values = []
    for key, value in data.items():
        if key in ['bio', 'avatar']:
            fields.append(f'{key} = ?')
            values.append(value)
    
    if not fields:
        conn.close()
        return False
    
    values.append(user_id)
    c.execute(f'''UPDATE users SET {', '.join(fields)}, updated_at = CURRENT_TIMESTAMP 
                 WHERE id = ?''', values)
    conn.commit()
    conn.close()
    return True

if __name__ == '__main__':
    init_db()
    print('数据库初始化完成！')
