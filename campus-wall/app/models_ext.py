"""
扩展数据模型 — 身份组、积分、签到、虚拟币、恋爱情报、交易、看板娘、管理后台
"""
import os
import json
import sqlite3
import random
from datetime import datetime, timedelta
from app.models import get_db, query_db, execute_db


def init_extended_db():
    """创建扩展表（在 init_db 之后调用）"""
    conn = get_db()
    c = conn.cursor()

    # ── 用户扩展字段 ──
    for col, coltype, default in [
        ('coins', 'INTEGER', '0'), ('points', 'INTEGER', '0'), ('level', 'INTEGER', '1'),
        ('exp', 'INTEGER', '0'), ('checkin_streak', 'INTEGER', '0'), ('last_checkin', "TEXT", "''"),
        ('identity_group', "TEXT", "''"), ('title', "TEXT", "''"), ('mood', "TEXT", "''")
    ]:
        try:
            c.execute(f"ALTER TABLE users ADD COLUMN {col} {coltype} DEFAULT {default}")
        except sqlite3.OperationalError:
            pass

    # ── 帖子扩展字段 ──
    for col, default in [('post_type', "'text'"), ('images', "'[]'"), ('extra', "'{}'")]:
        try:
            c.execute(f"ALTER TABLE posts ADD COLUMN {col} TEXT DEFAULT {default}")
        except sqlite3.OperationalError:
            pass

    # ── 身份组 ──
    c.execute('''CREATE TABLE IF NOT EXISTS identity_groups (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        name TEXT NOT NULL UNIQUE,
        icon TEXT DEFAULT '🏷️',
        color TEXT DEFAULT '#fb6f92',
        description TEXT DEFAULT '',
        permissions TEXT DEFAULT '[]',
        min_level INTEGER DEFAULT 1,
        is_default INTEGER DEFAULT 0,
        sort_order INTEGER DEFAULT 0,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )''')

    # ── 签到 ──
    c.execute('''CREATE TABLE IF NOT EXISTS checkins (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        user_id INTEGER REFERENCES users(id),
        checkin_date TEXT NOT NULL,
        streak INTEGER DEFAULT 1,
        coins_earned INTEGER DEFAULT 0,
        points_earned INTEGER DEFAULT 0,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        UNIQUE(user_id, checkin_date)
    )''')

    # ── 虚拟币流水 ──
    c.execute('''CREATE TABLE IF NOT EXISTS coin_transactions (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        user_id INTEGER REFERENCES users(id),
        amount INTEGER NOT NULL,
        balance_after INTEGER DEFAULT 0,
        type TEXT NOT NULL,
        description TEXT DEFAULT '',
        ref_type TEXT DEFAULT '',
        ref_id INTEGER DEFAULT 0,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )''')

    # ── 积分商城商品 ──
    c.execute('''CREATE TABLE IF NOT EXISTS shop_items (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        name TEXT NOT NULL,
        description TEXT DEFAULT '',
        icon TEXT DEFAULT '🎁',
        price_coins INTEGER DEFAULT 0,
        price_points INTEGER DEFAULT 0,
        item_type TEXT DEFAULT 'badge',
        item_data TEXT DEFAULT '{}',
        stock INTEGER DEFAULT -1,
        is_active INTEGER DEFAULT 1,
        sort_order INTEGER DEFAULT 0,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )''')

    # ── 商城订单 ──
    c.execute('''CREATE TABLE IF NOT EXISTS shop_orders (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        user_id INTEGER REFERENCES users(id),
        item_id INTEGER REFERENCES shop_items(id),
        quantity INTEGER DEFAULT 1,
        total_coins INTEGER DEFAULT 0,
        total_points INTEGER DEFAULT 0,
        status TEXT DEFAULT 'completed',
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )''')

    # ── 恋爱情报专区 ──
    c.execute('''CREATE TABLE IF NOT EXISTS romance_profiles (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        user_id INTEGER UNIQUE REFERENCES users(id),
        nickname TEXT DEFAULT '',
        gender TEXT DEFAULT '',
        age INTEGER DEFAULT 0,
        department TEXT DEFAULT '',
        hobbies TEXT DEFAULT '[]',
        looking_for TEXT DEFAULT '',
        photo TEXT DEFAULT '',
        is_visible INTEGER DEFAULT 1,
        likes_received INTEGER DEFAULT 0,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )''')

    c.execute('''CREATE TABLE IF NOT EXISTS romance_links (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        from_user_id INTEGER REFERENCES users(id),
        to_user_id INTEGER REFERENCES users(id),
        link_type TEXT DEFAULT 'crush',
        description TEXT DEFAULT '',
        is_anonymous INTEGER DEFAULT 0,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )''')

    c.execute('''CREATE TABLE IF NOT EXISTS romance_tasks (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        creator_id INTEGER REFERENCES users(id),
        title TEXT NOT NULL,
        description TEXT DEFAULT '',
        task_type TEXT DEFAULT 'matchmake',
        target_user_id INTEGER DEFAULT 0,
        reward_coins INTEGER DEFAULT 0,
        status TEXT DEFAULT 'open',
        assigned_to INTEGER DEFAULT 0,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )''')

    # ── 爆料/树洞 ──
    c.execute('''CREATE TABLE IF NOT EXISTS gossip (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        content TEXT NOT NULL,
        images TEXT DEFAULT '[]',
        station_id INTEGER REFERENCES stations(id),
        likes_count INTEGER DEFAULT 0,
        comments_count INTEGER DEFAULT 0,
        is_anonymous INTEGER DEFAULT 1,
        is_hot INTEGER DEFAULT 0,
        is_deleted INTEGER DEFAULT 0,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )''')

    c.execute('''CREATE TABLE IF NOT EXISTS gossip_comments (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        gossip_id INTEGER REFERENCES gossip(id),
        content TEXT NOT NULL,
        author_name TEXT DEFAULT '匿名',
        likes_count INTEGER DEFAULT 0,
        is_deleted INTEGER DEFAULT 0,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )''')

    # ── 看板娘消息 ──
    c.execute('''CREATE TABLE IF NOT EXISTS kanban_messages (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        message TEXT NOT NULL,
        message_type TEXT DEFAULT 'greeting',
        trigger_type TEXT DEFAULT 'time',
        trigger_data TEXT DEFAULT '{}',
        is_active INTEGER DEFAULT 1,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )''')

    # ── 管理日志 ──
    c.execute('''CREATE TABLE IF NOT EXISTS admin_log (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        admin_id INTEGER REFERENCES users(id),
        action TEXT NOT NULL,
        target_type TEXT DEFAULT '',
        target_id INTEGER DEFAULT 0,
        detail TEXT DEFAULT '',
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )''')

    # ── 交易帖 ──
    c.execute('''CREATE TABLE IF NOT EXISTS trade_posts (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        post_id INTEGER REFERENCES posts(id),
        user_id INTEGER REFERENCES users(id),
        price REAL DEFAULT 0,
        original_price REAL DEFAULT 0,
        condition TEXT DEFAULT 'good',
        category TEXT DEFAULT '',
        status TEXT DEFAULT 'available',
        contact TEXT DEFAULT '',
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )''')

    # ── 收藏 ──
    c.execute('''CREATE TABLE IF NOT EXISTS favorites (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        user_id INTEGER NOT NULL,
        target_type TEXT NOT NULL,
        target_id INTEGER NOT NULL,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        UNIQUE(user_id, target_type, target_id)
    )''')

    # ── 举报 ──
    c.execute('''CREATE TABLE IF NOT EXISTS reports (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        reporter_id INTEGER NOT NULL,
        target_type TEXT NOT NULL,
        target_id INTEGER NOT NULL,
        reason TEXT DEFAULT '',
        detail TEXT DEFAULT '',
        status TEXT DEFAULT 'pending',
        handler_id INTEGER,
        handled_at TIMESTAMP,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )''')

    # ── 通知偏好 ──
    c.execute('''CREATE TABLE IF NOT EXISTS user_settings (
        user_id INTEGER PRIMARY KEY,
        notify_comment INTEGER DEFAULT 1,
        notify_like INTEGER DEFAULT 1,
        notify_follow INTEGER DEFAULT 1,
        notify_system INTEGER DEFAULT 1,
        theme TEXT DEFAULT 'auto',
        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )''')

    conn.commit()
    conn.close()


# ══════════════════════════════════════════════
# 身份组
# ══════════════════════════════════════════════

def get_identity_groups():
    return query_db('SELECT * FROM identity_groups ORDER BY sort_order ASC')

def get_identity_group_by_id(gid):
    return query_db('SELECT * FROM identity_groups WHERE id = ?', (gid,), one=True)

def create_identity_group(name, icon, color, description, permissions, min_level, is_default):
    return execute_db(
        'INSERT INTO identity_groups (name, icon, color, description, permissions, min_level, is_default) VALUES (?,?,?,?,?,?,?)',
        (name, icon, color, description, json.dumps(permissions), min_level, is_default)
    )

def assign_user_group(user_id, group_name):
    from app.models import execute_db as exec_db
    execute_db('UPDATE users SET identity_group = ? WHERE id = ?', (group_name, user_id))

def get_user_group(user):
    group_name = user.get('identity_group', '')
    if not group_name:
        return None
    return query_db('SELECT * FROM identity_groups WHERE name = ?', (group_name,), one=True)


# ══════════════════════════════════════════════
# 签到
# ══════════════════════════════════════════════

def do_checkin(user_id):
    today = datetime.now().strftime('%Y-%m-%d')
    existing = query_db('SELECT * FROM checkins WHERE user_id = ? AND checkin_date = ?', (user_id, today), one=True)
    if existing:
        return None  # 已签到

    yesterday = (datetime.now() - timedelta(days=1)).strftime('%Y-%m-%d')
    last = query_db('SELECT * FROM checkins WHERE user_id = ? AND checkin_date = ?', (user_id, yesterday), one=True)
    streak = (last['streak'] + 1) if last else 1

    # 奖励计算：基础10币 + 连签加成
    base_coins = 10
    streak_bonus = min(streak - 1, 7) * 5  # 最多+35
    total_coins = base_coins + streak_bonus
    points_earned = streak * 2

    cid = execute_db(
        'INSERT INTO checkins (user_id, checkin_date, streak, coins_earned, points_earned) VALUES (?,?,?,?,?)',
        (user_id, today, streak, total_coins, points_earned)
    )

    # 更新用户
    user = query_db('SELECT coins, points, checkin_streak FROM users WHERE id = ?', (user_id,), one=True)
    new_coins = (user['coins'] or 0) + total_coins
    new_points = (user['points'] or 0) + points_earned
    execute_db('UPDATE users SET coins = ?, points = ?, checkin_streak = ?, last_checkin = ? WHERE id = ?',
               (new_coins, new_points, streak, today, user_id))

    # 记录流水
    add_coin_transaction(user_id, total_coins, 'checkin', f'每日签到 (连签{streak}天)', 'checkin', cid)

    return {
        'streak': streak, 'coins_earned': total_coins,
        'points_earned': points_earned, 'total_coins': new_coins, 'total_points': new_points
    }

def get_checkin_history(user_id, limit=30):
    return query_db(
        'SELECT * FROM checkins WHERE user_id = ? ORDER BY checkin_date DESC LIMIT ?',
        (user_id, limit)
    )

def get_checkin_today(user_id):
    today = datetime.now().strftime('%Y-%m-%d')
    return query_db('SELECT * FROM checkins WHERE user_id = ? AND checkin_date = ?', (user_id, today), one=True)


# ══════════════════════════════════════════════
# 虚拟币
# ══════════════════════════════════════════════

def add_coin_transaction(user_id, amount, tx_type, description='', ref_type='', ref_id=0):
    user = query_db('SELECT coins FROM users WHERE id = ?', (user_id,), one=True)
    new_balance = (user['coins'] or 0) + amount
    execute_db('UPDATE users SET coins = ? WHERE id = ?', (new_balance, user_id))
    execute_db(
        'INSERT INTO coin_transactions (user_id, amount, balance_after, type, description, ref_type, ref_id) VALUES (?,?,?,?,?,?,?)',
        (user_id, amount, new_balance, tx_type, description, ref_type, ref_id)
    )
    return new_balance

def get_coin_transactions(user_id, limit=50):
    return query_db(
        'SELECT * FROM coin_transactions WHERE user_id = ? ORDER BY created_at DESC LIMIT ?',
        (user_id, limit)
    )

def spend_coins(user_id, amount, description='', ref_type='', ref_id=0):
    user = query_db('SELECT coins FROM users WHERE id = ?', (user_id,), one=True)
    if (user['coins'] or 0) < amount:
        return False
    return add_coin_transaction(user_id, -amount, 'spend', description, ref_type, ref_id)


# ══════════════════════════════════════════════
# 积分商城
# ══════════════════════════════════════════════

def get_shop_items(active_only=True):
    sql = 'SELECT * FROM shop_items'
    if active_only:
        sql += ' WHERE is_active = 1'
    sql += ' ORDER BY sort_order ASC, id ASC'
    return query_db(sql)

def get_shop_item_by_id(item_id):
    return query_db('SELECT * FROM shop_items WHERE id = ?', (item_id,), one=True)

def buy_shop_item(user_id, item_id, quantity=1):
    item = get_shop_item_by_id(item_id)
    if not item or not item['is_active']:
        return None, '商品不存在或已下架'
    if item['stock'] == 0:
        return None, '库存不足'

    total_coins = item['price_coins'] * quantity
    total_points = item['price_points'] * quantity
    user = query_db('SELECT coins, points FROM users WHERE id = ?', (user_id,), one=True)

    if total_coins > 0 and (user['coins'] or 0) < total_coins:
        return None, '金币不足'
    if total_points > 0 and (user['points'] or 0) < total_points:
        return None, '积分不足'

    if total_coins > 0:
        spend_coins(user_id, total_coins, f'购买 {item["name"]}', 'shop', item_id)
    if total_points > 0:
        new_pts = (user['points'] or 0) - total_points
        execute_db('UPDATE users SET points = ? WHERE id = ?', (new_pts, user_id))

    if item['stock'] > 0:
        execute_db('UPDATE shop_items SET stock = stock - ? WHERE id = ?', (quantity, item_id))

    order_id = execute_db(
        'INSERT INTO shop_orders (user_id, item_id, quantity, total_coins, total_points) VALUES (?,?,?,?,?)',
        (user_id, item_id, quantity, total_coins, total_points)
    )
    return {'order_id': order_id, 'item': item}, None


def use_shop_item(user_id, item_id, extra=None):
    """使用已购道具（改名卡/置顶卡/匿名卡/称号/彩虹昵称）。消耗一个已购库存。"""
    extra = extra or {}
    order = query_db(
        '''SELECT o.*, i.item_type, i.name as item_name, i.item_data
           FROM shop_orders o JOIN shop_items i ON o.item_id = i.id
           WHERE o.user_id = ? AND o.item_id = ? AND o.status = 'completed'
           ORDER BY o.id ASC LIMIT 1''',
        (user_id, item_id), one=True)
    if not order:
        return None, '没有可用库存，请先购买该道具'
    item_type = order['item_type']

    if item_type in ('rename_card', 'rename'):
        new_name = (extra.get('new_name') or '').strip()
        if not new_name or len(new_name) < 2 or len(new_name) > 30:
            return None, '用户名需为2-30个字符'
        from app.models import get_user_by_username
        if get_user_by_username(new_name) and get_user_by_username(new_name)['id'] != user_id:
            return None, '该用户名已被使用'
        execute_db('UPDATE users SET username = ? WHERE id = ?', (new_name, user_id))
    elif item_type in ('pin_card', 'pin'):
        post_id = extra.get('post_id')
        if not post_id:
            return None, '缺少帖子ID'
        execute_db('UPDATE posts SET is_pinned = 1 WHERE id = ? AND author_id = ?',
                   (int(post_id), user_id))
    elif item_type in ('anonymity_card', 'anonymous'):
        # 标记用户可匿名发帖 3 次
        count = int(extra.get('count') or 3)
        from app.models import query_db as q, execute_db as e
        row = q('SELECT extra FROM posts WHERE id = 0', one=True)  # no-op keep
        execute_db('UPDATE users SET mood = ? WHERE id = ?',
                   (f'anonymity:{count}', user_id)) if False else None
        # 存到 users.title 特殊标记不可行，改用 coin_transactions 记录
        execute_db(
            'INSERT INTO coin_transactions (user_id, amount, balance_after, type, description, ref_type, ref_id) VALUES (?,?,?,?,?,?,?)',
            (user_id, 0, 0, 'card_used', '匿名卡已激活（可匿名发帖3次）', 'shop_item', item_id))
    elif item_type in ('title', 'rainbow_name'):
        new_title = (extra.get('title') or '').strip()
        if item_type == 'rainbow_name':
            new_title = '🌈 ' + (new_title or '彩虹用户')
        execute_db('UPDATE users SET title = ? WHERE id = ?', (new_title, user_id))
    else:
        return None, '该道具暂不支持使用'

    # 消耗一个库存订单
    execute_db('UPDATE shop_orders SET status = ? WHERE id = ?', ('used', order['id']))
    return {'message': f'已使用「{order["item_name"]}」'}, None


# ══════════════════════════════════════════════
# 恋爱情报专区
# ══════════════════════════════════════════════

def get_romance_profiles(limit=50, offset=0, gender=None):
    sql = 'SELECT rp.*, u.username, u.avatar FROM romance_profiles rp JOIN users u ON rp.user_id = u.id WHERE rp.is_visible = 1'
    args = []
    if gender:
        sql += ' AND rp.gender = ?'
        args.append(gender)
    sql += ' ORDER BY rp.likes_received DESC LIMIT ? OFFSET ?'
    args += [limit, offset]
    return query_db(sql, args)

def get_romance_profile(user_id):
    return query_db(
        'SELECT rp.*, u.username, u.avatar FROM romance_profiles rp JOIN users u ON rp.user_id = u.id WHERE rp.user_id = ?',
        (user_id,), one=True
    )

def create_romance_profile(user_id, **kwargs):
    existing = get_romance_profile(user_id)
    if existing:
        fields = {k: v for k, v in kwargs.items() if k in ('nickname','gender','age','department','hobbies','looking_for','photo','is_visible')}
        if not fields:
            return False
        sets = ', '.join(f'{k} = ?' for k in fields)
        vals = list(fields.values()) + [user_id]
        execute_db(f'UPDATE romance_profiles SET {sets} WHERE user_id = ?', vals)
        return True
    else:
        execute_db(
            'INSERT INTO romance_profiles (user_id, nickname, gender, age, department, hobbies, looking_for, photo) VALUES (?,?,?,?,?,?,?,?)',
            (user_id, kwargs.get('nickname',''), kwargs.get('gender',''),
             kwargs.get('age',0), kwargs.get('department',''),
             json.dumps(kwargs.get('hobbies',[]), ensure_ascii=False),
             kwargs.get('looking_for',''), kwargs.get('photo',''))
        )
        return True

def create_romance_link(from_uid, to_uid, link_type='crush', description='', is_anonymous=0):
    return execute_db(
        'INSERT INTO romance_links (from_user_id, to_user_id, link_type, description, is_anonymous) VALUES (?,?,?,?,?)',
        (from_uid, to_uid, link_type, description, is_anonymous)
    )

def get_romance_links(user_id, direction='to'):
    if direction == 'to':
        return query_db(
            '''SELECT rl.*, u.username as from_name, u.avatar as from_avatar
               FROM romance_links rl LEFT JOIN users u ON rl.from_user_id = u.id
               WHERE rl.to_user_id = ? ORDER BY rl.created_at DESC''', (user_id,))
    else:
        return query_db(
            '''SELECT rl.*, u.username as to_name, u.avatar as to_avatar
               FROM romance_links rl LEFT JOIN users u ON rl.to_user_id = u.id
               WHERE rl.from_user_id = ? ORDER BY rl.created_at DESC''', (user_id,))

def create_romance_task(creator_id, title, description='', task_type='matchmake', target_user_id=0, reward_coins=0):
    return execute_db(
        'INSERT INTO romance_tasks (creator_id, title, description, task_type, target_user_id, reward_coins) VALUES (?,?,?,?,?,?)',
        (creator_id, title, description, task_type, target_user_id, reward_coins)
    )

def get_romance_tasks(status='open', limit=50):
    return query_db(
        '''SELECT rt.*, u.username as creator_name
           FROM romance_tasks rt JOIN users u ON rt.creator_id = u.id
           WHERE rt.status = ? ORDER BY rt.created_at DESC LIMIT ?''',
        (status, limit))


# ══════════════════════════════════════════════
# 爆料/树洞
# ══════════════════════════════════════════════

def create_gossip(content, station_id=None, images='[]', is_anonymous=1):
    return execute_db(
        'INSERT INTO gossip (content, station_id, images, is_anonymous) VALUES (?,?,?,?)',
        (content, station_id, images, is_anonymous)
    )

def get_gossip(station_id=None, limit=50, offset=0, sort='newest'):
    sql = 'SELECT * FROM gossip WHERE is_deleted = 0'
    args = []
    if station_id:
        sql += ' AND station_id = ?'
        args.append(station_id)
    if sort == 'hot':
        sql += ' ORDER BY is_hot DESC, likes_count DESC'
    else:
        sql += ' ORDER BY created_at DESC'
    sql += ' LIMIT ? OFFSET ?'
    args += [limit, offset]
    return query_db(sql, args)

def get_gossip_by_id(gid):
    return query_db('SELECT * FROM gossip WHERE id = ? AND is_deleted = 0', (gid,), one=True)

def toggle_gossip_like(gid, user_id=None):
    """树洞点赞（幂等）：有 user_id 时按用户去重，无则退化为计数+1"""
    if not user_id:
        execute_db('UPDATE gossip SET likes_count = likes_count + 1 WHERE id = ?', (gid,))
        return {'liked': True}
    # 复用 likes 多态点赞表，target_type='gossip'
    row = query_db(
        'SELECT id FROM likes WHERE user_id = ? AND target_type = ? AND target_id = ?',
        (user_id, 'gossip', gid), one=True)
    if row:
        execute_db('DELETE FROM likes WHERE id = ?', (row['id'],))
        execute_db('UPDATE gossip SET likes_count = MAX(0, likes_count - 1) WHERE id = ?', (gid,))
        return {'liked': False}
    execute_db(
        'INSERT INTO likes (user_id, target_type, target_id) VALUES (?, ?, ?)',
        (user_id, 'gossip', gid))
    execute_db('UPDATE gossip SET likes_count = likes_count + 1 WHERE id = ?', (gid,))
    return {'liked': True}

def create_gossip_comment(gid, content, author_name='匿名'):
    cid = execute_db(
        'INSERT INTO gossip_comments (gossip_id, content, author_name) VALUES (?,?,?)',
        (gid, content, author_name)
    )
    execute_db('UPDATE gossip SET comments_count = comments_count + 1 WHERE id = ?', (gid,))
    return cid

def get_gossip_comments(gid, limit=100):
    return query_db(
        'SELECT * FROM gossip_comments WHERE gossip_id = ? AND is_deleted = 0 ORDER BY created_at ASC LIMIT ?',
        (gid, limit))


# ══════════════════════════════════════════════
# 交易帖
# ══════════════════════════════════════════════

def create_trade_post(post_id, user_id, price, original_price=0, condition='good', category='', contact=''):
    return execute_db(
        'INSERT INTO trade_posts (post_id, user_id, price, original_price, condition, category, contact) VALUES (?,?,?,?,?,?,?)',
        (post_id, user_id, price, original_price, condition, category, contact)
    )

def get_trade_posts(category=None, status='available', limit=50, offset=0):
    sql = '''SELECT tp.*, p.title, p.content, p.image, u.username, u.avatar
             FROM trade_posts tp
             JOIN posts p ON tp.post_id = p.id
             JOIN users u ON tp.user_id = u.id
             WHERE tp.status = ? AND p.is_deleted = 0'''
    args = [status]
    if category:
        sql += ' AND tp.category = ?'
        args.append(category)
    sql += ' ORDER BY tp.created_at DESC LIMIT ? OFFSET ?'
    args += [limit, offset]
    return query_db(sql, args)

def update_trade_status(trade_id, status):
    execute_db('UPDATE trade_posts SET status = ? WHERE id = ?', (status, trade_id))


# ══════════════════════════════════════════════
# 看板娘
# ══════════════════════════════════════════════

KANBAN_GREETINGS = [
    "欢迎回来！今天也要元气满满哦~ ✨",
    "主人，你终于来啦！我等你好久了~ 💕",
    "今天想做些什么呢？发帖？逛子站？还是...和我聊天？😊",
    "校园墙因为有你而精彩！加油！🌟",
    "有什么烦恼的话，可以去树洞说说哦~ 🌳",
    "记得每天签到领金币呀！💰",
    "听说恋爱情报专区有新动态，要不要去看看？💌",
    "主人辛苦了！要不要休息一下？☕",
]

KANBAN_TIPS = [
    "💡 小贴士：连续签到可以获得额外奖励哦！",
    "💡 小贴士：在积分商城可以兑换专属徽章！",
    "💡 小贴士：发帖时可以选择不同的帖子类型~",
    "💡 小贴士：恋爱情报专区可以匿名表白！",
    "💡 小贴士：关注感兴趣的人，不错过他们的动态！",
]

def get_kanban_message():
    """获取随机看板娘消息"""
    msg = random.choice(KANBAN_GREETINGS + KANBAN_TIPS)
    return msg


# ══════════════════════════════════════════════
# 推流算法
# ══════════════════════════════════════════════

def get_recommended_posts(user_id=None, limit=20, offset=0):
    """推流算法：综合热度、时间、用户兴趣"""
    sql = '''
        SELECT p.*, u.username as author_name, u.avatar as author_avatar,
               s.name as station_name, s.icon as station_icon,
               (p.likes_count * 3 + p.comments_count * 5 + p.views * 0.1) as hot_score
        FROM posts p
        JOIN users u ON p.author_id = u.id
        JOIN stations s ON p.station_id = s.id
        WHERE p.is_deleted = 0
        ORDER BY
            p.is_pinned DESC,
            (p.likes_count * 3 + p.comments_count * 5 + p.views * 0.1) * 0.6
            + (JULIANDAY('now') - JULIANDAY(p.created_at)) * (-0.4)
            DESC
        LIMIT ? OFFSET ?
    '''
    return query_db(sql, (limit, offset))


def get_user_interest_stations(user_id, limit=5):
    """根据用户互动历史推荐子站"""
    return query_db('''
        SELECT s.id, s.name, s.icon, COUNT(*) as interaction_count
        FROM posts p
        JOIN stations s ON p.station_id = s.id
        WHERE p.author_id = ? OR p.id IN (
            SELECT target_id FROM likes WHERE user_id = ? AND target_type = 'post'
        )
        GROUP BY s.id
        ORDER BY interaction_count DESC
        LIMIT ?
    ''', (user_id, user_id, limit))


# ══════════════════════════════════════════════
# 搜索算法
# ══════════════════════════════════════════════

def smart_search(keyword, user_id=None):
    """综合搜索：子站 + 帖子 + 用户"""
    results = {'stations': [], 'posts': [], 'users': []}

    results['stations'] = query_db(
        '''SELECT *, (user_count * 2 + post_count) as relevance
           FROM stations WHERE name LIKE ? OR description LIKE ?
           ORDER BY relevance DESC LIMIT 10''',
        (f'%{keyword}%', f'%{keyword}%')
    )

    results['posts'] = query_db(
        '''SELECT p.*, u.username as author_name, u.avatar as author_avatar,
                  s.name as station_name, s.icon as station_icon
           FROM posts p JOIN users u ON p.author_id = u.id JOIN stations s ON p.station_id = s.id
           WHERE p.is_deleted = 0 AND (p.title LIKE ? OR p.content LIKE ?)
           ORDER BY p.likes_count DESC LIMIT 15''',
        (f'%{keyword}%', f'%{keyword}%')
    )

    results['users'] = query_db(
        'SELECT id, username, avatar, bio FROM users WHERE username LIKE ? OR bio LIKE ? LIMIT 10',
        (f'%{keyword}%', f'%{keyword}%')
    )

    return results


# ══════════════════════════════════════════════
# 管理后台
# ══════════════════════════════════════════════

def get_admin_stats():
    return {
        'users': query_db('SELECT COUNT(*) as c FROM users', one=True)['c'],
        'stations': query_db('SELECT COUNT(*) as c FROM stations', one=True)['c'],
        'posts': query_db('SELECT COUNT(*) as c FROM posts WHERE is_deleted = 0', one=True)['c'],
        'comments': query_db('SELECT COUNT(*) as c FROM comments WHERE is_deleted = 0', one=True)['c'],
        'today_posts': query_db("SELECT COUNT(*) as c FROM posts WHERE date(created_at) = date('now')", one=True)['c'],
        'today_users': query_db("SELECT COUNT(*) as c FROM users WHERE date(created_at) = date('now')", one=True)['c'],
    }

def get_all_users_admin(limit=100, offset=0):
    return query_db(
        'SELECT id, username, email, avatar, role, coins, points, level, identity_group, created_at FROM users ORDER BY id DESC LIMIT ? OFFSET ?',
        (limit, offset))

def update_user_admin(uid, **kwargs):
    allowed = {'role', 'coins', 'points', 'level', 'identity_group', 'username', 'bio'}
    fields = {k: v for k, v in kwargs.items() if k in allowed}
    if not fields:
        return False
    sets = ', '.join(f'{k} = ?' for k in fields)
    vals = list(fields.values()) + [uid]
    execute_db(f'UPDATE users SET {sets}, updated_at = CURRENT_TIMESTAMP WHERE id = ?', vals)
    return True

def admin_log(admin_id, action, target_type='', target_id=0, detail=''):
    execute_db(
        'INSERT INTO admin_log (admin_id, action, target_type, target_id, detail) VALUES (?,?,?,?,?)',
        (admin_id, action, target_type, target_id, detail)
    )

def get_admin_logs(limit=100):
    return query_db(
        '''SELECT al.*, u.username as admin_name
           FROM admin_log al LEFT JOIN users u ON al.admin_id = u.id
           ORDER BY al.created_at DESC LIMIT ?''', (limit,))


# ══════════════════════════════════════════════
# 收藏
# ══════════════════════════════════════════════

def toggle_favorite(user_id, target_type, target_id):
    """收藏 / 取消收藏，返回 {favorited: bool}"""
    row = query_db(
        'SELECT id FROM favorites WHERE user_id = ? AND target_type = ? AND target_id = ?',
        (user_id, target_type, target_id), one=True)
    if row:
        execute_db('DELETE FROM favorites WHERE id = ?', (row['id'],))
        return False
    execute_db('INSERT INTO favorites (user_id, target_type, target_id) VALUES (?,?,?)',
               (user_id, target_type, target_id))
    return True


def is_favorited(user_id, target_type, target_id):
    return query_db(
        'SELECT 1 FROM favorites WHERE user_id = ? AND target_type = ? AND target_id = ?',
        (user_id, target_type, target_id), one=True) is not None


def get_favorites(user_id, target_type='post', limit=50, offset=0):
    """获取用户的收藏列表（关联帖子/子站信息）"""
    if target_type == 'post':
        return query_db(
            '''SELECT f.target_id, f.created_at as favorited_at, p.*, u.username as author_name,
                      s.name as station_name, s.icon as station_icon
               FROM favorites f
               JOIN posts p ON f.target_id = p.id AND p.is_deleted = 0
               LEFT JOIN users u ON p.author_id = u.id
               LEFT JOIN stations s ON p.station_id = s.id
               WHERE f.user_id = ? AND f.target_type = 'post'
               ORDER BY f.created_at DESC LIMIT ? OFFSET ?''',
            (user_id, limit, offset))
    if target_type == 'station':
        return query_db(
            '''SELECT f.target_id, f.created_at as favorited_at, s.*
               FROM favorites f JOIN stations s ON f.target_id = s.id
               WHERE f.user_id = ? AND f.target_type = 'station'
               ORDER BY f.created_at DESC LIMIT ? OFFSET ?''',
            (user_id, limit, offset))
    return []


# ══════════════════════════════════════════════
# 举报
# ══════════════════════════════════════════════

def create_report(reporter_id, target_type, target_id, reason='', detail=''):
    """提交举报，返回举报id（同一目标不可重复举报）"""
    existing = query_db(
        'SELECT id FROM reports WHERE reporter_id = ? AND target_type = ? AND target_id = ? AND status = ?',
        (reporter_id, target_type, target_id, 'pending'), one=True)
    if existing:
        return None
    return execute_db(
        'INSERT INTO reports (reporter_id, target_type, target_id, reason, detail) VALUES (?,?,?,?,?)',
        (reporter_id, target_type, target_id, reason, detail))


def get_reports(status='pending', limit=100):
    """管理端举报列表"""
    return query_db(
        '''SELECT r.*, u.username as reporter_name, t.username as handler_name
           FROM reports r
           LEFT JOIN users u ON r.reporter_id = u.id
           LEFT JOIN users t ON r.handler_id = t.id
           WHERE ? = 'all' OR r.status = ?
           ORDER BY r.created_at DESC LIMIT ?''',
        (status, status, limit))


def handle_report(report_id, handler_id, status, note=''):
    """处理举报：approve（确认违规）/ reject（驳回）"""
    execute_db(
        'UPDATE reports SET status = ?, handler_id = ?, handled_at = CURRENT_TIMESTAMP WHERE id = ?',
        (status, handler_id, report_id))
    return True


# ══════════════════════════════════════════════
# 通知偏好设置
# ══════════════════════════════════════════════

def get_user_settings(user_id):
    row = query_db('SELECT * FROM user_settings WHERE user_id = ?', (user_id,), one=True)
    if row:
        return dict(row)
    return {
        'notify_comment': 1, 'notify_like': 1, 'notify_follow': 1,
        'notify_system': 1, 'theme': 'auto'
    }


def save_user_settings(user_id, **kwargs):
    allowed = ('notify_comment', 'notify_like', 'notify_follow', 'notify_system', 'theme')
    fields = {k: int(v) for k, v in kwargs.items() if k in allowed and k != 'theme'}
    if 'theme' in kwargs:
        fields['theme'] = kwargs['theme']
    if not fields:
        return False
    sets = ', '.join(f'{k} = ?' for k in fields)
    vals = list(fields.values()) + [user_id]
    execute_db(
        f'''INSERT INTO user_settings (user_id, {', '.join(fields.keys())}) VALUES (?, {', '.join('?' for _ in fields)})
            ON CONFLICT(user_id) DO UPDATE SET {sets}, updated_at = CURRENT_TIMESTAMP''',
        [user_id] + list(fields.values()) + vals)
    return True
