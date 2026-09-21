"""
扩展数据模型 — 身份组、积分、签到、虚拟币、恋爱情报、交易、看板娘、管理后台
"""
import os
import json
import re
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
    for col, default in [('post_type', "'text'"), ('images', "'[]'"), ('extra', "'{}'"), ('is_anonymous', "'0'"), ('status', "'approved'")]:
        try:
            c.execute(f"ALTER TABLE posts ADD COLUMN {col} TEXT DEFAULT {default}")
        except sqlite3.OperationalError:
            pass
    try:
        c.execute("CREATE INDEX IF NOT EXISTS idx_posts_status ON posts(status, created_at)")
    except sqlite3.OperationalError:
        pass

    # ── 举报增强（R8）：证据图列 ──
    try:
        c.execute("ALTER TABLE reports ADD COLUMN evidence TEXT DEFAULT '[]'")
    except sqlite3.OperationalError:
        pass

    # ── 皮肤系统（R9）：用户偏好列 ──
    try:
        c.execute("ALTER TABLE user_settings ADD COLUMN skin TEXT DEFAULT 'glass'")
    except sqlite3.OperationalError:
        pass

    # ── 身份组 ──
    c.execute('''CREATE TABLE IF NOT EXISTS identity_groups (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        name TEXT NOT NULL UNIQUE,
        icon TEXT DEFAULT 'tag',
        color TEXT DEFAULT '#fb6f92',
        description TEXT DEFAULT '',
        permissions TEXT DEFAULT '[]',
        min_level INTEGER DEFAULT 1,
        is_default INTEGER DEFAULT 0,
        sort_order INTEGER DEFAULT 0,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )''')

    # ── 站内公告 ──
    c.execute('''CREATE TABLE IF NOT EXISTS site_announcements (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        title TEXT NOT NULL,
        content TEXT DEFAULT '',
        created_by INTEGER REFERENCES users(id),
        is_active INTEGER DEFAULT 1,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )''')

    # ── 站点配置（键值）：广告位等可配置占位 ──
    c.execute('''CREATE TABLE IF NOT EXISTS site_config (
        key TEXT PRIMARY KEY,
        value TEXT DEFAULT ''
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
        icon TEXT DEFAULT 'gift',
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

    # ── 角色关系图（world）：多人共同新建/串联维护的关系图谱 ──
    c.execute('''CREATE TABLE IF NOT EXISTS character_nodes (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        name TEXT NOT NULL,
        portrait TEXT DEFAULT '',
        tagline TEXT DEFAULT '',
        color TEXT DEFAULT '',
        status TEXT DEFAULT 'active',
        created_by INTEGER REFERENCES users(id),
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )''')
    c.execute('''CREATE TABLE IF NOT EXISTS character_relations (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        from_id INTEGER NOT NULL REFERENCES character_nodes(id),
        to_id INTEGER NOT NULL REFERENCES character_nodes(id),
        label TEXT NOT NULL,
        description TEXT DEFAULT '',
        reciprocal INTEGER DEFAULT 0,
        created_by INTEGER REFERENCES users(id),
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )''')
    c.execute("CREATE INDEX IF NOT EXISTS idx_char_rel_src ON character_relations(from_id)")
    c.execute("CREATE INDEX IF NOT EXISTS idx_char_rel_dst ON character_relations(to_id)")

    # ── 话题/标签 + 热搜（R8）──
    c.execute('''CREATE TABLE IF NOT EXISTS topics (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        name TEXT NOT NULL UNIQUE,
        created_by INTEGER REFERENCES users(id),
        use_count INTEGER DEFAULT 0,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )''')
    c.execute('''CREATE TABLE IF NOT EXISTS post_topics (
        topic_id INTEGER NOT NULL REFERENCES topics(id),
        post_id INTEGER NOT NULL REFERENCES posts(id),
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        PRIMARY KEY (topic_id, post_id)
    )''')
    c.execute("CREATE INDEX IF NOT EXISTS idx_post_topics_post ON post_topics(post_id)")
    c.execute("CREATE INDEX IF NOT EXISTS idx_post_topics_topic ON post_topics(topic_id)")

    # ── 任务系统（R8）：系统任务 / 管理员任务 / 积分悬赏 ──
    c.execute('''CREATE TABLE IF NOT EXISTS tasks (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        kind TEXT NOT NULL DEFAULT 'system',
        title TEXT NOT NULL,
        description TEXT DEFAULT '',
        reward_coins INTEGER DEFAULT 0,
        reward_points INTEGER DEFAULT 0,
        cost INTEGER DEFAULT 0,
        max_claims INTEGER DEFAULT 0,
        status TEXT DEFAULT 'active',
        created_by INTEGER REFERENCES users(id),
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )''')
    c.execute('''CREATE TABLE IF NOT EXISTS task_claims (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        task_id INTEGER NOT NULL REFERENCES tasks(id),
        user_id INTEGER NOT NULL REFERENCES users(id),
        status TEXT DEFAULT 'in_progress',
        proof TEXT DEFAULT '',
        claimed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        completed_at TIMESTAMP,
        UNIQUE(task_id, user_id)
    )''')
    c.execute("CREATE INDEX IF NOT EXISTS idx_task_claims_user ON task_claims(user_id)")
    c.execute("CREATE INDEX IF NOT EXISTS idx_task_claims_task ON task_claims(task_id)")

    # ── 聊天室（R10）：多主题房间 + 成员 + 消息（撤回软删、@提及存 JSON）──
    c.execute('''CREATE TABLE IF NOT EXISTS chat_rooms (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        name TEXT NOT NULL,
        description TEXT DEFAULT '',
        icon TEXT DEFAULT 'message-square',
        created_by INTEGER REFERENCES users(id),
        is_public INTEGER DEFAULT 1,
        status TEXT DEFAULT 'active',
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )''')
    c.execute('''CREATE TABLE IF NOT EXISTS chat_members (
        room_id INTEGER NOT NULL REFERENCES chat_rooms(id),
        user_id INTEGER NOT NULL REFERENCES users(id),
        role TEXT DEFAULT 'member',
        joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        PRIMARY KEY (room_id, user_id)
    )''')
    c.execute('''CREATE TABLE IF NOT EXISTS chat_messages (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        room_id INTEGER NOT NULL REFERENCES chat_rooms(id),
        sender_id INTEGER NOT NULL REFERENCES users(id),
        content TEXT NOT NULL,
        mentions TEXT DEFAULT '[]',
        is_deleted INTEGER DEFAULT 0,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )''')
    c.execute("CREATE INDEX IF NOT EXISTS idx_chat_msgs_room ON chat_messages(room_id, id)")

    # ── 悬赏问答（R11 暗阁）：提问预扣积分，采纳放款 ──
    c.execute('''
        CREATE TABLE IF NOT EXISTS bounty_questions (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            user_id INTEGER NOT NULL REFERENCES users(id),
            title TEXT NOT NULL,
            content TEXT DEFAULT '',
            bounty INTEGER DEFAULT 0,
            status TEXT DEFAULT 'open',
            accepted_answer_id INTEGER,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )''')
    c.execute('''
        CREATE TABLE IF NOT EXISTS bounty_answers (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            question_id INTEGER NOT NULL REFERENCES bounty_questions(id),
            user_id INTEGER NOT NULL REFERENCES users(id),
            content TEXT NOT NULL,
            is_accepted INTEGER DEFAULT 0,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )''')
    c.execute("CREATE INDEX IF NOT EXISTS idx_bqa_q ON bounty_answers(question_id)")

    # ── 活动系统（R11）：发布 / 报名 / 打卡 ──
    c.execute('''CREATE TABLE IF NOT EXISTS events (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        title TEXT NOT NULL,
        description TEXT DEFAULT '',
        location TEXT DEFAULT '',
        event_date TEXT NOT NULL,
        capacity INTEGER DEFAULT 0,
        reward_coins INTEGER DEFAULT 0,
        status TEXT DEFAULT 'active',
        created_by INTEGER REFERENCES users(id),
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )''')
    c.execute('''CREATE TABLE IF NOT EXISTS event_registrations (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        event_id INTEGER NOT NULL REFERENCES events(id),
        user_id INTEGER NOT NULL REFERENCES users(id),
        checked_in INTEGER DEFAULT 0,
        registered_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        checked_in_at TIMESTAMP,
        UNIQUE(event_id, user_id)
    )''')
    c.execute("CREATE INDEX IF NOT EXISTS idx_event_reg_event ON event_registrations(event_id)")

    # ── 暗阁完整版（R11）：付费内容 + 推流加权 ──
    c.execute('''CREATE TABLE IF NOT EXISTS content_unlocks (
        user_id INTEGER NOT NULL REFERENCES users(id),
        post_id INTEGER NOT NULL REFERENCES posts(id),
        creator_id INTEGER NOT NULL,
        points INTEGER NOT NULL DEFAULT 0,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        PRIMARY KEY (user_id, post_id)
    )''')
    for col, coltype, default in [
        ('pay_points', 'INTEGER', '0'),
        ('boosted_until', "TEXT", "''"),
    ]:
        try:
            c.execute(f"ALTER TABLE posts ADD COLUMN {col} {coltype} DEFAULT {default}")
        except sqlite3.OperationalError:
            pass

    # ── 等级规则 + AI（R12）──
    c.execute('''CREATE TABLE IF NOT EXISTS level_rules (
        level INTEGER PRIMARY KEY,
        exp_required INTEGER NOT NULL,
        reward_coins INTEGER DEFAULT 0
    )''')
    for lvl, exp, coins in ((2, 50, 20), (3, 120, 30), (4, 250, 50), (5, 500, 80)):
        c.execute('INSERT OR IGNORE INTO level_rules (level, exp_required, reward_coins) VALUES (?,?,?)',
                  (lvl, exp, coins))
    try:
        c.execute("ALTER TABLE identity_groups ADD COLUMN auto_assign INTEGER DEFAULT 0")
    except sqlite3.OperationalError:
        pass
    try:
        c.execute("ALTER TABLE identity_groups ADD COLUMN is_public INTEGER DEFAULT 1")
    except sqlite3.OperationalError:
        pass
    try:
        c.execute("ALTER TABLE user_settings ADD COLUMN ai_reply INTEGER DEFAULT 0")
    except sqlite3.OperationalError:
        pass

    # ── 私信 ──
    init_dm_tables(c)

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

    # 签到经验（R12）：+5
    award_exp(user_id, 5)

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
            new_title = (new_title or '彩虹用户')
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
        # 与 INSERT 分支一致：列表列序列化、布尔转 int、age 强转（修复 UPDATE 传 list 直接 500）
        if 'hobbies' in fields and not isinstance(fields['hobbies'], str):
            fields['hobbies'] = json.dumps(fields['hobbies'] or [], ensure_ascii=False)
        if 'looking_for' in fields and not isinstance(fields['looking_for'], str):
            fields['looking_for'] = json.dumps(fields['looking_for'] or [], ensure_ascii=False)
        if 'is_visible' in fields:
            fields['is_visible'] = 1 if fields['is_visible'] else 0
        if 'age' in fields:
            try:
                fields['age'] = int(fields['age'] or 0)
            except (TypeError, ValueError):
                fields['age'] = 0
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

def get_trade_posts(category=None, status='available', limit=50, offset=0, keyword=None):
    sql = '''SELECT tp.*, p.title, p.content, p.image, u.username, u.avatar
             FROM trade_posts tp
             JOIN posts p ON tp.post_id = p.id
             JOIN users u ON tp.user_id = u.id
             WHERE tp.status = ? AND p.is_deleted = 0'''
    args = [status]
    if category:
        sql += ' AND tp.category = ?'
        args.append(category)
    if keyword:
        sql += ' AND (p.title LIKE ? OR p.content LIKE ?)'
        kw = f'%{keyword}%'
        args += [kw, kw]
    sql += ' ORDER BY tp.created_at DESC LIMIT ? OFFSET ?'
    args += [limit, offset]
    return query_db(sql, args)

def update_trade_status(trade_id, status):
    execute_db('UPDATE trade_posts SET status = ? WHERE id = ?', (status, trade_id))


# ══════════════════════════════════════════════
# 看板娘
# ══════════════════════════════════════════════

KANBAN_GREETINGS = [
    "欢迎回来！今天也要元气满满哦~",
    "主人，你终于来啦！我等你好久了~",
    "今天想做些什么呢？发帖？逛子站？还是...和我聊天？",
    "校园墙因为有你而精彩！加油！",
    "有什么烦恼的话，可以去树洞说说哦~",
    "记得每天签到领金币呀！",
    "听说恋爱情报专区有新动态，要不要去看看？",
    "主人辛苦了！要不要休息一下？来杯咖啡~",
]

KANBAN_TIPS = [
    "小贴士：连续签到可以获得额外奖励哦！",
    "小贴士：在积分商城可以兑换专属徽章！",
    "小贴士：发帖时可以选择不同的帖子类型~",
    "小贴士：恋爱情报专区可以匿名表白！",
    "小贴士：关注感兴趣的人，不错过他们的动态！",
]

def get_kanban_message():
    """获取随机看板娘消息"""
    msg = random.choice(KANBAN_GREETINGS + KANBAN_TIPS)
    return msg


# ══════════════════════════════════════════════
# 推流算法
# ══════════════════════════════════════════════

def get_recommended_posts(user_id=None, limit=20, offset=0, station_id=None):
    """推流算法：综合热度、时间、用户兴趣。
    审核闸：仅 approved；脱敏交 handler 层 shape_post（保留 owner/admin 例外）。
    station_id：可选，仅返回该子站内容（供「我关注的子站」筛选）。"""
    where = "p.is_deleted = 0 AND p.status = 'approved'"
    params = []
    if station_id:
        where += " AND p.station_id = ?"
        params.append(station_id)
    params.append(limit)
    params.append(offset)
    sql = f'''
        SELECT p.*, u.username as author_name, u.avatar as author_avatar,
               s.name as station_name, s.icon as station_icon,
               (p.likes_count * 3 + p.comments_count * 5 + p.views * 0.1) as hot_score
        FROM posts p
        JOIN users u ON p.author_id = u.id
        JOIN stations s ON p.station_id = s.id
        WHERE {where}
        ORDER BY
            CASE WHEN p.boosted_until >= datetime('now') THEN 1 ELSE 0 END DESC,
            p.is_pinned DESC,
            (p.likes_count * 3 + p.comments_count * 5 + p.views * 0.1) * 0.6
            + (JULIANDAY('now') - JULIANDAY(p.created_at)) * (-0.4)
            DESC
        LIMIT ? OFFSET ?
    '''
    return query_db(sql, params)


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
           WHERE p.is_deleted = 0 AND p.status = 'approved' AND (p.title LIKE ? OR p.content LIKE ?)
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

def get_admin_stats_series(days=7):
    """返回最近 days 天的新增用户 / 帖子 / 评论趋势"""
    days = max(1, min(30, int(days)))
    labels, users, posts, comments = [], [], [], []
    for i in range(days - 1, -1, -1):
        d = (datetime.now() - timedelta(days=i)).strftime('%Y-%m-%d')
        labels.append(d[5:])  # MM-DD
        users.append(query_db(
            "SELECT COUNT(*) as c FROM users WHERE date(created_at) = ?", (d,), one=True)['c'])
        posts.append(query_db(
            "SELECT COUNT(*) as c FROM posts WHERE date(created_at) = ? AND is_deleted = 0", (d,), one=True)['c'])
        comments.append(query_db(
            "SELECT COUNT(*) as c FROM comments WHERE date(created_at) = ? AND is_deleted = 0", (d,), one=True)['c'])
    return {
        'labels': labels,
        'users': users,
        'posts': posts,
        'comments': comments,
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


# ── 站点配置（键值）──
_SITE_CONFIG_DEFAULTS = {
    'ad_enabled': '1',
    'ad_header': '校园墙 · 商业合作 / 品牌橱窗 招商中（预留广告位）',
    'ad_footer': '广告位招租 · 联系站务合作（预留）',
    'visitor_mode': 'open',
    'ai_moderation': '0',
    'custom_css': '',
    'custom_html': '',
    'custom_js': '',
}


def visitor_mode_open():
    """访客模式（R9）：closed 时未登录用户不能浏览内容流。"""
    return get_site_config().get('visitor_mode', 'open') != 'closed'


def ai_moderation_enabled():
    """AI 审核自动模式（R12）开关。"""
    return get_site_config().get('ai_moderation', '0') == '1'


def get_site_config():
    rows = query_db('SELECT key, value FROM site_config')
    cfg = dict(_SITE_CONFIG_DEFAULTS)
    for r in rows:
        cfg[r['key']] = r['value']
    return cfg


def set_site_config(key, value):
    if key not in _SITE_CONFIG_DEFAULTS:
        return False
    execute_db('INSERT INTO site_config (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value',
               (key, str(value)[:200]))
    return True


def get_ad_config():
    cfg = get_site_config()
    return {
        'ad_enabled': cfg.get('ad_enabled') in ('1', 'true', 'True', 'on'),
        'ad_header': cfg.get('ad_header', ''),
        'ad_footer': cfg.get('ad_footer', ''),
    }


# ── 站内公告 ──
def get_announcements(active_only=False, limit=20):
    sql = '''SELECT a.*, u.username as creator_name
             FROM site_announcements a LEFT JOIN users u ON a.created_by = u.id'''
    args = []
    if active_only:
        sql += ' WHERE a.is_active = 1'
    sql += ' ORDER BY a.created_at DESC LIMIT ?'
    args.append(limit)
    return query_db(sql, args)

def create_announcement(title, content, admin_id):
    execute_db(
        'INSERT INTO site_announcements (title, content, created_by) VALUES (?,?,?)',
        (title, content, admin_id))

def update_announcement(aid, title=None, content=None):
    if title is not None:
        execute_db('UPDATE site_announcements SET title = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?', (title, aid))
    if content is not None:
        execute_db('UPDATE site_announcements SET content = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?', (content, aid))

def toggle_announcement(aid, is_active):
    execute_db('UPDATE site_announcements SET is_active = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?', (1 if is_active else 0, aid))

def delete_announcement(aid):
    execute_db('DELETE FROM site_announcements WHERE id = ?', (aid,))


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
                      u.avatar as author_avatar, u.identity_group as author_identity_group,
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

# 举报分类（与前端举报弹窗标签一致；R8 举报增强）
REPORT_CATEGORIES = ('广告', '色情低俗', '暴力', '诈骗', '辱骂', '侵权', '其他')


# ══════════════════════════════════════════════
# 数据导出（R8）：用户个人数据 JSON
# ══════════════════════════════════════════════

def get_user_export(user_id):
    """个人数据导出：各类数据上限 500 条，按时间倒序。"""
    posts = query_db(
        'SELECT id, title, content, post_type, images, status, likes_count, comments_count, created_at '
        'FROM posts WHERE author_id = ? AND is_deleted = 0 ORDER BY id DESC LIMIT 500', (user_id,))
    comments = query_db(
        'SELECT id, post_id, content, created_at FROM comments '
        'WHERE author_id = ? AND is_deleted = 0 ORDER BY id DESC LIMIT 500', (user_id,))
    likes = query_db(
        'SELECT target_type, target_id, created_at FROM likes WHERE user_id = ? ORDER BY id DESC LIMIT 500', (user_id,))
    favorites = query_db(
        'SELECT target_type, target_id, created_at FROM favorites WHERE user_id = ? ORDER BY id DESC LIMIT 500', (user_id,))
    dms = query_db(
        'SELECT id, sender_id, receiver_id, content, created_at FROM dm_messages '
        'WHERE sender_id = ? OR receiver_id = ? ORDER BY id DESC LIMIT 500', (user_id, user_id))
    checkins = query_db(
        'SELECT checkin_date, streak, coins_earned, points_earned FROM checkins '
        'WHERE user_id = ? ORDER BY checkin_date DESC LIMIT 500', (user_id,))
    profile = query_db(
        'SELECT id, username, email, avatar, bio, role, coins, points, level, exp, '
        'checkin_streak, identity_group, title, created_at FROM users WHERE id = ?', (user_id,), one=True)
    return {
        'exported_at': query_db("SELECT datetime('now') AS n", one=True)['n'],
        'profile': profile,
        'posts': posts,
        'comments': comments,
        'likes': likes,
        'favorites': favorites,
        'dm_messages': dms,
        'checkins': checkins,
    }


# ══════════════════════════════════════════════
# 聊天室（R10）：房间 / 成员 / 消息（@提及、撤回）
# ══════════════════════════════════════════════

def create_chat_room(name, description='', icon='message-square', is_public=1, created_by=None):
    name = (name or '').strip()
    if not name or len(name) > 40:
        return None
    rid = execute_db(
        'INSERT INTO chat_rooms (name, description, icon, is_public, created_by) VALUES (?,?,?,?,?)',
        (name, (description or '').strip()[:200], icon or 'message-square', 1 if is_public else 0, created_by))
    if rid and created_by:
        execute_db("INSERT OR IGNORE INTO chat_members (room_id, user_id, role) VALUES (?,?, 'owner')", (rid, created_by))
    return rid


def get_chat_room(room_id):
    return query_db('''
        SELECT cr.*, u.username AS creator_name,
               (SELECT COUNT(*) FROM chat_members cm WHERE cm.room_id = cr.id) AS member_count,
               (SELECT COUNT(*) FROM chat_messages cm2 WHERE cm2.room_id = cr.id AND cm2.is_deleted = 0) AS message_count
        FROM chat_rooms cr LEFT JOIN users u ON u.id = cr.created_by
        WHERE cr.id = ? AND cr.status = 'active' ''', (room_id,), one=True)


def list_chat_rooms(user_id=None, limit=50):
    """房间列表：成员数 + 最后一条消息预览 + 我是否成员。"""
    my_sel = ''',
               (SELECT 1 FROM chat_members mm WHERE mm.room_id = cr.id AND mm.user_id = ?) AS is_member'''
    args = [user_id] if user_id else []
    my_sel = my_sel if user_id else ''',
               0 AS is_member'''
    return query_db(f'''
        SELECT cr.id, cr.name, cr.description, cr.icon, cr.is_public, cr.created_by, cr.created_at,
               u.username AS creator_name,
               (SELECT COUNT(*) FROM chat_members cm WHERE cm.room_id = cr.id) AS member_count,
               (SELECT cm2.content FROM chat_messages cm2 WHERE cm2.room_id = cr.id AND cm2.is_deleted = 0
                 ORDER BY cm2.id DESC LIMIT 1) AS last_message{my_sel}
        FROM chat_rooms cr LEFT JOIN users u ON u.id = cr.created_by
        WHERE cr.status = 'active'
        ORDER BY cr.id DESC LIMIT ?''', args + [limit])


def is_room_member(room_id, user_id):
    return bool(query_db('SELECT 1 FROM chat_members WHERE room_id = ? AND user_id = ?', (room_id, user_id), one=True))


def join_chat_room(room_id, user_id):
    r = query_db('SELECT * FROM chat_rooms WHERE id = ? AND status = ?', (room_id, 'active'), one=True)
    if not r:
        return None, '房间不存在'
    if not r['is_public']:
        return None, '私有房间仅限受邀请成员'
    execute_db('INSERT OR IGNORE INTO chat_members (room_id, user_id) VALUES (?,?)', (room_id, user_id))
    return True, None


def leave_chat_room(room_id, user_id):
    r = query_db('SELECT created_by FROM chat_rooms WHERE id = ?', (room_id,), one=True)
    if r and r['created_by'] == user_id:
        return None, '房主不能退出，可转让或关闭房间'
    execute_db('DELETE FROM chat_members WHERE room_id = ? AND user_id = ?', (room_id, user_id))
    return True, None


def chat_room_members(room_id):
    return query_db('''
        SELECT cm.user_id, cm.role, cm.joined_at, u.username, u.avatar
        FROM chat_members cm JOIN users u ON u.id = cm.user_id
        WHERE cm.room_id = ? ORDER BY cm.joined_at ASC''', (room_id,))


def add_chat_member(room_id, user_id, operator):
    """房主或管理员添加成员（私有房间唯一入房途径）。"""
    r = query_db('SELECT created_by FROM chat_rooms WHERE id = ?', (room_id,), one=True)
    if not r:
        return None, '房间不存在'
    if r['created_by'] != operator['id'] and operator['role'] != 'admin':
        return None, '仅房主或管理员可添加成员'
    execute_db('INSERT OR IGNORE INTO chat_members (room_id, user_id) VALUES (?,?)', (room_id, user_id))
    return True, None


def parse_mentions(content):
    """从消息中解析 @用户名 → 已存在用户的 id 列表（去重，int）。"""
    names = re.findall(r'@([\w\u4e00-\u9fa5]{1,20})', content or '')
    ids = []
    for n in names:
        row = query_db('SELECT id FROM users WHERE username = ?', (n,), one=True)
        if row:
            uid = int(row['id'])
            if uid not in ids:
                ids.append(uid)
    return ids


def send_chat_message(room_id, sender_id, content):
    content = (content or '').strip()
    if not content:
        return None, '消息不能为空'
    if len(content) > 1000:
        return None, '消息过长（≤1000字）'
    if not is_room_member(room_id, sender_id):
        return None, '请先加入房间'
    import json as _json
    mentions = parse_mentions(content)
    mid = execute_db(
        'INSERT INTO chat_messages (room_id, sender_id, content, mentions) VALUES (?,?,?,?)',
        (room_id, sender_id, content, _json.dumps(mentions)))
    # @提及通知
    sender = query_db('SELECT username FROM users WHERE id = ?', (sender_id,), one=True)
    room = query_db('SELECT name FROM chat_rooms WHERE id = ?', (room_id,), one=True)
    for uid in mentions:
        if uid != sender_id:
            try:
                from app.models import create_notification
                create_notification(uid, sender_id, 'mention',
                                    f"{sender['username']} 在聊天室「{room['name']}」提到了你", f'/chat?room={room_id}')
            except Exception:
                pass
    return mid, None


def get_chat_messages(room_id, after_id=0, limit=50):
    """增量拉取：after_id>0 时取其后消息（轮询），否则取最新 limit 条（升序返回）。"""
    if after_id:
        return query_db('''
            SELECT m.id, m.room_id, m.sender_id, m.content, m.mentions, m.is_deleted, m.created_at,
                   u.username AS sender_name, u.avatar AS sender_avatar
            FROM chat_messages m JOIN users u ON u.id = m.sender_id
            WHERE m.room_id = ? AND m.id > ?
            ORDER BY m.id ASC LIMIT ?''', (room_id, after_id, limit))
    rows = query_db('''
        SELECT m.id, m.room_id, m.sender_id, m.content, m.mentions, m.is_deleted, m.created_at,
               u.username AS sender_name, u.avatar AS sender_avatar
        FROM chat_messages m JOIN users u ON u.id = m.sender_id
        WHERE m.room_id = ?
        ORDER BY m.id DESC LIMIT ?''', (room_id, limit))
    return list(reversed(rows))


def delete_chat_message(msg_id, user_id, is_admin=False):
    m = query_db('SELECT * FROM chat_messages WHERE id = ?', (msg_id,), one=True)
    if not m:
        return None, '消息不存在'
    if m['sender_id'] != user_id and not is_admin:
        return None, '只能撤回自己的消息'
    execute_db('UPDATE chat_messages SET is_deleted = 1, content = ? WHERE id = ?', ('', msg_id))
    return True, None


def create_report(reporter_id, target_type, target_id, reason='', detail='', evidence=None):
    """提交举报，返回举报id（同一目标不可重复举报）。evidence: 证据图 url 列表。"""
    existing = query_db(
        'SELECT id FROM reports WHERE reporter_id = ? AND target_type = ? AND target_id = ? AND status = ?',
        (reporter_id, target_type, target_id, 'pending'), one=True)
    if existing:
        return None
    import json as _json
    ev = _json.dumps([u for u in (evidence or []) if isinstance(u, str)][:4], ensure_ascii=False)
    return execute_db(
        'INSERT INTO reports (reporter_id, target_type, target_id, reason, detail, evidence) VALUES (?,?,?,?,?,?)',
        (reporter_id, target_type, target_id, reason, detail, ev))


def get_reports(status='pending', category='', limit=100):
    """管理端举报列表（evidence 展开为数组）。category 可选过滤。"""
    rows = query_db(
        '''SELECT r.*, u.username as reporter_name, t.username as handler_name
           FROM reports r
           LEFT JOIN users u ON r.reporter_id = u.id
           LEFT JOIN users t ON r.handler_id = t.id
           WHERE (? = 'all' OR r.status = ?) AND (? = '' OR r.reason = ?)
           ORDER BY r.created_at DESC LIMIT ?''',
        (status, status, category, category, limit))
    import json as _json
    for r in rows:
        try:
            r['evidence'] = _json.loads(r.get('evidence') or '[]') if isinstance(r.get('evidence'), str) else []
        except (ValueError, TypeError):
            r['evidence'] = []
    return rows


def handle_report(report_id, handler_id, status, note=''):
    """处理举报：approve（确认违规）/ reject（驳回）"""
    execute_db(
        'UPDATE reports SET status = ?, handler_id = ?, handled_at = CURRENT_TIMESTAMP WHERE id = ?',
        (status, handler_id, report_id))
    return True


# ══════════════════════════════════════════════
# 私信（R4-M4）
# ══════════════════════════════════════════════

def init_dm_tables(c):
    c.execute('''CREATE TABLE IF NOT EXISTS dm_messages (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        sender_id INTEGER NOT NULL,
        receiver_id INTEGER NOT NULL,
        content TEXT NOT NULL,
        is_read INTEGER DEFAULT 0,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )''')
    c.execute('CREATE INDEX IF NOT EXISTS idx_dm_pair ON dm_messages(sender_id, receiver_id, id)')
    c.execute('CREATE INDEX IF NOT EXISTS idx_dm_recv ON dm_messages(receiver_id, is_read)')


def send_dm(sender_id, receiver_id, content):
    return execute_db(
        'INSERT INTO dm_messages (sender_id, receiver_id, content) VALUES (?,?,?)',
        (sender_id, receiver_id, content))


def get_dm_threads(user_id, limit=30):
    """会话列表：每人最后一条消息 + 未读数 + 对方身份（一条 SQL）。"""
    return query_db(
        '''SELECT m.id AS last_id,
                  CASE WHEN m.sender_id = ? THEN m.receiver_id ELSE m.sender_id END AS peer_id,
                  u.username AS peer_name, u.avatar AS peer_avatar, u.identity_group AS peer_identity,
                  m.content AS last_content, m.created_at AS last_at,
                  (SELECT COUNT(*) FROM dm_messages d
                    WHERE d.sender_id = u.id AND d.receiver_id = ? AND d.is_read = 0) AS unread
           FROM dm_messages m
           JOIN users u ON u.id = CASE WHEN m.sender_id = ? THEN m.receiver_id ELSE m.sender_id END
           WHERE m.id IN (
             SELECT MAX(id) FROM dm_messages
             WHERE sender_id = ? OR receiver_id = ?
             GROUP BY CASE WHEN sender_id = ? THEN receiver_id ELSE sender_id END
           )
           ORDER BY m.id DESC LIMIT ?''',
        (user_id, user_id, user_id, user_id, user_id, user_id, limit))


def get_dm_thread(me_id, peer_id, limit=100, offset=0):
    return query_db(
        '''SELECT id, sender_id, receiver_id, content, is_read, created_at
           FROM dm_messages
           WHERE (sender_id = ? AND receiver_id = ?) OR (sender_id = ? AND receiver_id = ?)
           ORDER BY id DESC LIMIT ? OFFSET ?''',
        (me_id, peer_id, peer_id, me_id, limit, offset))


def mark_dm_read(me_id, peer_id):
    execute_db('UPDATE dm_messages SET is_read = 1 WHERE receiver_id = ? AND sender_id = ? AND is_read = 0',
               (me_id, peer_id))
    return True


def dm_unread_total(user_id):
    row = query_db('SELECT COUNT(*) AS c FROM dm_messages WHERE receiver_id = ? AND is_read = 0',
                   (user_id,), one=True)
    return row['c'] if row else 0


# ══════════════════════════════════════════════
# 通知偏好设置
# ══════════════════════════════════════════════

def get_user_settings(user_id):
    row = query_db('SELECT * FROM user_settings WHERE user_id = ?', (user_id,), one=True)
    if row:
        d = dict(row)
        d.setdefault('skin', 'glass')
        return d
    return {
        'notify_comment': 1, 'notify_like': 1, 'notify_follow': 1,
        'notify_system': 1, 'theme': 'auto', 'skin': 'glass'
    }


USER_SETTINGS_SKINS = ('glass', 'galgame', 'minimal', 'cyberpunk')


def save_user_settings(user_id, **kwargs):
    allowed = ('notify_comment', 'notify_like', 'notify_follow', 'notify_system', 'theme', 'ai_reply')
    fields = {k: int(v) for k, v in kwargs.items() if k in allowed and k != 'theme'}
    if 'theme' in kwargs:
        fields['theme'] = str(kwargs['theme'])
    # 皮肤（R9）：仅接受白名单值
    if kwargs.get('skin') in USER_SETTINGS_SKINS:
        fields['skin'] = kwargs['skin']
    if not fields:
        return False
    sets = ', '.join(f'{k} = ?' for k in fields)
    values = list(fields.values())
    # 占位符：INSERT(user_id + n 个字段) + ON CONFLICT SET(n 个字段)
    execute_db(
        f'''INSERT INTO user_settings (user_id, {', '.join(fields.keys())}) VALUES (?, {', '.join('?' for _ in fields)})
            ON CONFLICT(user_id) DO UPDATE SET {sets}, updated_at = CURRENT_TIMESTAMP''',
        [user_id] + values + values)
    return True


# ══════════════════════════════════════════════
# 角色关系图（world）：多人共同维护的关系图谱
# ══════════════════════════════════════════════

def _node_to_dict(r):
    return {
        'id': r['id'], 'name': r['name'], 'portrait': r['portrait'],
        'tagline': r['tagline'], 'color': r['color'], 'status': r['status'],
        'created_by': r['created_by'], 'created_at': r['created_at'],
    }


def get_first_public_station():
    """爆料帖未指定子站时，回退到首个公开子站。"""
    r = query_db('SELECT id FROM stations WHERE is_public = 1 ORDER BY id ASC LIMIT 1', one=True)
    return r['id'] if r else None


def create_character_node(name, portrait='', tagline='', color='', created_by=None):
    name = (name or '').strip()
    if not name:
        return None
    return execute_db(
        'INSERT INTO character_nodes (name, portrait, tagline, color, created_by) VALUES (?,?,?,?,?)',
        (name, portrait or '', tagline or '', color or '', created_by))


def update_character_node(node_id, **fields):
    allowed = ('name', 'portrait', 'tagline', 'color')
    upd = {k: v for k, v in fields.items() if k in allowed}
    if not upd:
        return False
    sets = ', '.join(f'{k} = ?' for k in upd)
    vals = list(upd.values()) + [node_id]
    execute_db(f'UPDATE character_nodes SET {sets} WHERE id = ?', vals)
    return True


def delete_character_node(node_id):
    execute_db('DELETE FROM character_relations WHERE from_id = ? OR to_id = ?', (node_id, node_id))
    execute_db('DELETE FROM character_nodes WHERE id = ?', (node_id,))


def get_character_node(node_id):
    r = query_db('SELECT * FROM character_nodes WHERE id = ? AND status = "active"', (node_id,), one=True)
    if not r:
        return None
    node = _node_to_dict(r)
    rels = query_db(
        'SELECT * FROM character_relations WHERE (from_id = ? OR to_id = ?) ORDER BY id ASC',
        (node_id, node_id))
    node['relations'] = [_rel_to_dict(x) for x in rels]
    return node


def get_character_graph():
    """全量图谱：active 节点 + 其关系，供 /world 页一次拉取渲染。"""
    nodes = [dict(r) for r in query_db('SELECT * FROM character_nodes WHERE status = "active" ORDER BY id ASC')]
    ids = [n['id'] for n in nodes]
    if ids:
        ph = ','.join('?' for _ in ids)
        raw = query_db(
            f'SELECT * FROM character_relations WHERE from_id IN ({ph}) AND to_id IN ({ph}) ORDER BY id ASC',
            ids + ids)
        rels = [_rel_to_dict(r) for r in raw]
    else:
        rels = []
    for n in nodes:
        n.pop('status')  # 前端不关心
    return {'nodes': nodes, 'relations': rels}


def _rel_to_dict(r):
    return {
        'id': r['id'], 'from_id': r['from_id'], 'to_id': r['to_id'],
        'label': r['label'], 'description': r['description'],
        'reciprocal': bool(r['reciprocal']), 'created_by': r['created_by'],
        'created_at': r['created_at'],
    }


def create_character_relation(from_id, to_id, label, description='', reciprocal=0, created_by=None):
    label = (label or '').strip()
    from_id = int(from_id or 0)
    to_id = int(to_id or 0)
    if not label or from_id <= 0 or to_id <= 0 or from_id == to_id:
        return None
    if not query_db('SELECT 1 FROM character_nodes WHERE id = ?', (from_id,), one=True) or \
       not query_db('SELECT 1 FROM character_nodes WHERE id = ?', (to_id,), one=True):
        return None
    return execute_db(
        'INSERT INTO character_relations (from_id, to_id, label, description, reciprocal, created_by) VALUES (?,?,?,?,?,?)',
        (from_id, to_id, label, description or '', 1 if reciprocal else 0, created_by))


def delete_character_relation(rel_id):
    execute_db('DELETE FROM character_relations WHERE id = ?', (rel_id,))


# ══════════════════════════════════════════════
# 话题/标签 + 热搜（R8）
# ══════════════════════════════════════════════

_MAX_TOPICS_PER_POST = 5
_MAX_TOPIC_LEN = 20


def normalize_topics(raw):
    """清洗话题列表：去 # 前缀/去重/截断；返回 list[str]（≤5 个，每个 ≤20 字符）。"""
    out, seen = [], set()
    for t in (raw or []):
        name = str(t).strip().lstrip('#').strip()
        if not name or len(name) > _MAX_TOPIC_LEN:
            continue
        key = name.lower()
        if key in seen:
            continue
        seen.add(key)
        out.append(name)
        if len(out) >= _MAX_TOPICS_PER_POST:
            break
    return out


def attach_post_topics(post_id, raw_topics, user_id=None):
    """幂等重挂：先清旧关联再写入（编辑话题=替换）。返回最终话题名列表。"""
    execute_db('DELETE FROM post_topics WHERE post_id = ?', (post_id,))
    names = normalize_topics(raw_topics)
    for name in names:
        row = query_db('SELECT id FROM topics WHERE name = ? COLLATE NOCASE', (name,), one=True)
        if row:
            tid = row['id']
            execute_db('UPDATE topics SET use_count = use_count + 1 WHERE id = ?', (tid,))
        else:
            tid = execute_db('INSERT INTO topics (name, created_by, use_count) VALUES (?, ?, 1)', (name, user_id))
        if tid:
            execute_db('INSERT OR IGNORE INTO post_topics (topic_id, post_id) VALUES (?, ?)', (tid, post_id))
    return names


def get_topics_for_posts(post_ids):
    """批量取话题 map：{post_id: [name, ...]}（供列表渲染，避免 N+1）。"""
    if not post_ids:
        return {}
    ph = ','.join('?' for _ in post_ids)
    rows = query_db(
        f'SELECT pt.post_id, t.name FROM post_topics pt JOIN topics t ON t.id = pt.topic_id '
        f'WHERE pt.post_id IN ({ph}) ORDER BY pt.rowid ASC', list(post_ids))
    out = {}
    for r in rows:
        out.setdefault(r['post_id'], []).append(r['name'])
    return out


def get_post_topics(post_id):
    rows = query_db(
        'SELECT t.name FROM post_topics pt JOIN topics t ON t.id = pt.topic_id WHERE pt.post_id = ? ORDER BY pt.rowid ASC',
        (post_id,))
    return [r['name'] for r in rows]


def get_trending_topics(limit=20):
    """热搜榜：按近 7 天被引用热度排序（recent 计数联查），回退总 use_count。"""
    return query_db('''
        SELECT t.id, t.name,
               COUNT(CASE WHEN pt.created_at >= datetime('now', '-7 days') THEN 1 END) AS recent,
               COUNT(*) AS total
        FROM topics t LEFT JOIN post_topics pt ON pt.topic_id = t.id
        GROUP BY t.id
        HAVING total > 0
        ORDER BY recent DESC, total DESC, t.id DESC
        LIMIT ?''', (limit,))


def get_topic_posts(name, limit=20, offset=0):
    """某话题下的已发布帖子（形状与 /api/posts 一致，脱敏交 handler）。"""
    return query_db('''
        SELECT p.*, u.username as author_name, u.avatar as author_avatar,
               s.name as station_name, s.icon as station_icon
        FROM posts p
        JOIN users u ON p.author_id = u.id
        JOIN stations s ON p.station_id = s.id
        JOIN post_topics pt ON pt.post_id = p.id
        JOIN topics t ON t.id = pt.topic_id
        WHERE p.is_deleted = 0 AND p.status = 'approved' AND t.name = ? COLLATE NOCASE
        ORDER BY p.is_pinned DESC, p.created_at DESC
        LIMIT ? OFFSET ?''', (name, limit, offset))


def get_all_topics_admin():
    return query_db('''
        SELECT t.id, t.name, t.use_count,
               (SELECT COUNT(*) FROM post_topics pt WHERE pt.topic_id = t.id) AS live_posts
        FROM topics t ORDER BY t.use_count DESC, t.id DESC''')


def delete_topic(tid):
    execute_db('DELETE FROM post_topics WHERE topic_id = ?', (tid,))
    execute_db('DELETE FROM topics WHERE id = ?', (tid,))


# ══════════════════════════════════════════════
# 任务系统（R8）：系统任务 / 管理员任务 / 积分悬赏
# ══════════════════════════════════════════════

TASK_KINDS = ('system', 'admin', 'bounty')


def grant_points(user_id, amount, description='', ref_type='', ref_id=0):
    user = query_db('SELECT points FROM users WHERE id = ?', (user_id,), one=True)
    new_points = (user['points'] or 0) + amount
    execute_db('UPDATE users SET points = ? WHERE id = ?', (new_points, user_id))
    return new_points


def spend_points(user_id, amount, description='', ref_type='', ref_id=0):
    """积分扣减（悬赏发布）。余额不足返回 False。"""
    user = query_db('SELECT points FROM users WHERE id = ?', (user_id,), one=True)
    if (user['points'] or 0) < amount:
        return False
    return grant_points(user_id, -amount, description, ref_type, ref_id) is not None


def create_task(kind, title, description='', reward_coins=0, reward_points=0,
                cost=0, max_claims=0, created_by=None):
    if kind not in TASK_KINDS:
        return None
    title = (title or '').strip()
    if not title or len(title) > 60:
        return None
    return execute_db(
        'INSERT INTO tasks (kind, title, description, reward_coins, reward_points, cost, max_claims, created_by) '
        'VALUES (?,?,?,?,?,?,?,?)',
        (kind, title, (description or '').strip()[:300], int(reward_coins or 0), int(reward_points or 0),
         int(cost or 0), int(max_claims or 0), created_by))


def close_task(task_id, user_id=None, is_admin=False):
    t = query_db('SELECT * FROM tasks WHERE id = ?', (task_id,), one=True)
    if not t or t['status'] != 'active':
        return False
    if not is_admin and t['created_by'] != user_id:
        return False
    execute_db("UPDATE tasks SET status = 'closed' WHERE id = ?", (task_id,))
    return True


def list_tasks(kind=None, user_id=None, limit=50):
    """active 任务 + 领取数 + 我的领取状态。"""
    where = "t.status = 'active'"
    args = []
    if kind in TASK_KINDS:
        where += ' AND t.kind = ?'
        args.append(kind)
    my_sel = ''
    if user_id:
        my_sel = ''',
               (SELECT status FROM task_claims mc WHERE mc.task_id = t.id AND mc.user_id = ?) AS my_status'''
        args_my = [user_id]
    else:
        args_my = []
        my_sel = ''',
               NULL AS my_status'''
    sql = f'''SELECT t.*,
               (SELECT COUNT(*) FROM task_claims tc WHERE tc.task_id = t.id) AS claim_count{my_sel}
            FROM tasks t WHERE {where} ORDER BY t.created_at DESC LIMIT ?'''
    # 占位符顺序：SELECT 里的 my_status 子查询 ? 在 WHERE 参数之前
    rows = query_db(sql, args_my + args + [limit])
    return rows


def get_task_by_id(task_id):
    return query_db('SELECT * FROM tasks WHERE id = ?', (task_id,), one=True)


def claim_task(task_id, user_id):
    t = get_task_by_id(task_id)
    if not t or t['status'] != 'active':
        return None, '任务不存在或已关闭'
    if t['max_claims'] and t['max_claims'] <= (query_db(
            'SELECT COUNT(*) AS n FROM task_claims WHERE task_id = ?', (task_id,), one=True)['n']):
        return None, '领取名额已满'
    if query_db('SELECT 1 FROM task_claims WHERE task_id = ? AND user_id = ?', (task_id, user_id), one=True):
        return None, '你已领取过该任务'
    execute_db('INSERT INTO task_claims (task_id, user_id) VALUES (?,?)', (task_id, user_id))
    return query_db('SELECT * FROM task_claims WHERE task_id = ? AND user_id = ?', (task_id, user_id), one=True), None


def grant_task_reward(task, user_id):
    msg = []
    if task['reward_coins']:
        add_coin_transaction(user_id, task['reward_coins'], 'task', f"任务奖励：{task['title']}", 'task', task['id'])
        msg.append(f"+{task['reward_coins']} 金币")
    if task['reward_points']:
        grant_points(user_id, task['reward_points'], ref_type='task', ref_id=task['id'])
        msg.append(f"+{task['reward_points']} 积分")
    return '，'.join(msg) or '完成'


def submit_task(task_id, user_id, proof=''):
    """交任务：system 类自动发奖；admin/bounty 进入 submitted 待审核。"""
    t = get_task_by_id(task_id)
    c = query_db('SELECT * FROM task_claims WHERE task_id = ? AND user_id = ?', (task_id, user_id), one=True)
    if not t or not c:
        return None, '请先领取任务'
    if c['status'] != 'in_progress':
        return None, '当前状态不可提交'
    if t['kind'] == 'system':
        execute_db("UPDATE task_claims SET status = 'completed', proof = ?, completed_at = CURRENT_TIMESTAMP WHERE id = ?",
                   (proof[:300], c['id']))
        reward = grant_task_reward(t, user_id)
        return {'status': 'completed', 'reward': reward}, None
    execute_db("UPDATE task_claims SET status = 'submitted', proof = ?, completed_at = CURRENT_TIMESTAMP WHERE id = ?",
               (proof[:300], c['id']))
    return {'status': 'submitted'}, None


def get_claim_by_id(claim_id):
    return query_db('SELECT * FROM task_claims WHERE id = ?', (claim_id,), one=True)


def review_task_claim(claim_id, action, reviewer):
    """bounty/管理员任务的完成审核：发布者本人或管理员。"""
    c = get_claim_by_id(claim_id)
    if not c:
        return None, '记录不存在'
    t = get_task_by_id(c['task_id'])
    is_admin = reviewer['role'] == 'admin'
    is_publisher = t['created_by'] == reviewer['id']
    if not (is_admin or is_publisher):
        return None, '无权审核该任务'
    if c['status'] != 'submitted':
        return None, '该提交不在待审核状态'
    if action == 'approve':
        execute_db("UPDATE task_claims SET status = 'completed' WHERE id = ?", (claim_id,))
        reward = grant_task_reward(t, c['user_id'])
        create_task_notification(c['user_id'], f"任务「{t['title']}」审核通过，获得 {reward}")
        return {'status': 'completed', 'reward': reward}, None
    if action == 'reject':
        # 驳回后允许重新提交（回到 in_progress）
        execute_db("UPDATE task_claims SET status = 'in_progress', completed_at = NULL WHERE id = ?", (claim_id,))
        create_task_notification(c['user_id'], f"任务「{t['title']}」提交被驳回，可修改后重新提交")
        return {'status': 'in_progress'}, None
    return None, '无效操作'


def create_task_notification(user_id, text):
    try:
        from app.models import create_notification
        create_notification(user_id, None, 'system', text, '/tasks')
    except Exception:
        pass


def get_my_claims(user_id, limit=50):
    return query_db('''
        SELECT tc.*, t.title, t.kind, t.reward_coins, t.reward_points, t.status AS task_status
        FROM task_claims tc JOIN tasks t ON t.id = tc.task_id
        WHERE tc.user_id = ? ORDER BY tc.claimed_at DESC LIMIT ?''', (user_id, limit))


def get_task_claims(task_id):
    return query_db('''
        SELECT tc.*, u.username, u.avatar
        FROM task_claims tc JOIN users u ON u.id = tc.user_id
        WHERE tc.task_id = ? ORDER BY tc.claimed_at DESC''', (task_id,))


def get_submitted_claims(limit=50):
    return query_db('''
        SELECT tc.*, t.title, t.kind, t.reward_coins, t.reward_points, t.created_by AS publisher_id,
               u.username
        FROM task_claims tc JOIN tasks t ON t.id = tc.task_id JOIN users u ON u.id = tc.user_id
        WHERE tc.status = 'submitted' ORDER BY tc.completed_at ASC LIMIT ?''', (limit,))


def get_all_tasks_admin(limit=100):
    return query_db('''
        SELECT t.*, (SELECT COUNT(*) FROM task_claims tc WHERE tc.task_id = t.id) AS claim_count
        FROM tasks t ORDER BY t.created_at DESC LIMIT ?''', (limit,))


# ══════════════════════════════════════════════
# 活动系统（R11）：发布 / 报名 / 打卡
# ══════════════════════════════════════════════

def create_event(title, description='', location='', event_date='', capacity=0,
                 reward_coins=0, created_by=None):
    title = (title or '').strip()
    if not title or len(title) > 80:
        return None
    import re as _re
    if not _re.match(r'^\d{4}-\d{2}-\d{2}$', event_date or ''):
        return None
    return execute_db(
        'INSERT INTO events (title, description, location, event_date, capacity, reward_coins, created_by) '
        'VALUES (?,?,?,?,?,?,?)',
        (title, (description or '').strip()[:500], (location or '').strip()[:120],
         event_date, max(int(capacity or 0), 0), max(int(reward_coins or 0), 0), created_by))


def list_events(user_id=None, limit=50):
    my_sel = ''',
               (SELECT 1 FROM event_registrations er WHERE er.event_id = e.id AND er.user_id = ?) AS my_registered,
               (SELECT er.checked_in FROM event_registrations er2 WHERE er2.event_id = e.id AND er2.user_id = ?) AS my_checked_in'''
    args = [user_id, user_id] if user_id else []
    my_sel = my_sel if user_id else ''',
               0 AS my_registered, 0 AS my_checked_in'''
    return query_db(f'''
        SELECT e.*, u.username AS creator_name,
               (SELECT COUNT(*) FROM event_registrations er WHERE er.event_id = e.id) AS registered_count{my_sel}
        FROM events e LEFT JOIN users u ON u.id = e.created_by
        WHERE e.status = 'active'
        ORDER BY e.event_date ASC LIMIT ?''', args + [limit])


def get_event(event_id):
    e = query_db('''
        SELECT e.*, u.username AS creator_name,
               (SELECT COUNT(*) FROM event_registrations er WHERE er.event_id = e.id) AS registered_count
        FROM events e LEFT JOIN users u ON u.id = e.created_by
        WHERE e.id = ?''', (event_id,), one=True)
    if not e:
        return None
    e['registrations'] = query_db('''
        SELECT er.user_id, er.checked_in, er.registered_at, u.username
        FROM event_registrations er JOIN users u ON u.id = er.user_id
        WHERE er.event_id = ? ORDER BY er.registered_at ASC''', (event_id,))
    return e


def register_event(event_id, user_id):
    e = query_db("SELECT * FROM events WHERE id = ? AND status = 'active'", (event_id,), one=True)
    if not e:
        return None, '活动不存在或已结束'
    if query_db('SELECT 1 FROM event_registrations WHERE event_id = ? AND user_id = ?', (event_id, user_id), one=True):
        return None, '你已报名该活动'
    if e['capacity']:
        n = query_db('SELECT COUNT(*) AS n FROM event_registrations WHERE event_id = ?', (event_id,), one=True)['n']
        if n >= e['capacity']:
            return None, '报名名额已满'
    execute_db('INSERT INTO event_registrations (event_id, user_id) VALUES (?,?)', (event_id, user_id))
    return True, None


def cancel_event_registration(event_id, user_id):
    row = query_db('SELECT checked_in FROM event_registrations WHERE event_id = ? AND user_id = ?',
                   (event_id, user_id), one=True)
    if not row:
        return None, '你尚未报名该活动'
    if row['checked_in']:
        return None, '已打卡，不可取消'
    execute_db('DELETE FROM event_registrations WHERE event_id = ? AND user_id = ?', (event_id, user_id))
    return True, None


def checkin_event(event_id, user_id):
    """活动打卡：需已报名、活动当天（或已过）、未重复打卡；发放奖励金币。"""
    e = query_db('SELECT * FROM events WHERE id = ?', (event_id,), one=True)
    row = query_db('SELECT * FROM event_registrations WHERE event_id = ? AND user_id = ?',
                   (event_id, user_id), one=True)
    if not e or not row:
        return None, '请先报名该活动'
    if row['checked_in']:
        return None, '已打卡过，请勿重复操作'
    import datetime as _dt
    today = _dt.date.today().isoformat()
    if e['event_date'] > today:
        return None, '活动尚未开始，无法打卡'
    execute_db('UPDATE event_registrations SET checked_in = 1, checked_in_at = CURRENT_TIMESTAMP '
               'WHERE event_id = ? AND user_id = ?', (event_id, user_id))
    reward = ''
    if e['reward_coins']:
        add_coin_transaction(user_id, e['reward_coins'], 'task', f"活动打卡：{e['title']}", 'event', event_id)
        reward = f"+{e['reward_coins']} 金币"
    return {'reward': reward or '打卡成功'}, None


def close_event(event_id):
    execute_db("UPDATE events SET status = 'closed' WHERE id = ?", (event_id,))
    return True


# ══════════════════════════════════════════════
# 暗阁（R11）：付费内容解锁 + 积分推流
# ══════════════════════════════════════════════

def has_unlocked_post(user_id, post_id):
    return bool(query_db('SELECT 1 FROM content_unlocks WHERE user_id = ? AND post_id = ?',
                         (user_id, post_id), one=True))


def unlock_post(user_id, post):
    """购买解锁：买家付积分，作者收款。"""
    pay = int(post.get('pay_points') or 0)
    if pay <= 0:
        return None, '该内容无需解锁'
    buyer = query_db('SELECT points FROM users WHERE id = ?', (user_id,), one=True)
    if (buyer['points'] or 0) < pay:
        return None, '积分不足，无法解锁'
    new_pts = (buyer['points'] or 0) - pay
    execute_db('UPDATE users SET points = ? WHERE id = ?', (new_pts, user_id))
    author = query_db('SELECT points FROM users WHERE id = ?', (post['author_id'],), one=True)
    execute_db('UPDATE users SET points = ? WHERE id = ?',
               ((author['points'] or 0) + pay, post['author_id']))
    execute_db('INSERT OR IGNORE INTO content_unlocks (user_id, post_id, creator_id, points) VALUES (?,?,?,?)',
               (user_id, post['id'], post['author_id'], pay))
    return {'paid': pay}, None


def boost_post(post_id, user_id, days):
    """推流：作者花积分把帖子在推荐流置前 N 天。10 积分/天，1-7 天。"""
    days = max(1, min(int(days or 0), 7))
    cost = days * 10
    if not spend_points(user_id, cost, ref_type='boost', ref_id=post_id):
        return None, '积分不足（推流 10 积分/天）'
    execute_db("UPDATE posts SET boosted_until = datetime('now', ?) WHERE id = ?",
               (f'+{days} days', post_id))
    return {'days': days, 'cost': cost}, None


# ══════════════════════════════════════════════
# 等级自动升级（R12）：经验奖励 + 规则引擎 + 身份组自动授予
# ══════════════════════════════════════════════

def get_level_rules():
    return query_db('SELECT level, exp_required, reward_coins FROM level_rules ORDER BY level ASC')


def upsert_level_rule(level, exp_required, reward_coins=0):
    if not level or level < 2 or level > 50:
        return False
    execute_db('''INSERT INTO level_rules (level, exp_required, reward_coins) VALUES (?,?,?)
                  ON CONFLICT(level) DO UPDATE SET exp_required = excluded.exp_required,
                  reward_coins = excluded.reward_coins''',
               (level, max(int(exp_required or 0), 0), max(int(reward_coins or 0), 0)))
    return True


def award_exp(user_id, amount):
    """加经验并按规则自动升级；升级发奖励金币 + 自动授予可达身份组。返回新等级或 None。"""
    user = query_db('SELECT level, exp FROM users WHERE id = ?', (user_id,), one=True)
    if not user:
        return None
    exp = (user['exp'] or 0) + max(int(amount or 0), 0)
    level = user['level'] or 1
    leveled_to = None
    while True:
        rule = query_db('SELECT exp_required, reward_coins FROM level_rules WHERE level = ?', (level + 1,), one=True)
        if not rule or exp < rule['exp_required']:
            break
        level += 1
        leveled_to = level
        if rule['reward_coins']:
            add_coin_transaction(user_id, rule['reward_coins'], 'task', f'升级 Lv{level} 奖励', 'level', level)
    execute_db('UPDATE users SET exp = ?, level = ? WHERE id = ?', (exp, level, user_id))
    if leveled_to:
        _auto_assign_groups(user_id, level)
    return leveled_to


def _auto_assign_groups(user_id, level):
    """升级后自动授予：auto_assign=1 且 min_level<=level 的最高身份组。"""
    grp = query_db('''
        SELECT name FROM identity_groups
        WHERE auto_assign = 1 AND min_level <= ? AND is_public = 1
        ORDER BY min_level DESC LIMIT 1''', (level,), one=True)
    if grp:
        execute_db('UPDATE users SET identity_group = ? WHERE id = ?', (grp['name'], user_id))


# ══════════════════════════════════════════════
# AI 生态 v1（R12）：可插拔 provider + 规则引擎实现
# ══════════════════════════════════════════════

AI_BOT_USERNAME = 'xiaozhi'


def _ai_bot_user():
    """AI 机器人账号（不存在则自动创建）。"""
    row = query_db('SELECT id, username FROM users WHERE username = ?', (AI_BOT_USERNAME,), one=True)
    if row:
        return row
    import bcrypt as _bcrypt
    pw_hash = _bcrypt.hashpw('ai-bot-no-login-9f2e'.encode(), _bcrypt.gensalt()).decode()
    uid = execute_db(
        'INSERT INTO users (username, email, password_hash, bio, role) VALUES (?,?,?,?,?)',
        (AI_BOT_USERNAME, 'xiaozhi@campuswall.local', pw_hash,
         '我是 AI 助手小智，由校园墙规则引擎驱动', 'user'))
    return query_db('SELECT id, username FROM users WHERE id = ?', (uid,), one=True)


def ai_moderate(title, content):
    """规则引擎式审核建议：返回 {action: approve|reject, reason}。"""
    text = f'{title}\n{content}'
    import re as _re
    contact = _re.findall(r'(1[3-9]\d{9}|微信\s*[:：]?\s*\w{4,}|QQ\s*[:：]?\s*\d{5,}|加我好友)', text)
    if contact:
        return {'action': 'reject', 'reason': '疑似留联系方式引流：' + '、'.join(contact[:2])}
    if len(content) < 10:
        return {'action': 'reject', 'reason': '内容过短（<10字）'}
    links = _re.findall(r'https?://\S+', content)
    if len(links) >= 3:
        return {'action': 'reject', 'reason': f'包含 {len(links)} 个外链，疑似广告'}
    if len(content) > 2000:
        return {'action': 'review', 'reason': '内容超长，建议人工复核'}
    return {'action': 'approve', 'reason': '规则引擎未命中风险模式'}


def ai_reply_text(text):
    """规则引擎式回复：关键词命中模板池，兜底通用回复。"""
    t = text or ''
    pool = [
        '收到！这就去帮你看看～',
        '好问题！建议去「表白墙」或对应子站再问问，说不定有同学知道。',
        '哈哈，这个话题有意思，支持一下！',
        '如果是二手交易相关，记得走平台流程更安全哦。',
        '听起来不错，祝顺利！',
    ]
    if any(k in t for k in ('?', '？', '怎么', '如何', '求助')):
        return pool[1]
    if any(k in t for k in ('卖', '出', '转让')):
        return pool[3]
    if any(k in t for k in ('谢谢', '感谢')):
        return '不客气～有问题随时找我！'
    return pool[2]


def ai_bot_interact():
    """AI 用户模拟互动：随机挑最近一篇帖点赞 + 发一条模板评论。"""
    import random as _random
    bot = _ai_bot_user()
    posts = query_db(
        "SELECT id, author_id, title, content FROM posts WHERE is_deleted = 0 AND status = 'approved' "
        'ORDER BY id DESC LIMIT 20')
    if not posts:
        return None
    post = _random.choice(posts)
    from app.models import toggle_like, create_comment, create_notification
    liked = toggle_like(bot['id'], 'post', post['id'])
    comment_id = execute_db(
        "INSERT INTO comments (post_id, author_id, content) VALUES (?,?,?)",
        (post['id'], bot['id'], ai_reply_text(post['content'] or post['title'])))
    execute_db('UPDATE posts SET comments_count = comments_count + 1 WHERE id = ?', (post['id'],))
    create_notification(
        post['author_id'], bot['id'], 'comment',
        f"AI 小智 评论了你的帖子「{post['title']}」", f'/post/{post["id"]}')
    return {'post_id': post['id'], 'liked': bool(liked), 'comment_id': comment_id}

# ══════════════════════════════════════════════
# 悬赏问答（R11 暗阁）：提问预扣积分，采纳放款
# ══════════════════════════════════════════════

def create_bounty_question(user_id, title, content='', bounty=0):
    title = (title or '').strip()
    if not title or len(title) > 80:
        return None
    bounty = max(int(bounty or 0), 0)
    if bounty > 0 and spend_points(user_id, bounty, ref_type='qna') is False:
        return 'insufficient'
    return execute_db(
        'INSERT INTO bounty_questions (user_id, title, content, bounty) VALUES (?,?,?,?)',
        (user_id, title, (content or '').strip()[:2000], bounty))


def list_bounty_questions(user_id=None, limit=50):
    my_sel = ''',
               (SELECT 1 FROM bounty_answers ba WHERE ba.question_id = q.id AND ba.user_id = ?) AS my_answered'''
    args = [user_id, user_id] if user_id else []
    if not user_id:
        my_sel = ''',
               0 AS my_answered'''
    sql = f"""SELECT q.*, u.username AS asker_name,
               (SELECT COUNT(*) FROM bounty_answers ba WHERE ba.question_id = q.id) AS answer_count{my_sel}
        FROM bounty_questions q JOIN users u ON u.id = q.user_id
        ORDER BY CASE q.status WHEN 'open' THEN 0 ELSE 1 END, q.created_at DESC LIMIT ?"""
    return query_db(sql, args + [limit])


def get_bounty_question(qid):
    q = query_db('''
        SELECT q.*, u.username AS asker_name FROM bounty_questions q
        JOIN users u ON u.id = q.user_id WHERE q.id = ?''', (qid,), one=True)
    if not q:
        return None
    q['answers'] = query_db('''
        SELECT a.*, u.username AS answerer_name, u.avatar AS answerer_avatar
        FROM bounty_answers a JOIN users u ON u.id = a.user_id
        WHERE a.question_id = ? ORDER BY a.is_accepted DESC, a.id ASC''', (qid,))
    return q


def create_bounty_answer(qid, user_id, content):
    content = (content or '').strip()
    if not content:
        return None, '回答不能为空'
    if len(content) > 2000:
        return None, '回答过长（≤2000字）'
    q = query_db("SELECT * FROM bounty_questions WHERE id = ? AND status = 'open'", (qid,), one=True)
    if not q:
        return None, '问题不存在或已关闭'
    if q['user_id'] == user_id:
        return None, '不能回答自己的提问'
    aid = execute_db(
        'INSERT INTO bounty_answers (question_id, user_id, content) VALUES (?,?,?)',
        (qid, user_id, content))
    return aid, None


def accept_bounty_answer(qid, answer_id, user_id):
    q = query_db('SELECT * FROM bounty_questions WHERE id = ?', (qid,), one=True)
    if not q:
        return None, '问题不存在'
    if q['user_id'] != user_id:
        return None, '仅提问者可采纳回答'
    if q['status'] != 'open':
        return None, '该问题已采纳/关闭'
    a = query_db('SELECT * FROM bounty_answers WHERE id = ? AND question_id = ?', (answer_id, qid), one=True)
    if not a:
        return None, '回答不存在'
    execute_db('UPDATE bounty_answers SET is_accepted = 1 WHERE id = ?', (answer_id,))
    execute_db("UPDATE bounty_questions SET status = 'answered', accepted_answer_id = ? WHERE id = ?",
               (answer_id, qid))
    reward = ''
    if q['bounty']:
        grant_points(a['user_id'], q['bounty'], ref_type='qna', ref_id=qid)
        reward = '+' + str(q['bounty']) + ' 积分'
        try:
            from app.models import create_notification
            create_notification(a['user_id'], user_id, 'system',
                                '你的回答被采纳，获得 ' + reward, '/qna')
        except Exception:
            pass
    return {'reward': reward or '已采纳'}, None


def close_bounty_question(qid, user_id):
    q = query_db('SELECT * FROM bounty_questions WHERE id = ?', (qid,), one=True)
    if not q:
        return None, '问题不存在'
    if q['user_id'] != user_id:
        return None, '仅提问者可关闭'
    if q['status'] != 'open':
        return None, '该问题已关闭'
    execute_db("UPDATE bounty_questions SET status = 'closed' WHERE id = ?", (qid,))
    if q['bounty']:
        grant_points(user_id, q['bounty'], ref_type='qna', ref_id=qid)
    return {'refunded': q['bounty']}, None