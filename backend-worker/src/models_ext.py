"""扩展数据模型 —— 移植自 app/models_ext.py（身份组/签到/虚拟币/商城/恋爱情报/树洞/交易/收藏/举报/公告/看板娘/管理后台）"""
import json
import random
from datetime import datetime, timedelta

import db
from db import IntegrityError


# ── 身份组 ──

async def get_identity_groups():
    return await db.query('SELECT * FROM identity_groups ORDER BY sort_order ASC')


async def get_identity_group_by_id(gid):
    return await db.query('SELECT * FROM identity_groups WHERE id = ?', (gid,), one=True)


async def create_identity_group(name, icon, color, description, permissions, min_level, is_default):
    return await db.execute(
        'INSERT INTO identity_groups (name, icon, color, description, permissions, min_level, is_default) '
        'VALUES (?,?,?,?,?,?,?)',
        (name, icon, color, description, json.dumps(permissions), min_level, is_default))


async def assign_user_group(user_id, group_name):
    await db.execute('UPDATE users SET identity_group = ? WHERE id = ?', (group_name, user_id))


async def get_user_group(user):
    group_name = (user or {}).get('identity_group', '')
    if not group_name:
        return None
    return await db.query('SELECT * FROM identity_groups WHERE name = ?', (group_name,), one=True)


# ── 签到 ──

async def do_checkin(user_id):
    today = datetime.now().strftime('%Y-%m-%d')
    existing = await db.query('SELECT * FROM checkins WHERE user_id = ? AND checkin_date = ?',
                              (user_id, today), one=True)
    if existing:
        return None

    yesterday = (datetime.now() - timedelta(days=1)).strftime('%Y-%m-%d')
    last = await db.query('SELECT * FROM checkins WHERE user_id = ? AND checkin_date = ?',
                          (user_id, yesterday), one=True)
    streak = (last['streak'] + 1) if last else 1

    base_coins = 10
    streak_bonus = min(streak - 1, 7) * 5
    total_coins = base_coins + streak_bonus
    points_earned = streak * 2

    cid = await db.execute(
        'INSERT INTO checkins (user_id, checkin_date, streak, coins_earned, points_earned) VALUES (?,?,?,?,?)',
        (user_id, today, streak, total_coins, points_earned))

    user = await db.query('SELECT coins, points, checkin_streak FROM users WHERE id = ?', (user_id,), one=True)
    new_coins = (user['coins'] or 0) + total_coins
    new_points = (user['points'] or 0) + points_earned
    await db.execute('UPDATE users SET coins = ?, points = ?, checkin_streak = ?, last_checkin = ? WHERE id = ?',
                     (new_coins, new_points, streak, today, user_id))

    await add_coin_transaction(user_id, total_coins, 'checkin', f'每日签到 (连签{streak}天)', 'checkin', cid)

    return {'streak': streak, 'coins_earned': total_coins,
            'points_earned': points_earned, 'total_coins': new_coins, 'total_points': new_points}


async def get_checkin_history(user_id, limit=30):
    return await db.query('SELECT * FROM checkins WHERE user_id = ? ORDER BY checkin_date DESC LIMIT ?',
                          (user_id, limit))


async def get_checkin_today(user_id):
    today = datetime.now().strftime('%Y-%m-%d')
    return await db.query('SELECT * FROM checkins WHERE user_id = ? AND checkin_date = ?', (user_id, today), one=True)


# ── 虚拟币 ──

async def add_coin_transaction(user_id, amount, tx_type, description='', ref_type='', ref_id=0):
    user = await db.query('SELECT coins FROM users WHERE id = ?', (user_id,), one=True)
    new_balance = (user['coins'] or 0) + amount
    await db.execute('UPDATE users SET coins = ? WHERE id = ?', (new_balance, user_id))
    await db.execute(
        'INSERT INTO coin_transactions (user_id, amount, balance_after, type, description, ref_type, ref_id) '
        'VALUES (?,?,?,?,?,?,?)',
        (user_id, amount, new_balance, tx_type, description, ref_type, ref_id))
    return new_balance


async def get_coin_transactions(user_id, limit=50):
    return await db.query('SELECT * FROM coin_transactions WHERE user_id = ? ORDER BY created_at DESC LIMIT ?',
                          (user_id, limit))


async def spend_coins(user_id, amount, description='', ref_type='', ref_id=0):
    user = await db.query('SELECT coins FROM users WHERE id = ?', (user_id,), one=True)
    if (user['coins'] or 0) < amount:
        return False
    return await add_coin_transaction(user_id, -amount, 'spend', description, ref_type, ref_id)


# ── 积分商城 ──

async def get_shop_items(active_only=True):
    sql = 'SELECT * FROM shop_items'
    if active_only:
        sql += ' WHERE is_active = 1'
    sql += ' ORDER BY sort_order ASC, id ASC'
    return await db.query(sql)


async def get_shop_item_by_id(item_id):
    return await db.query('SELECT * FROM shop_items WHERE id = ?', (item_id,), one=True)


async def buy_shop_item(user_id, item_id, quantity=1):
    item = await get_shop_item_by_id(item_id)
    if not item or not item['is_active']:
        return None, '商品不存在或已下架'
    if item['stock'] == 0:
        return None, '库存不足'

    total_coins = item['price_coins'] * quantity
    total_points = item['price_points'] * quantity
    user = await db.query('SELECT coins, points FROM users WHERE id = ?', (user_id,), one=True)

    if total_coins > 0 and (user['coins'] or 0) < total_coins:
        return None, '金币不足'
    if total_points > 0 and (user['points'] or 0) < total_points:
        return None, '积分不足'

    if total_coins > 0:
        await spend_coins(user_id, total_coins, f'购买 {item["name"]}', 'shop', item_id)
    if total_points > 0:
        new_pts = (user['points'] or 0) - total_points
        await db.execute('UPDATE users SET points = ? WHERE id = ?', (new_pts, user_id))

    if item['stock'] > 0:
        await db.execute('UPDATE shop_items SET stock = stock - ? WHERE id = ?', (quantity, item_id))

    order_id = await db.execute(
        'INSERT INTO shop_orders (user_id, item_id, quantity, total_coins, total_points) VALUES (?,?,?,?,?)',
        (user_id, item_id, quantity, total_coins, total_points))
    return {'order_id': order_id, 'item': _decode_shop_item(item)}, None


def _decode_shop_item(item):
    if item and 'tags' in item and isinstance(item.get('tags'), str):
        try:
            item = dict(item)
            item['tags'] = json.loads(item['tags'])
        except (ValueError, TypeError):
            pass
    return item


async def use_shop_item(user_id, item_id, extra=None):
    extra = extra or {}
    order = await db.query(
        '''SELECT o.*, i.item_type, i.name as item_name, i.item_data
           FROM shop_orders o JOIN shop_items i ON o.item_id = i.id
           WHERE o.user_id = ? AND o.item_id = ? AND o.status = 'completed'
           ORDER BY o.id ASC LIMIT 1''', (user_id, item_id), one=True)
    if not order:
        return None, '没有可用库存，请先购买该道具'
    item_type = order['item_type']

    if item_type in ('rename_card', 'rename'):
        new_name = (extra.get('new_name') or '').strip()
        if not new_name or len(new_name) < 2 or len(new_name) > 30:
            return None, '用户名需为2-30个字符'
        from models import get_user_by_username
        existing = await get_user_by_username(new_name)
        if existing and existing['id'] != user_id:
            return None, '该用户名已被使用'
        await db.execute('UPDATE users SET username = ? WHERE id = ?', (new_name, user_id))
    elif item_type in ('pin_card', 'pin'):
        post_id = extra.get('post_id')
        if not post_id:
            return None, '缺少帖子ID'
        await db.execute('UPDATE posts SET is_pinned = 1 WHERE id = ? AND author_id = ?',
                         (int(post_id), user_id))
    elif item_type in ('anonymity_card', 'anonymous'):
        await db.execute(
            'INSERT INTO coin_transactions (user_id, amount, balance_after, type, description, ref_type, ref_id) '
            'VALUES (?,?,?,?,?,?,?)',
            (user_id, 0, 0, 'card_used', '匿名卡已激活（可匿名发帖3次）', 'shop_item', item_id))
    elif item_type in ('title', 'rainbow_name'):
        new_title = (extra.get('title') or '').strip()
        if item_type == 'rainbow_name':
            new_title = (new_title or '彩虹用户')
        await db.execute('UPDATE users SET title = ? WHERE id = ?', (new_title, user_id))
    else:
        return None, '该道具暂不支持使用'

    await db.execute('UPDATE shop_orders SET status = ? WHERE id = ?', ('used', order['id']))
    return {'message': f'已使用「{order["item_name"]}」'}, None


# ── 恋爱情报专区 ──

async def get_romance_profiles(limit=50, offset=0, gender=None):
    sql = 'SELECT rp.*, u.username, u.avatar FROM romance_profiles rp JOIN users u ON rp.user_id = u.id WHERE rp.is_visible = 1'
    args = []
    if gender:
        sql += ' AND rp.gender = ?'
        args.append(gender)
    sql += ' ORDER BY rp.likes_received DESC LIMIT ? OFFSET ?'
    args += [limit, offset]
    return await db.query(sql, args)


async def get_romance_profile(user_id):
    return await db.query(
        'SELECT rp.*, u.username, u.avatar FROM romance_profiles rp JOIN users u ON rp.user_id = u.id '
        'WHERE rp.user_id = ?', (user_id,), one=True)


async def create_romance_profile(user_id, **kwargs):
    existing = await get_romance_profile(user_id)
    if existing:
        fields = {k: v for k, v in kwargs.items()
                  if k in ('nickname', 'gender', 'age', 'department', 'hobbies', 'looking_for', 'photo', 'is_visible')}
        if not fields:
            return False
        if isinstance(fields.get('hobbies'), (list, dict)):
            fields['hobbies'] = json.dumps(fields['hobbies'], ensure_ascii=False)
        if 'is_visible' in fields:
            fields['is_visible'] = 1 if fields['is_visible'] else 0
        sets = ', '.join(f'{k} = ?' for k in fields)
        vals = list(fields.values()) + [user_id]
        await db.execute(f'UPDATE romance_profiles SET {sets} WHERE user_id = ?', vals)
        return True
    else:
        await db.execute(
            'INSERT INTO romance_profiles (user_id, nickname, gender, age, department, hobbies, looking_for, photo) '
            'VALUES (?,?,?,?,?,?,?,?)',
            (user_id, kwargs.get('nickname', ''), kwargs.get('gender', ''),
             kwargs.get('age', 0), kwargs.get('department', ''),
             json.dumps(kwargs.get('hobbies', []), ensure_ascii=False),
             kwargs.get('looking_for', ''), kwargs.get('photo', '')))
        return True


async def create_romance_link(from_uid, to_uid, link_type='crush', description='', is_anonymous=0):
    return await db.execute(
        'INSERT INTO romance_links (from_user_id, to_user_id, link_type, description, is_anonymous) VALUES (?,?,?,?,?)',
        (from_uid, to_uid, link_type, description, is_anonymous))


async def get_romance_links(user_id, direction='to'):
    if direction == 'to':
        return await db.query(
            '''SELECT rl.*, u.username as from_name, u.avatar as from_avatar
               FROM romance_links rl LEFT JOIN users u ON rl.from_user_id = u.id
               WHERE rl.to_user_id = ? ORDER BY rl.created_at DESC''', (user_id,))
    else:
        return await db.query(
            '''SELECT rl.*, u.username as to_name, u.avatar as to_avatar
               FROM romance_links rl LEFT JOIN users u ON rl.to_user_id = u.id
               WHERE rl.from_user_id = ? ORDER BY rl.created_at DESC''', (user_id,))


async def create_romance_task(creator_id, title, description='', task_type='matchmake', target_user_id=0, reward_coins=0):
    return await db.execute(
        'INSERT INTO romance_tasks (creator_id, title, description, task_type, target_user_id, reward_coins) '
        'VALUES (?,?,?,?,?,?)',
        (creator_id, title, description, task_type, target_user_id, reward_coins))


async def get_romance_tasks(status='open', limit=50):
    return await db.query(
        '''SELECT rt.*, u.username as creator_name
           FROM romance_tasks rt JOIN users u ON rt.creator_id = u.id
           WHERE rt.status = ? ORDER BY rt.created_at DESC LIMIT ?''', (status, limit))


# ── 爆料/树洞 ──

async def create_gossip(content, station_id=None, images=None, is_anonymous=1):
    return await db.execute(
        'INSERT INTO gossip (content, station_id, images, is_anonymous) VALUES (?,?,?,?)',
        (content, station_id, json.dumps(images or [], ensure_ascii=False), is_anonymous))


async def get_gossip(station_id=None, limit=50, offset=0, sort='newest'):
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
    return await db.query(sql, args)


async def get_gossip_by_id(gid):
    return await db.query('SELECT * FROM gossip WHERE id = ? AND is_deleted = 0', (gid,), one=True)


async def toggle_gossip_like(gid, user_id=None):
    if not user_id:
        await db.execute('UPDATE gossip SET likes_count = likes_count + 1 WHERE id = ?', (gid,))
        return {'liked': True}
    row = await db.query('SELECT id FROM likes WHERE user_id = ? AND target_type = ? AND target_id = ?',
                         (user_id, 'gossip', gid), one=True)
    if row:
        await db.execute('DELETE FROM likes WHERE id = ?', (row['id'],))
        await db.execute('UPDATE gossip SET likes_count = MAX(0, likes_count - 1) WHERE id = ?', (gid,))
        return {'liked': False}
    await db.execute('INSERT INTO likes (user_id, target_type, target_id) VALUES (?, ?, ?)',
                     (user_id, 'gossip', gid))
    await db.execute('UPDATE gossip SET likes_count = likes_count + 1 WHERE id = ?', (gid,))
    return {'liked': True}


async def create_gossip_comment(gid, content, author_name='匿名'):
    cid = await db.execute('INSERT INTO gossip_comments (gossip_id, content, author_name) VALUES (?,?,?)',
                           (gid, content, author_name))
    await db.execute('UPDATE gossip SET comments_count = comments_count + 1 WHERE id = ?', (gid,))
    return cid


async def get_gossip_comments(gid, limit=100):
    return await db.query(
        'SELECT * FROM gossip_comments WHERE gossip_id = ? AND is_deleted = 0 ORDER BY created_at ASC LIMIT ?',
        (gid, limit))


# ── 交易帖 ──

async def create_trade_post(post_id, user_id, price, original_price=0, condition='good', category='', contact=''):
    return await db.execute(
        'INSERT INTO trade_posts (post_id, user_id, price, original_price, condition, category, contact) '
        'VALUES (?,?,?,?,?,?,?)',
        (post_id, user_id, price, original_price, condition, category, contact))


async def get_trade_posts(category=None, status='available', limit=50, offset=0, keyword=None):
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
    return await db.query(sql, args)


async def update_trade_status(trade_id, status):
    await db.execute('UPDATE trade_posts SET status = ? WHERE id = ?', (status, trade_id))


# ── 看板娘 ──

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
    return random.choice(KANBAN_GREETINGS + KANBAN_TIPS)


# ── 推流算法 ──

async def get_recommended_posts(user_id=None, limit=20, offset=0):
    return await db.query('''
        SELECT p.*, u.username as author_name, u.avatar as author_avatar,
               s.name as station_name, s.icon as station_icon,
               (p.likes_count * 3 + p.comments_count * 5 + p.views * 0.1) as hot_score
        FROM posts p
        JOIN users u ON p.author_id = u.id
        LEFT JOIN stations s ON p.station_id = s.id
        WHERE p.is_deleted = 0
        ORDER BY
            p.is_pinned DESC,
            (p.likes_count * 3 + p.comments_count * 5 + p.views * 0.1) * 0.6
            + (JULIANDAY('now') - JULIANDAY(p.created_at)) * (-0.4)
            DESC
        LIMIT ? OFFSET ?
    ''', (limit, offset))


async def get_user_interest_stations(user_id, limit=5):
    return await db.query('''
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


# ── 搜索算法 ──

async def smart_search(keyword, user_id=None):
    results = {'stations': [], 'posts': [], 'users': []}

    results['stations'] = await db.query(
        '''SELECT *, (user_count * 2 + post_count) as relevance
           FROM stations WHERE name LIKE ? OR description LIKE ?
           ORDER BY relevance DESC LIMIT 10''', (f'%{keyword}%', f'%{keyword}%'))

    results['posts'] = await db.query(
        '''SELECT p.*, u.username as author_name, u.avatar as author_avatar,
                  s.name as station_name, s.icon as station_icon
           FROM posts p JOIN users u ON p.author_id = u.id LEFT JOIN stations s ON p.station_id = s.id
           WHERE p.is_deleted = 0 AND (p.title LIKE ? OR p.content LIKE ?)
           ORDER BY p.likes_count DESC LIMIT 15''', (f'%{keyword}%', f'%{keyword}%'))

    results['users'] = await db.query(
        'SELECT id, username, avatar, bio FROM users WHERE username LIKE ? OR bio LIKE ? LIMIT 10',
        (f'%{keyword}%', f'%{keyword}%'))

    return results


# ── 管理后台 ──

async def get_admin_stats():
    return {
        'users': (await db.query('SELECT COUNT(*) as c FROM users', one=True))['c'],
        'stations': (await db.query('SELECT COUNT(*) as c FROM stations', one=True))['c'],
        'posts': (await db.query('SELECT COUNT(*) as c FROM posts WHERE is_deleted = 0', one=True))['c'],
        'comments': (await db.query('SELECT COUNT(*) as c FROM comments WHERE is_deleted = 0', one=True))['c'],
        'trend': await get_admin_stats_series(7),
        'coins': (await db.query('SELECT COALESCE(SUM(coins),0) as c FROM users', one=True))['c'],
        # 前端仪表盘消费的今日新增字段（date(created_at) 与 CURRENT_TIMESTAMP 同为 UTC）
        'today_posts': (await db.query(
            "SELECT COUNT(*) as c FROM posts WHERE date(created_at) = date('now') AND is_deleted = 0",
            one=True))['c'],
        'today_users': (await db.query(
            "SELECT COUNT(*) as c FROM users WHERE date(created_at) = date('now')",
            one=True))['c'],
    }


async def get_admin_stats_series(days=7):
    days = max(1, min(30, int(days)))
    labels, users, posts, comments = [], [], [], []
    for i in range(days - 1, -1, -1):
        d = (datetime.now() - timedelta(days=i)).strftime('%Y-%m-%d')
        labels.append(d[5:])
        users.append((await db.query("SELECT COUNT(*) as c FROM users WHERE date(created_at) = ?", (d,), one=True))['c'])
        posts.append((await db.query("SELECT COUNT(*) as c FROM posts WHERE date(created_at) = ? AND is_deleted = 0",
                                     (d,), one=True))['c'])
        comments.append((await db.query("SELECT COUNT(*) as c FROM comments WHERE date(created_at) = ? AND is_deleted = 0",
                                        (d,), one=True))['c'])
    return {'labels': labels, 'users': users, 'posts': posts, 'comments': comments}


async def get_all_users_admin(limit=100, offset=0):
    return await db.query(
        'SELECT id, username, email, avatar, role, coins, points, level, identity_group, created_at '
        'FROM users ORDER BY id DESC LIMIT ? OFFSET ?', (limit, offset))


async def update_user_admin(uid, **kwargs):
    allowed = {'role', 'coins', 'points', 'level', 'identity_group', 'username', 'bio'}
    fields = {k: v for k, v in kwargs.items() if k in allowed}
    if not fields:
        return False
    sets = ', '.join(f'{k} = ?' for k in fields)
    vals = list(fields.values()) + [uid]
    await db.execute(f'UPDATE users SET {sets}, updated_at = CURRENT_TIMESTAMP WHERE id = ?', vals)
    return True


async def admin_log(admin_id, action, target_type='', target_id=0, detail=''):
    await db.execute('INSERT INTO admin_log (admin_id, action, target_type, target_id, detail) VALUES (?,?,?,?,?)',
                     (admin_id, action, target_type, target_id, detail))


async def get_admin_logs(limit=100):
    return await db.query(
        '''SELECT al.*, u.username as admin_name
           FROM admin_log al LEFT JOIN users u ON al.admin_id = u.id
           ORDER BY al.created_at DESC LIMIT ?''', (limit,))


# ── 站内公告 ──

async def get_announcements(active_only=False, limit=20):
    sql = '''SELECT a.*, u.username as creator_name
             FROM site_announcements a LEFT JOIN users u ON a.created_by = u.id'''
    args = []
    if active_only:
        sql += ' WHERE a.is_active = 1'
    sql += ' ORDER BY a.created_at DESC LIMIT ?'
    args.append(limit)
    return await db.query(sql, args)


async def create_announcement(title, content, admin_id):
    await db.execute('INSERT INTO site_announcements (title, content, created_by) VALUES (?,?,?)',
                     (title, content, admin_id))


async def update_announcement(aid, title=None, content=None):
    if title is not None:
        await db.execute('UPDATE site_announcements SET title = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?',
                         (title, aid))
    if content is not None:
        await db.execute('UPDATE site_announcements SET content = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?',
                         (content, aid))


async def toggle_announcement(aid, is_active):
    await db.execute('UPDATE site_announcements SET is_active = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?',
                     (1 if is_active else 0, aid))


async def delete_announcement(aid):
    await db.execute('DELETE FROM site_announcements WHERE id = ?', (aid,))


# ── 收藏 ──

async def toggle_favorite(user_id, target_type, target_id):
    row = await db.query('SELECT id FROM favorites WHERE user_id = ? AND target_type = ? AND target_id = ?',
                         (user_id, target_type, target_id), one=True)
    if row:
        await db.execute('DELETE FROM favorites WHERE id = ?', (row['id'],))
        return False
    await db.execute('INSERT INTO favorites (user_id, target_type, target_id) VALUES (?,?,?)',
                     (user_id, target_type, target_id))
    return True


async def is_favorited(user_id, target_type, target_id):
    return (await db.query('SELECT 1 FROM favorites WHERE user_id = ? AND target_type = ? AND target_id = ?',
                           (user_id, target_type, target_id), one=True)) is not None


async def get_favorites(user_id, target_type='post', limit=50, offset=0):
    if target_type == 'post':
        return await db.query(
            '''SELECT f.target_id, f.created_at as favorited_at, p.*, u.username as author_name,
                      u.avatar as author_avatar, u.identity_group as author_identity_group,
                      s.name as station_name, s.icon as station_icon
               FROM favorites f
               JOIN posts p ON f.target_id = p.id AND p.is_deleted = 0
               LEFT JOIN users u ON p.author_id = u.id
               LEFT JOIN stations s ON p.station_id = s.id
               WHERE f.user_id = ? AND f.target_type = 'post'
               ORDER BY f.created_at DESC LIMIT ? OFFSET ?''', (user_id, limit, offset))
    if target_type == 'station':
        return await db.query(
            '''SELECT f.target_id, f.created_at as favorited_at, s.*
               FROM favorites f JOIN stations s ON f.target_id = s.id
               WHERE f.user_id = ? AND f.target_type = 'station'
               ORDER BY f.created_at DESC LIMIT ? OFFSET ?''', (user_id, limit, offset))
    return []


# ── 举报 ──

async def create_report(reporter_id, target_type, target_id, reason='', detail=''):
    existing = await db.query(
        'SELECT id FROM reports WHERE reporter_id = ? AND target_type = ? AND target_id = ? AND status = ?',
        (reporter_id, target_type, target_id, 'pending'), one=True)
    if existing:
        return None
    return await db.execute(
        'INSERT INTO reports (reporter_id, target_type, target_id, reason, detail) VALUES (?,?,?,?,?)',
        (reporter_id, target_type, target_id, reason, detail))


async def get_reports(status='pending', limit=100):
    return await db.query(
        '''SELECT r.*, u.username as reporter_name, t.username as handler_name
           FROM reports r
           LEFT JOIN users u ON r.reporter_id = u.id
           LEFT JOIN users t ON r.handler_id = t.id
           WHERE ? = 'all' OR r.status = ?
           ORDER BY r.created_at DESC LIMIT ?''', (status, status, limit))


async def handle_report(report_id, handler_id, status, note=''):
    await db.execute('UPDATE reports SET status = ?, handler_id = ?, handled_at = CURRENT_TIMESTAMP WHERE id = ?',
                     (status, handler_id, report_id))
    return True


# ── 通知偏好设置 ──

async def get_user_settings(user_id):
    row = await db.query('SELECT * FROM user_settings WHERE user_id = ?', (user_id,), one=True)
    if row:
        return dict(row)
    return {'notify_comment': 1, 'notify_like': 1, 'notify_follow': 1, 'notify_system': 1, 'theme': 'auto'}


async def save_user_settings(user_id, **kwargs):
    allowed = ('notify_comment', 'notify_like', 'notify_follow', 'notify_system', 'theme')
    fields = {k: int(v) for k, v in kwargs.items() if k in allowed and k != 'theme'}
    if 'theme' in kwargs:
        fields['theme'] = kwargs['theme']
    if not fields:
        return False
    cols = ', '.join(fields.keys())
    placeholders = ', '.join('?' for _ in fields)
    sets = ', '.join(f'{k} = ?' for k in fields)
    vals = list(fields.values())
    await db.execute(
        f'INSERT INTO user_settings (user_id, {cols}) VALUES (?, {placeholders}) '
        f'ON CONFLICT(user_id) DO UPDATE SET {sets}, updated_at = CURRENT_TIMESTAMP',
        [user_id] + vals + vals)
    return True