"""
扩展 API 路由 — 身份组、签到、积分商城、恋爱情报、爆料、交易、看板娘、搜索、推流、管理后台
"""
import json
from flask import Blueprint, request, jsonify, g
from app.utils.auth import token_required, optional_auth
from app.utils.limiter import limiter
from app.utils.sensitive import scan_text
from app.models import is_liked
from app.models_ext import (
    get_identity_groups, create_identity_group, assign_user_group, get_user_group,
    do_checkin, get_checkin_history, get_checkin_today,
    get_coin_transactions,
    get_shop_items, get_shop_item_by_id, buy_shop_item, use_shop_item,
    get_romance_profiles, get_romance_profile, create_romance_profile,
    create_romance_link, get_romance_links, create_romance_task, get_romance_tasks,
    create_gossip, get_gossip, get_gossip_by_id, toggle_gossip_like,
    create_gossip_comment, get_gossip_comments,
    create_trade_post, get_trade_posts, update_trade_status,
    get_kanban_message,
    get_recommended_posts, get_user_interest_stations,
    smart_search,
    get_admin_stats, get_admin_stats_series, get_all_users_admin, update_user_admin, admin_log, get_admin_logs,
    toggle_favorite, is_favorited, get_favorites,
    create_report, get_reports, handle_report,
    get_user_settings, save_user_settings,
    get_announcements, create_announcement, update_announcement, toggle_announcement, delete_announcement,
    get_character_graph, create_character_node, update_character_node, delete_character_node,
    get_character_node, create_character_relation, delete_character_relation,
    get_site_config, set_site_config, get_ad_config,
)
from app.models import get_user_by_id, query_db, execute_db, get_posts, update_post_status, get_post_by_id, create_notification
from app.models import shape_post, is_liked_batch

# ══════════════════════════════════════════════
# 身份组
# ══════════════════════════════════════════════

identity_bp = Blueprint('identity', __name__)

@identity_bp.route('/groups', methods=['GET'])
def list_groups():
    return jsonify(get_identity_groups())

@identity_bp.route('/groups', methods=['POST'])
@token_required
def create_group():
    if g.current_user['role'] != 'admin':
        return jsonify({'error': '需要管理员权限'}), 403
    data = request.get_json(silent=True) or {}
    name = (data.get('name') or '').strip()
    if not name:
        return jsonify({'error': '名称不能为空'}), 400
    gid = create_identity_group(
        name, data.get('icon', 'tag'), data.get('color', '#fb6f92'),
        data.get('description', ''), data.get('permissions', []),
        data.get('min_level', 1), data.get('is_default', 0)
    )
    return jsonify({'message': '创建成功', 'id': gid}), 201

@identity_bp.route('/assign', methods=['POST'])
@token_required
def assign():
    if g.current_user['role'] != 'admin':
        return jsonify({'error': '需要管理员权限'}), 403
    data = request.get_json(silent=True) or {}
    uid = data.get('user_id')
    group = data.get('group', '')
    if not uid:
        return jsonify({'error': '缺少用户ID'}), 400
    assign_user_group(uid, group)
    admin_log(g.current_user['id'], 'assign_group', 'user', uid, f'设置身份组: {group}')
    return jsonify({'message': '设置成功'})

@identity_bp.route('/my', methods=['GET'])
@token_required
def my_group():
    group = get_user_group(g.current_user)
    return jsonify(group or {})


# ══════════════════════════════════════════════
# 签到
# ══════════════════════════════════════════════

checkin_bp = Blueprint('checkin', __name__)

@checkin_bp.route('', methods=['POST'])
@limiter.limit('10/minute')
@token_required
def checkin():
    result = do_checkin(g.current_user['id'])
    if not result:
        return jsonify({'message': '今天已经签到过了', 'already': True})
    return jsonify({'message': '签到成功！', **result})

@checkin_bp.route('/status', methods=['GET'])
@token_required
def status():
    today = get_checkin_today(g.current_user['id'])
    history = get_checkin_history(g.current_user['id'], 30)
    user = get_user_by_id(g.current_user['id'])
    return jsonify({
        'checked_in_today': today is not None,
        'streak': user.get('checkin_streak', 0),
        'coins': user.get('coins', 0),
        'points': user.get('points', 0),
        'history': history
    })


# ══════════════════════════════════════════════
# 积分商城
# ══════════════════════════════════════════════

shop_bp = Blueprint('shop', __name__)

@shop_bp.route('/items', methods=['GET'])
def list_items():
    return jsonify(get_shop_items())

@shop_bp.route('/items/<int:item_id>', methods=['GET'])
def get_item(item_id):
    item = get_shop_item_by_id(item_id)
    if not item:
        return jsonify({'error': '商品不存在'}), 404
    return jsonify(item)

@shop_bp.route('/buy', methods=['POST'])
@token_required
def buy():
    data = request.get_json(silent=True) or {}
    item_id = data.get('item_id')
    quantity = data.get('quantity', 1)
    if not item_id:
        return jsonify({'error': '缺少商品ID'}), 400
    result, err = buy_shop_item(g.current_user['id'], item_id, quantity)
    if err:
        return jsonify({'error': err}), 400
    return jsonify({'message': '购买成功', **result})

@shop_bp.route('/orders', methods=['GET'])
@token_required
def my_orders():
    orders = query_db(
        '''SELECT so.*, si.name as item_name, si.icon as item_icon, si.item_type
           FROM shop_orders so JOIN shop_items si ON so.item_id = si.id
           WHERE so.user_id = ? ORDER BY so.created_at DESC LIMIT 50''',
        (g.current_user['id'],))
    return jsonify(orders)

@shop_bp.route('/coins', methods=['GET'])
@token_required
def my_coins():
    user = get_user_by_id(g.current_user['id'])
    return jsonify({
        'coins': user.get('coins', 0),
        'points': user.get('points', 0)
    })

@shop_bp.route('/transactions', methods=['GET'])
@token_required
def my_transactions():
    return jsonify(get_coin_transactions(g.current_user['id']))

@shop_bp.route('/use', methods=['POST'])
@token_required
def use_item():
    """使用已购道具（改名卡/置顶卡/匿名卡/称号/彩虹昵称）"""
    data = request.get_json(silent=True) or {}
    item_id = data.get('item_id')
    if not item_id:
        return jsonify({'error': '缺少商品ID'}), 400
    result, err = use_shop_item(g.current_user['id'], item_id, data.get('extra') or {})
    if err:
        return jsonify({'error': err}), 400
    return jsonify({'message': result['message']})


# ══════════════════════════════════════════════
# 恋爱情报专区
# ══════════════════════════════════════════════

romance_bp = Blueprint('romance', __name__)

@romance_bp.route('/profiles', methods=['GET'])
def list_profiles():
    limit = request.args.get('limit', 50, type=int)
    offset = request.args.get('offset', 0, type=int)
    gender = request.args.get('gender')
    return jsonify(get_romance_profiles(limit, offset, gender))

@romance_bp.route('/profile', methods=['GET'])
@token_required
def my_profile():
    profile = get_romance_profile(g.current_user['id'])
    return jsonify(profile or {})

@romance_bp.route('/profile', methods=['POST'])
@token_required
def save_profile():
    data = request.get_json(silent=True) or {}
    create_romance_profile(g.current_user['id'], **data)
    return jsonify({'message': '保存成功'})

@romance_bp.route('/profile/<int:uid>', methods=['GET'])
def view_profile(uid):
    profile = get_romance_profile(uid)
    if not profile:
        return jsonify({'error': '未找到'}), 404
    return jsonify(profile)

@romance_bp.route('/link', methods=['POST'])
@token_required
def link():
    data = request.get_json(silent=True) or {}
    to_uid = data.get('to_user_id')
    if not to_uid:
        return jsonify({'error': '缺少目标用户'}), 400
    if to_uid == g.current_user['id']:
        return jsonify({'error': '不能给自己发链接'}), 400
    lid = create_romance_link(
        g.current_user['id'], to_uid,
        data.get('link_type', 'crush'),
        data.get('description', ''),
        data.get('is_anonymous', 0)
    )
    return jsonify({'message': '发送成功', 'id': lid}), 201

@romance_bp.route('/links', methods=['GET'])
@token_required
def my_links():
    direction = request.args.get('direction', 'to')
    return jsonify(get_romance_links(g.current_user['id'], direction))

@romance_bp.route('/task', methods=['POST'])
@token_required
def create_task():
    data = request.get_json(silent=True) or {}
    title = (data.get('title') or '').strip()
    if not title:
        return jsonify({'error': '标题不能为空'}), 400
    tid = create_romance_task(
        g.current_user['id'], title,
        data.get('description', ''), data.get('task_type', 'matchmake'),
        data.get('target_user_id', 0), data.get('reward_coins', 0)
    )
    return jsonify({'message': '任务发布成功', 'id': tid}), 201

@romance_bp.route('/tasks', methods=['GET'])
def list_tasks():
    status = request.args.get('status', 'open')
    return jsonify(get_romance_tasks(status))


# ══════════════════════════════════════════════
# 爆料/树洞
# ══════════════════════════════════════════════

gossip_bp = Blueprint('gossip', __name__)

@gossip_bp.route('', methods=['GET'])
@optional_auth
def list_gossip():
    limit = request.args.get('limit', 50, type=int)
    offset = request.args.get('offset', 0, type=int)
    station_id = request.args.get('station_id', type=int)
    sort = request.args.get('sort', 'newest')
    items = get_gossip(station_id, limit, offset, sort)
    if g.current_user:
        for item in items:
            item['is_liked'] = is_liked(g.current_user['id'], 'gossip', item['id'])
    return jsonify(items)

@gossip_bp.route('', methods=['POST'])
@limiter.limit('10/minute')
@token_required
def post_gossip():
    data = request.get_json(silent=True) or {}
    content = (data.get('content') or '').strip()
    if not content:
        return jsonify({'error': '内容不能为空'}), 400
    if scan_text(content):
        return jsonify({'error': '内容包含违规信息，请修改后再发布'}), 400
    gid = create_gossip(
        content, data.get('station_id'),
        json.dumps(data.get('images', []), ensure_ascii=False),
        data.get('is_anonymous', 1)
    )
    return jsonify({'message': '发布成功', 'id': gid}), 201

@gossip_bp.route('/<int:gid>', methods=['GET'])
@optional_auth
def get_one_gossip(gid):
    g_item = get_gossip_by_id(gid)
    if not g_item:
        return jsonify({'error': '不存在'}), 404
    g_item['comments'] = get_gossip_comments(gid)
    if g.current_user:
        g_item['is_liked'] = is_liked(g.current_user['id'], 'gossip', gid)
    return jsonify(g_item)

@gossip_bp.route('/<int:gid>/like', methods=['POST'])
@token_required
def like_gossip(gid):
    result = toggle_gossip_like(gid, g.current_user['id'])
    return jsonify(result)

@gossip_bp.route('/<int:gid>/comment', methods=['POST'])
@token_required
def comment_gossip(gid):
    data = request.get_json(silent=True) or {}
    content = (data.get('content') or '').strip()
    if not content:
        return jsonify({'error': '评论不能为空'}), 400
    author = data.get('author_name', '匿名')
    if not data.get('is_anonymous', True) and g.current_user:
        author = g.current_user['username']
    cid = create_gossip_comment(gid, content, author)
    return jsonify({'message': '评论成功', 'id': cid}), 201


# ══════════════════════════════════════════════
# 交易
# ══════════════════════════════════════════════

trade_bp = Blueprint('trade', __name__)

@trade_bp.route('', methods=['GET'])
def list_trades():
    limit = request.args.get('limit', 50, type=int)
    offset = request.args.get('offset', 0, type=int)
    category = request.args.get('category')
    keyword = (request.args.get('q') or '').strip() or None
    return jsonify(get_trade_posts(category, limit=limit, offset=offset, keyword=keyword))

@trade_bp.route('', methods=['POST'])
@limiter.limit('10/minute')
@token_required
def create_trade():
    from app.models import create_post as create_post_fn, get_post_by_id, get_station_by_id
    data = request.get_json(silent=True) or {}
    title = (data.get('title') or '').strip()
    content = (data.get('content') or '').strip()
    price = data.get('price', 0)
    if not title or not content:
        return jsonify({'error': '请填写完整'}), 400

    station_id = data.get('station_id')
    if not station_id:
        # 默认找"二手交易"子站
        station = query_db("SELECT id FROM stations WHERE name LIKE '%二手%' OR name LIKE '%交易%' LIMIT 1", one=True)
        station_id = station['id'] if station else 1

    post_id = create_post_fn(title, content, g.current_user['id'], station_id, data.get('image', ''))
    if not post_id:
        return jsonify({'error': '创建失败'}), 500

    trade_id = create_trade_post(
        post_id, g.current_user['id'], price,
        data.get('original_price', 0), data.get('condition', 'good'),
        data.get('category', ''), data.get('contact', '')
    )
    return jsonify({'message': '发布成功', 'post_id': post_id, 'trade_id': trade_id}), 201

@trade_bp.route('/<int:tid>/status', methods=['PUT'])
@token_required
def update_status(tid):
    data = request.get_json(silent=True) or {}
    status = data.get('status', 'available')
    if status not in ('available', 'reserved', 'sold'):
        return jsonify({'error': '无效状态'}), 400
    # 属主校验：此前任何人可改任意在售商品的状态（审计修复）
    row = query_db('SELECT user_id FROM trade_posts WHERE id = ?', (tid,), one=True)
    if not row:
        return jsonify({'error': '商品不存在'}), 404
    if row['user_id'] != g.current_user['id'] and g.current_user.get('role') != 'admin':
        return jsonify({'error': '无权修改该商品'}), 403
    update_trade_status(tid, status)
    return jsonify({'message': '更新成功', 'status': status})


# ══════════════════════════════════════════════
# 看板娘
# ══════════════════════════════════════════════

kanban_bp = Blueprint('kanban', __name__)

@kanban_bp.route('/message', methods=['GET'])
def kanban_message():
    msg = get_kanban_message()
    user = g.current_user if hasattr(g, 'current_user') and g.current_user else None
    if user:
        name = user.get('username', '')
        msg = msg.replace('主人', name)
    return jsonify({'message': msg})


# ══════════════════════════════════════════════
# 推荐/搜索
# ══════════════════════════════════════════════

recommend_bp = Blueprint('recommend', __name__)

@recommend_bp.route('/posts', methods=['GET'])
@optional_auth
def recommended_posts():
    limit = request.args.get('limit', 20, type=int)
    offset = request.args.get('offset', 0, type=int)
    station_id = request.args.get('station_id', type=int)
    posts = get_recommended_posts(
        g.current_user['id'] if g.current_user else None,
        limit, offset, station_id
    )
    # 与 /api/posts 对齐：匿名脱敏（owner/admin 例外）+ vote/link 展开 + 批量点赞态
    liked = is_liked_batch(g.current_user['id'], 'post', [p['id'] for p in posts]) if (g.current_user and posts) else set()
    for p in posts:
        if g.current_user:
            p['is_liked'] = p['id'] in liked
        shape_post(p, g.current_user)
    return jsonify(posts)

@recommend_bp.route('/interests', methods=['GET'])
@token_required
def interests():
    return jsonify(get_user_interest_stations(g.current_user['id']))

@recommend_bp.route('/search', methods=['GET'])
@optional_auth
def search():
    keyword = (request.args.get('q') or '').strip()
    if not keyword:
        return jsonify({'stations': [], 'posts': [], 'users': []})
    results = smart_search(keyword, g.current_user['id'] if g.current_user else None)
    # 搜索结果同样过匿名脱敏/展开闸（防匿名作者经搜索泄露真名）
    for p in results.get('posts', []):
        shape_post(p, g.current_user)
    return jsonify(results)


# ══════════════════════════════════════════════
# 管理后台
# ══════════════════════════════════════════════

admin_bp = Blueprint('admin', __name__)

def admin_required(f):
    from functools import wraps
    @wraps(f)
    def decorated(*args, **kwargs):
        if not g.current_user or g.current_user['role'] != 'admin':
            return jsonify({'error': '需要管理员权限'}), 403
        return f(*args, **kwargs)
    return decorated

@admin_bp.route('/stats', methods=['GET'])
@token_required
@admin_required
def stats():
    return jsonify(get_admin_stats())

@admin_bp.route('/stats/series', methods=['GET'])
@token_required
@admin_required
def stats_series():
    days = request.args.get('days', 7, type=int)
    return jsonify(get_admin_stats_series(days))

@admin_bp.route('/users', methods=['GET'])
@token_required
@admin_required
def users():
    limit = request.args.get('limit', 100, type=int)
    offset = request.args.get('offset', 0, type=int)
    return jsonify(get_all_users_admin(limit, offset))

@admin_bp.route('/users/<int:uid>', methods=['PUT'])
@token_required
@admin_required
def update_user(uid):
    data = request.get_json(silent=True) or {}
    update_user_admin(uid, **data)
    admin_log(g.current_user['id'], 'update_user', 'user', uid, json.dumps(data, ensure_ascii=False))
    return jsonify({'message': '更新成功'})

@admin_bp.route('/stations', methods=['GET'])
@token_required
@admin_required
def stations():
    return jsonify(query_db('SELECT * FROM stations ORDER BY id DESC'))

@admin_bp.route('/posts', methods=['GET'])
@token_required
@admin_required
def posts():
    limit = request.args.get('limit', 100, type=int)
    return jsonify(query_db(
        '''SELECT p.*, u.username as author_name, u.identity_group as author_identity_group, s.name as station_name
           FROM posts p JOIN users u ON p.author_id = u.id JOIN stations s ON p.station_id = s.id
           ORDER BY p.created_at DESC LIMIT ?''', (limit,)))

@admin_bp.route('/posts/<int:pid>/pin', methods=['POST'])
@token_required
@admin_required
def pin_post(pid):
    from app.models import query_db as qdb, execute_db as edb
    post = qdb('SELECT is_pinned FROM posts WHERE id = ?', (pid,), one=True)
    if not post:
        return jsonify({'error': '帖子不存在'}), 404
    new_val = 0 if post['is_pinned'] else 1
    edb('UPDATE posts SET is_pinned = ? WHERE id = ?', (new_val, pid))
    admin_log(g.current_user['id'], 'pin_post', 'post', pid, f'is_pinned={new_val}')
    return jsonify({'message': '已置顶' if new_val else '已取消置顶'})

@admin_bp.route('/posts/<int:pid>', methods=['DELETE'])
@token_required
@admin_required
def delete_post(pid):
    from app.models import delete_post as del_post
    del_post(pid)
    admin_log(g.current_user['id'], 'delete_post', 'post', pid)
    return jsonify({'message': '已删除'})

@admin_bp.route('/logs', methods=['GET'])
@token_required
@admin_required
def logs():
    return jsonify(get_admin_logs())

@admin_bp.route('/shop/items', methods=['POST'])
@token_required
@admin_required
def create_shop_item():
    data = request.get_json(silent=True) or {}
    from app.models_ext import execute_db
    execute_db(
        'INSERT INTO shop_items (name, description, icon, price_coins, price_points, item_type, item_data, stock) VALUES (?,?,?,?,?,?,?,?)',
        (data.get('name',''), data.get('description',''), data.get('icon','gift'),
         data.get('price_coins',0), data.get('price_points',0),
         data.get('item_type','badge'), json.dumps(data.get('item_data',{}), ensure_ascii=False),
         data.get('stock',-1))
    )
    return jsonify({'message': '创建成功'}), 201

@admin_bp.route('/identity-groups', methods=['GET'])
@token_required
@admin_required
def identity_groups():
    return jsonify(get_identity_groups())


@admin_bp.route('/announcements', methods=['GET'])
@token_required
@admin_required
def admin_announcements():
    return jsonify(get_announcements())

@admin_bp.route('/announcements', methods=['POST'])
@token_required
@admin_required
def admin_create_announcement():
    data = request.get_json(silent=True) or {}
    title = (data.get('title') or '').strip()
    content = (data.get('content') or '').strip()
    if not title:
        return jsonify({'error': '公告标题不能为空'}), 400
    if len(title) > 100:
        return jsonify({'error': '标题最多100字符'}), 400
    if len(content) > 2000:
        return jsonify({'error': '内容最多2000字符'}), 400
    create_announcement(title, content, g.current_user['id'])
    admin_log(g.current_user['id'], 'create_announcement', 'announcement', 0, title)
    return jsonify({'message': '公告已发布'}), 201

@admin_bp.route('/announcements/<int:aid>', methods=['PUT'])
@token_required
@admin_required
def admin_update_announcement(aid):
    data = request.get_json(silent=True) or {}
    title = (data.get('title') or '').strip()
    content = (data.get('content') or '').strip()
    update_announcement(aid, title or None, content or None)
    return jsonify({'message': '已更新'})

@admin_bp.route('/announcements/<int:aid>/toggle', methods=['POST'])
@token_required
@admin_required
def admin_toggle_announcement(aid):
    data = request.get_json(silent=True) or {}
    toggle_announcement(aid, data.get('active', True))
    return jsonify({'message': '已更新状态'})

@admin_bp.route('/announcements/<int:aid>', methods=['DELETE'])
@token_required
@admin_required
def admin_delete_announcement(aid):
    delete_announcement(aid)
    return jsonify({'message': '已删除'})


# ── 公开公告 ──
announcements_bp = Blueprint('announcements', __name__)

@announcements_bp.route('', methods=['GET'])
def public_announcements():
    return jsonify(get_announcements(active_only=True, limit=5))


# ══════════════════════════════════════════════
# 收藏
# ══════════════════════════════════════════════

favorites_bp = Blueprint('favorites', __name__)


@favorites_bp.route('/<string:target_type>/<int:target_id>', methods=['POST'])
@token_required
def toggle_fav(target_type, target_id):
    if target_type not in ('post', 'station'):
        return jsonify({'error': '不支持的收藏类型'}), 400
    favorited = toggle_favorite(g.current_user['id'], target_type, target_id)
    return jsonify({'favorited': favorited, 'message': '已收藏' if favorited else '已取消收藏'})


@favorites_bp.route('/status/<string:target_type>/<int:target_id>', methods=['GET'])
@token_required
def fav_status(target_type, target_id):
    return jsonify({'favorited': is_favorited(g.current_user['id'], target_type, target_id)})


@favorites_bp.route('', methods=['GET'])
@token_required
def my_favorites():
    target_type = request.args.get('type', 'post')
    limit = request.args.get('limit', 50, type=int)
    offset = request.args.get('offset', 0, type=int)
    items = get_favorites(g.current_user['id'], target_type, limit, offset)
    return jsonify({'items': items, 'type': target_type})


# ══════════════════════════════════════════════
# 举报
# ══════════════════════════════════════════════

reports_bp = Blueprint('reports', __name__)


@admin_bp.route('/review', methods=['GET'])
@token_required
@admin_required
def review_queue():
    """待审核帖子队列（作者/子站信息齐全，含匿名帖真实身份——仅管理员可见）。"""
    limit = request.args.get('limit', 50, type=int)
    offset = request.args.get('offset', 0, type=int)
    posts = get_posts(status='pending', limit=limit, offset=offset)
    return jsonify({'posts': posts, 'total': len(posts)})


@admin_bp.route('/review', methods=['POST'])
@token_required
@admin_required
def review_action():
    """POST /api/admin/review {post_id, action: approve|reject, note?}"""
    data = request.get_json(silent=True) or {}
    pid = data.get('post_id')
    action = data.get('action')
    note = (data.get('note') or '').strip()
    post = get_post_by_id(pid) if pid else None
    if not post:
        return jsonify({'error': '帖子不存在'}), 404
    if post.get('status') != 'pending':
        return jsonify({'error': '该帖子不在审核队列'}), 400
    if action not in ('approve', 'reject'):
        return jsonify({'error': 'action 仅支持 approve/reject'}), 400

    if action == 'approve':
        update_post_status(pid, 'approved')
        msg = f'你的帖子「{post["title"]}」已通过审核'
    else:
        update_post_status(pid, 'rejected')
        execute_db('UPDATE stations SET post_count = MAX(post_count - 1, 0) WHERE id = ?', (post['station_id'],))
        msg = f'你的帖子「{post["title"]}」未通过审核' + (f'：{note}' if note else '')
    create_notification(post['author_id'], g.current_user['id'], 'system', msg, f'/post/{pid}')
    admin_log(g.current_user['id'], f'review_{action}', 'post', pid, note)
    return jsonify({'message': '已通过' if action == 'approve' else '已驳回'})


@reports_bp.route('', methods=['POST'])
@token_required
def report():
    data = request.get_json(silent=True) or {}
    target_type = data.get('target_type', '')
    target_id = data.get('target_id')
    reason = (data.get('reason') or '').strip()
    detail = (data.get('detail') or '').strip()

    if target_type not in ('post', 'comment', 'user', 'gossip', 'trade'):
        return jsonify({'error': '不支持的举报类型'}), 400
    if not target_id:
        return jsonify({'error': '缺少举报目标'}), 400
    if not reason:
        return jsonify({'error': '请选择举报原因'}), 400

    rid = create_report(g.current_user['id'], target_type, int(target_id), reason, detail)
    if not rid:
        return jsonify({'error': '该内容已举报，等待处理'}), 409
    return jsonify({'message': '举报成功，感谢反馈', 'id': rid}), 201


@reports_bp.route('', methods=['GET'])
@token_required
@admin_required
def list_reports():
    status = request.args.get('status', 'pending')
    return jsonify(get_reports(status))


@reports_bp.route('/<int:rid>', methods=['PUT'])
@token_required
@admin_required
def handle(rid):
    data = request.get_json(silent=True) or {}
    status = data.get('status', '')
    note = (data.get('note') or '').strip()
    if status not in ('approved', 'rejected'):
        return jsonify({'error': '无效的处理结果'}), 400
    handle_report(rid, g.current_user['id'], status, note)
    admin_log(g.current_user['id'], 'handle_report', 'report', rid, status)
    return jsonify({'message': '已处理'})


# ══════════════════════════════════════════════
# 角色关系图（world）：多人共同维护、可视化
# ══════════════════════════════════════════════

world_bp = Blueprint('world', __name__, url_prefix='/api/world')


@world_bp.route('/graph', methods=['GET'])
@optional_auth
def graph():
    return jsonify(get_character_graph())


@world_bp.route('/nodes/<int:nid>', methods=['GET'])
def node_detail(nid):
    node = get_character_node(nid)
    if not node:
        return jsonify({'error': '角色不存在'}), 404
    return jsonify(node)


@world_bp.route('/nodes', methods=['POST'])
@token_required
def create_node():
    data = request.get_json(silent=True) or {}
    name = (data.get('name') or '').strip()
    if not name:
        return jsonify({'error': '角色名不能为空'}), 400
    if len(name) > 30:
        return jsonify({'error': '角色名过长'}), 400
    portrait = (data.get('portrait') or '').strip()[:10]
    tagline = (data.get('tagline') or '').strip()[:80]
    color = (data.get('color') or '').strip()[:20]
    nid = create_character_node(name, portrait, tagline, color, g.current_user['id'])
    return jsonify({'id': nid, 'message': '角色已添加'})


@world_bp.route('/nodes/<int:nid>', methods=['PUT'])
@token_required
def update_node(nid):
    node = query_db('SELECT * FROM character_nodes WHERE id = ?', (nid,), one=True)
    if not node:
        return jsonify({'error': '角色不存在'}), 404
    if node['created_by'] != g.current_user['id'] and g.current_user['role'] != 'admin':
        return jsonify({'error': '无权修改他人角色'}), 403
    data = request.get_json(silent=True) or {}
    fields = {
        'name': (data.get('name') or '').strip()[:30],
        'portrait': (data.get('portrait') or '').strip()[:10],
        'tagline': (data.get('tagline') or '').strip()[:80],
        'color': (data.get('color') or '').strip()[:20],
    }
    update_character_node(nid, **fields)
    return jsonify({'message': '已更新'})


@world_bp.route('/nodes/<int:nid>', methods=['DELETE'])
@token_required
def remove_node(nid):
    node = query_db('SELECT * FROM character_nodes WHERE id = ?', (nid,), one=True)
    if not node:
        return jsonify({'error': '角色不存在'}), 404
    if node['created_by'] != g.current_user['id'] and g.current_user['role'] != 'admin':
        return jsonify({'error': '无权删除他人角色'}), 403
    delete_character_node(nid)
    return jsonify({'message': '已删除'})


@world_bp.route('/relations', methods=['POST'])
@token_required
def create_rel():
    data = request.get_json(silent=True) or {}
    label = (data.get('label') or '').strip()
    from_id = data.get('from_id')
    to_id = data.get('to_id')
    if not label:
        return jsonify({'error': '关系标签不能为空'}), 400
    if len(label) > 20:
        return jsonify({'error': '关系标签过长'}), 400
    description = (data.get('description') or '').strip()[:120]
    reciprocal = 1 if data.get('reciprocal') else 0
    rid = create_character_relation(from_id, to_id, label, description, reciprocal, g.current_user['id'])
    if not rid:
        return jsonify({'error': '来源/目标角色无效，或不能指向自己'}), 400
    return jsonify({'id': rid, 'message': '关系已添加'})


@world_bp.route('/relations/<int:rid>', methods=['DELETE'])
@token_required
def remove_rel(rid):
    rel = query_db('SELECT * FROM character_relations WHERE id = ?', (rid,), one=True)
    if not rel:
        return jsonify({'error': '关系不存在'}), 404
    if rel['created_by'] != g.current_user['id'] and g.current_user['role'] != 'admin':
        return jsonify({'error': '无权删除他人关系'}), 403
    delete_character_relation(rid)
    return jsonify({'message': '已删除'})


# ══════════════════════════════════════════════
# 站点配置 / 广告位
# ══════════════════════════════════════════════

site_bp = Blueprint('site', __name__, url_prefix='/api/site')


@site_bp.route('/config', methods=['GET'])
def site_config():
    """公开读取广告位等站点配置，前端据此决定是否渲染占位。"""
    return jsonify(get_ad_config())


# admin_bp 内新增：广告位配置
@admin_bp.route('/site/config', methods=['PUT'])
@token_required
@admin_required
def admin_site_config():
    data = request.get_json(silent=True) or {}
    for k in ('ad_enabled', 'ad_header', 'ad_footer'):
        if k in data:
            set_site_config(k, data[k])
    return jsonify({'message': '已保存', **get_ad_config()})
