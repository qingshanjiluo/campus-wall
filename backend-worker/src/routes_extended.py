"""扩展路由：身份组、签到、积分商城、恋爱情报、爆料、交易、看板娘、推荐、管理后台、收藏、举报、公告。"""
import json

import web as httpmod
import auth as authmod
import db as _db
from models import get_user_by_id, create_post, get_post_by_id, is_liked
from models_ext import (
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
    get_admin_stats, get_admin_stats_series, get_all_users_admin,
    update_user_admin, admin_log, get_admin_logs,
    toggle_favorite, is_favorited, get_favorites,
    create_report, get_reports, handle_report,
    get_announcements, create_announcement, update_announcement,
    toggle_announcement, delete_announcement,
)

from http import HTTPMethod


async def _require_user(request):
    user = await authmod.require_user(request)
    if not user:
        return None, httpmod.error('缺少认证令牌', 401)
    return user, None


async def _optional_user(request):
    return await authmod.optional_user(request)


async def _require_admin(request):
    user, resp = await _require_user(request)
    if resp:
        return None, resp
    if user['role'] != 'admin':
        return None, httpmod.error('需要管理员权限', 403)
    return user, None


# ────── 通用文件上传辅助 ──────
from uploads import save_upload as _save_upload


# ─────────────────────────
# 身份组
# ─────────────────────────

async def list_groups(request, params):
    return httpmod.jsonify(await get_identity_groups())


async def create_group(request, params):
    user, resp = await _require_admin(request)
    if resp:
        return resp
    data = await httpmod.get_json_body(request) or {}
    name = (data.get('name') or '').strip()
    if not name:
        return httpmod.error('名称不能为空', 400)
    gid = await create_identity_group(
        name, data.get('icon', 'tag'), data.get('color', '#fb6f92'),
        data.get('description', ''), data.get('permissions', []),
        data.get('min_level', 1), data.get('is_default', 0))
    return httpmod.jsonify({'message': '创建成功', 'id': gid}, status=201)


async def assign(request, params):
    user, resp = await _require_admin(request)
    if resp:
        return resp
    data = await httpmod.get_json_body(request) or {}
    uid = data.get('user_id')
    group = data.get('group', '')
    if not uid:
        return httpmod.error('缺少用户ID', 400)
    await assign_user_group(uid, group)
    await admin_log(user['id'], 'assign_group', 'user', uid, f'设置身份组: {group}')
    return httpmod.jsonify({'message': '设置成功'})


async def my_group(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    group = await get_user_group(user)
    return httpmod.jsonify(group or {})


# ─────────────────────────
# 签到
# ─────────────────────────

async def checkin(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    result = await do_checkin(user['id'])
    if not result:
        return httpmod.jsonify({'message': '今天已经签到过了', 'already': True})
    return httpmod.jsonify({'message': '签到成功！', **result})


async def status(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    today = await get_checkin_today(user['id'])
    history = await get_checkin_history(user['id'], 30)
    u = await get_user_by_id(user['id'])
    return httpmod.jsonify({
        'checked_in_today': today is not None,
        'streak': u.get('checkin_streak', 0),
        'coins': u.get('coins', 0),
        'points': u.get('points', 0),
        'history': history,
    })


# ─────────────────────────
# 积分商城
# ─────────────────────────

async def list_items(request, params):
    return httpmod.jsonify(await get_shop_items())


async def get_item(request, params):
    item_id = int(params['item_id'])
    item = await get_shop_item_by_id(item_id)
    if not item:
        return httpmod.error('商品不存在', 404)
    return httpmod.jsonify(item)


async def buy(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    data = await httpmod.get_json_body(request) or {}
    item_id = data.get('item_id')
    quantity = data.get('quantity', 1)
    if not item_id:
        return httpmod.error('缺少商品ID', 400)
    result, err = await buy_shop_item(user['id'], item_id, quantity)
    if err:
        return httpmod.error(err, 400)
    return httpmod.jsonify({'message': '购买成功', **result})


async def my_orders(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    orders = await _db.query(
        '''SELECT so.*, si.name as item_name, si.icon as item_icon, si.item_type
           FROM shop_orders so JOIN shop_items si ON so.item_id = si.id
           WHERE so.user_id = ? ORDER BY so.created_at DESC LIMIT 50''',
        (user['id'],))
    return httpmod.jsonify(orders)


async def my_coins(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    u = await get_user_by_id(user['id'])
    return httpmod.jsonify({'coins': u.get('coins', 0), 'points': u.get('points', 0)})


async def my_transactions(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    return httpmod.jsonify(await get_coin_transactions(user['id']))


async def use_item(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    data = await httpmod.get_json_body(request) or {}
    item_id = data.get('item_id')
    if not item_id:
        return httpmod.error('缺少商品ID', 400)
    result, err = await use_shop_item(user['id'], item_id, data.get('extra') or {})
    if err:
        return httpmod.error(err, 400)
    return httpmod.jsonify({'message': result['message']})


# ─────────────────────────
# 恋爱情报专区
# ─────────────────────────

async def list_profiles(request, params):
    q = httpmod.get_query_params(request)
    limit = int(q.get('limit', 50))
    offset = int(q.get('offset', 0))
    gender = q.get('gender')
    return httpmod.jsonify(await get_romance_profiles(limit, offset, gender))


async def my_profile(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    profile = await get_romance_profile(user['id'])
    return httpmod.jsonify(profile or {})


async def save_profile(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    data = await httpmod.get_json_body(request) or {}
    await create_romance_profile(user['id'], **data)
    return httpmod.jsonify({'message': '保存成功'})


async def view_profile(request, params):
    uid = int(params['uid'])
    profile = await get_romance_profile(uid)
    if not profile:
        return httpmod.error('未找到', 404)
    return httpmod.jsonify(profile)


async def link(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    data = await httpmod.get_json_body(request) or {}
    to_uid = data.get('to_user_id')
    if not to_uid:
        return httpmod.error('缺少目标用户', 400)
    if to_uid == user['id']:
        return httpmod.error('不能给自己发链接', 400)
    lid = await create_romance_link(
        user['id'], to_uid,
        data.get('link_type', 'crush'),
        data.get('description', ''),
        data.get('is_anonymous', 0))
    return httpmod.jsonify({'message': '发送成功', 'id': lid}, status=201)


async def my_links(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    q = httpmod.get_query_params(request)
    direction = q.get('direction', 'to')
    return httpmod.jsonify(await get_romance_links(user['id'], direction))


async def create_task(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    data = await httpmod.get_json_body(request) or {}
    title = (data.get('title') or '').strip()
    if not title:
        return httpmod.error('标题不能为空', 400)
    tid = await create_romance_task(
        user['id'], title,
        data.get('description', ''), data.get('task_type', 'matchmake'),
        data.get('target_user_id', 0), data.get('reward_coins', 0))
    return httpmod.jsonify({'message': '任务发布成功', 'id': tid}, status=201)


async def list_tasks(request, params):
    q = httpmod.get_query_params(request)
    status = q.get('status', 'open')
    return httpmod.jsonify(await get_romance_tasks(status))


# ─────────────────────────
# 爆料/树洞
# ─────────────────────────

async def list_gossip(request, params):
    user = await _optional_user(request)
    q = httpmod.get_query_params(request)
    limit = int(q.get('limit', 50))
    offset = int(q.get('offset', 0))
    station_id = int(q['station_id']) if q.get('station_id') and q['station_id'].isdigit() else None
    sort = q.get('sort', 'newest')
    items = await get_gossip(station_id, limit, offset, sort)
    for item in items:
        item['is_liked'] = await is_liked(user['id'], 'gossip', item['id']) if user else False
    return httpmod.jsonify(items)


async def post_gossip(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    data = await httpmod.get_json_body(request) or {}
    content = (data.get('content') or '').strip()
    if not content:
        return httpmod.error('内容不能为空', 400)
    gid = await create_gossip(
        content, data.get('station_id'),
        data.get('images', []) if isinstance(data.get('images'), list) else [],
        data.get('is_anonymous', 1))
    return httpmod.jsonify({'message': '发布成功', 'id': gid}, status=201)


async def get_one_gossip(request, params):
    user = await _optional_user(request)
    gid = int(params['gid'])
    g_item = await get_gossip_by_id(gid)
    if not g_item:
        return httpmod.error('不存在', 404)
    g_item['comments'] = await get_gossip_comments(gid)
    g_item['is_liked'] = await is_liked(user['id'], 'gossip', gid) if user else False
    return httpmod.jsonify(g_item)


async def like_gossip(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    gid = int(params['gid'])
    result = await toggle_gossip_like(gid, user['id'])
    return httpmod.jsonify(result)


async def comment_gossip(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    gid = int(params['gid'])
    data = await httpmod.get_json_body(request) or {}
    content = (data.get('content') or '').strip()
    if not content:
        return httpmod.error('评论不能为空', 400)
    author = data.get('author_name', '匿名')
    if not data.get('is_anonymous', True) and user:
        author = user['username']
    cid = await create_gossip_comment(gid, content, author)
    return httpmod.jsonify({'message': '评论成功', 'id': cid}, status=201)


# ─────────────────────────
# 交易
# ─────────────────────────

async def list_trades(request, params):
    q = httpmod.get_query_params(request)
    limit = int(q.get('limit', 50))
    offset = int(q.get('offset', 0))
    category = q.get('category')
    keyword = (q.get('q') or '').strip() or None
    return httpmod.jsonify(await get_trade_posts(category, limit=limit, offset=offset, keyword=keyword))


async def create_trade(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    data = await httpmod.get_json_body(request) or {}
    title = (data.get('title') or '').strip()[:100]
    content = (data.get('content') or '').strip()[:5000]
    contact = str(data.get('contact') or '').strip()[:100]
    category = str(data.get('category') or '').strip()[:50]
    try:
        price = float(data.get('price') or 0)
        original_price = float(data.get('original_price') or 0)
    except (TypeError, ValueError):
        return httpmod.error('价格格式不正确', 400)
    if price < 0 or original_price < 0 or price > 1000000 or original_price > 1000000:
        return httpmod.error('价格超出合理范围', 400)
    condition = str(data.get('condition') or 'good').strip()
    if condition not in ('new', 'like_new', 'good', 'fair', 'poor'):
        condition = 'good'
    if not title or not content:
        return httpmod.error('请填写完整', 400)

    station_id = data.get('station_id')
    if not station_id:
        station = await _db.query(
            "SELECT id FROM stations WHERE name LIKE '%二手%' OR name LIKE '%交易%' LIMIT 1",
            one=True)
        station_id = station['id'] if station else 1

    post_id = await create_post(title, content, user['id'], station_id, data.get('image', ''))
    if not post_id:
        return httpmod.error('创建失败', 500)

    trade_id = await create_trade_post(
        post_id, user['id'], price,
        original_price, condition,
        category, contact)
    return httpmod.jsonify({'message': '发布成功', 'post_id': post_id, 'trade_id': trade_id}, status=201)


async def update_status(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    tid = int(params['tid'])
    data = await httpmod.get_json_body(request) or {}
    status = data.get('status', 'available')
    if status not in ('available', 'reserved', 'sold'):
        return httpmod.error('无效状态', 400)
    await update_trade_status(tid, status)
    return httpmod.jsonify({'message': '更新成功'})


# ─────────────────────────
# 看板娘
# ─────────────────────────

async def kanban_message(request, params):
    user = await _optional_user(request)
    msg = get_kanban_message()
    if user:
        msg = msg.replace('主人', user.get('username', ''))
    return httpmod.jsonify({'message': msg})


# ─────────────────────────
# 推荐/搜索
# ─────────────────────────

async def recommended_posts(request, params):
    user = await _optional_user(request)
    q = httpmod.get_query_params(request)
    limit = int(q.get('limit', 20))
    offset = int(q.get('offset', 0))
    posts = await get_recommended_posts(user['id'] if user else None, limit, offset)
    return httpmod.jsonify(posts)


async def interests(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    return httpmod.jsonify(await get_user_interest_stations(user['id']))


async def search(request, params):
    user = await _optional_user(request)
    q = httpmod.get_query_params(request)
    keyword = (q.get('q') or '').strip()
    if not keyword:
        return httpmod.jsonify({'stations': [], 'posts': [], 'users': []})
    return httpmod.jsonify(await smart_search(keyword, user['id'] if user else None))


# ─────────────────────────
# 管理后台
# ─────────────────────────

async def stats(request, params):
    user, resp = await _require_admin(request)
    if resp:
        return resp
    return httpmod.jsonify(await get_admin_stats())


async def stats_series(request, params):
    user, resp = await _require_admin(request)
    if resp:
        return resp
    q = httpmod.get_query_params(request)
    days = int(q.get('days', 7))
    return httpmod.jsonify(await get_admin_stats_series(days))


async def users(request, params):
    user, resp = await _require_admin(request)
    if resp:
        return resp
    q = httpmod.get_query_params(request)
    limit = int(q.get('limit', 100))
    offset = int(q.get('offset', 0))
    return httpmod.jsonify(await get_all_users_admin(limit, offset))


async def update_user(request, params):
    user, resp = await _require_admin(request)
    if resp:
        return resp
    uid = int(params['uid'])
    data = await httpmod.get_json_body(request) or {}
    await update_user_admin(uid, **data)
    await admin_log(user['id'], 'update_user', 'user', uid, json.dumps(data, ensure_ascii=False))
    return httpmod.jsonify({'message': '更新成功'})


async def stations(request, params):
    user, resp = await _require_admin(request)
    if resp:
        return resp
    return httpmod.jsonify(await _db.query('SELECT * FROM stations ORDER BY id DESC'))


async def posts(request, params):
    user, resp = await _require_admin(request)
    if resp:
        return resp
    q = httpmod.get_query_params(request)
    limit = int(q.get('limit', 100))
    return httpmod.jsonify(await _db.query(
        '''SELECT p.*, u.username as author_name, u.identity_group as author_identity_group,
                  s.name as station_name
           FROM posts p JOIN users u ON p.author_id = u.id
           JOIN stations s ON p.station_id = s.id
           ORDER BY p.created_at DESC LIMIT ?''', (limit,)))


async def pin_post(request, params):
    user, resp = await _require_admin(request)
    if resp:
        return resp
    pid = int(params['pid'])
    post = await _db.query('SELECT is_pinned FROM posts WHERE id = ?', (pid,), one=True)
    if not post:
        return httpmod.error('帖子不存在', 404)
    new_val = 0 if post['is_pinned'] else 1
    await _db.execute('UPDATE posts SET is_pinned = ? WHERE id = ?', (new_val, pid))
    await admin_log(user['id'], 'pin_post', 'post', pid, f'is_pinned={new_val}')
    return httpmod.jsonify({'message': '已置顶' if new_val else '已取消置顶'})


async def delete_post(request, params):
    user, resp = await _require_admin(request)
    if resp:
        return resp
    pid = int(params['pid'])
    await _db.execute('DELETE FROM posts WHERE id = ?', (pid,))
    await admin_log(user['id'], 'delete_post', 'post', pid)
    return httpmod.jsonify({'message': '已删除'})


async def logs(request, params):
    user, resp = await _require_admin(request)
    if resp:
        return resp
    return httpmod.jsonify(await get_admin_logs())


async def create_shop_item(request, params):
    user, resp = await _require_admin(request)
    if resp:
        return resp
    data = await httpmod.get_json_body(request) or {}
    await _db.execute(
        'INSERT INTO shop_items (name, description, icon, price_coins, price_points, item_type, item_data, stock) VALUES (?,?,?,?,?,?,?,?)',
        (data.get('name', ''), data.get('description', ''), data.get('icon', 'gift'),
         data.get('price_coins', 0), data.get('price_points', 0),
         data.get('item_type', 'badge'), json.dumps(data.get('item_data', {}), ensure_ascii=False),
         data.get('stock', -1)))
    return httpmod.jsonify({'message': '创建成功'}, status=201)


async def identity_groups(request, params):
    user, resp = await _require_admin(request)
    if resp:
        return resp
    return httpmod.jsonify(await get_identity_groups())


async def admin_announcements(request, params):
    user, resp = await _require_admin(request)
    if resp:
        return resp
    return httpmod.jsonify(await get_announcements())


async def admin_create_announcement(request, params):
    user, resp = await _require_admin(request)
    if resp:
        return resp
    data = await httpmod.get_json_body(request) or {}
    title = (data.get('title') or '').strip()
    content = (data.get('content') or '').strip()
    if not title:
        return httpmod.error('公告标题不能为空', 400)
    if len(title) > 100:
        return httpmod.error('标题最大100字符', 400)
    if len(content) > 2000:
        return httpmod.error('内容最大2000字符', 400)
    await create_announcement(title, content, user['id'])
    await admin_log(user['id'], 'create_announcement', 'announcement', 0, title)
    return httpmod.jsonify({'message': '公告已发布'}, status=201)


async def admin_update_announcement(request, params):
    user, resp = await _require_admin(request)
    if resp:
        return resp
    aid = int(params['aid'])
    data = await httpmod.get_json_body(request) or {}
    title = (data.get('title') or '').strip() or None
    content = (data.get('content') or '').strip() or None
    await update_announcement(aid, title, content)
    return httpmod.jsonify({'message': '已更新'})


async def admin_toggle_announcement(request, params):
    user, resp = await _require_admin(request)
    if resp:
        return resp
    aid = int(params['aid'])
    data = await httpmod.get_json_body(request) or {}
    await toggle_announcement(aid, data.get('active', True))
    return httpmod.jsonify({'message': '已更新状态'})


async def admin_delete_announcement(request, params):
    user, resp = await _require_admin(request)
    if resp:
        return resp
    aid = int(params['aid'])
    await delete_announcement(aid)
    return httpmod.jsonify({'message': '已删除'})


async def public_announcements(request, params):
    return httpmod.jsonify(await get_announcements(active_only=True, limit=5))


# ─────────────────────────
# 收藏
# ─────────────────────────

async def toggle_fav(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    target_type = params['target_type']
    target_id = int(params['target_id'])
    if target_type not in ('post', 'station'):
        return httpmod.error('不支持的收藏类型', 400)
    favorited = await toggle_favorite(user['id'], target_type, target_id)
    return httpmod.jsonify({'favorited': favorited, 'message': '已收藏' if favorited else '已取消收藏'})


async def fav_status(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    target_type = params['target_type']
    target_id = int(params['target_id'])
    return httpmod.jsonify({'favorited': await is_favorited(user['id'], target_type, target_id)})


async def my_favorites(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    q = httpmod.get_query_params(request)
    target_type = q.get('type', 'post')
    limit = int(q.get('limit', 50))
    offset = int(q.get('offset', 0))
    items = await get_favorites(user['id'], target_type, limit, offset)
    return httpmod.jsonify({'items': items, 'type': target_type})


# ─────────────────────────
# 举报
# ─────────────────────────

async def report(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    data = await httpmod.get_json_body(request) or {}
    target_type = data.get('target_type', '')
    target_id = data.get('target_id')
    reason = (data.get('reason') or '').strip()
    detail = (data.get('detail') or '').strip()

    if target_type not in ('post', 'comment', 'user', 'gossip', 'trade'):
        return httpmod.error('不支持的举报类型', 400)
    if not target_id:
        return httpmod.error('缺少举报目标', 400)
    if not reason:
        return httpmod.error('请选择举报原因', 400)

    rid = await create_report(user['id'], target_type, int(target_id), reason, detail)
    if not rid:
        return httpmod.error('该内容已举报，等待处理', 409)
    return httpmod.jsonify({'message': '举报成功，感谢反馈', 'id': rid}, status=201)


async def list_reports(request, params):
    user, resp = await _require_admin(request)
    if resp:
        return resp
    q = httpmod.get_query_params(request)
    status = q.get('status', 'pending')
    return httpmod.jsonify(await get_reports(status))


async def handle(request, params):
    user, resp = await _require_admin(request)
    if resp:
        return resp
    rid = int(params['rid'])
    data = await httpmod.get_json_body(request) or {}
    status = data.get('status', '')
    note = (data.get('note') or '').strip()
    if status not in ('approved', 'rejected'):
        return httpmod.error('无效的处理结果', 400)
    await handle_report(rid, user['id'], status, note)
    await admin_log(user['id'], 'handle_report', 'report', rid, status)
    return httpmod.jsonify({'message': '已处理'})


ROUTES = [
    # 身份组
    (HTTPMethod.GET, r'^/api/identity/groups$', list_groups),
    (HTTPMethod.POST, r'^/api/identity/groups$', create_group),
    (HTTPMethod.POST, r'^/api/identity/assign$', assign),
    (HTTPMethod.GET, r'^/api/identity/my$', my_group),
    # 签到
    (HTTPMethod.POST, r'^/api/checkin$', checkin),
    (HTTPMethod.GET, r'^/api/checkin/status$', status),
    # 商城
    (HTTPMethod.GET, r'^/api/shop/items$', list_items),
    (HTTPMethod.GET, r'^/api/shop/items/(?P<item_id>\d+)$', get_item),
    (HTTPMethod.POST, r'^/api/shop/buy$', buy),
    (HTTPMethod.GET, r'^/api/shop/orders$', my_orders),
    (HTTPMethod.GET, r'^/api/shop/coins$', my_coins),
    (HTTPMethod.GET, r'^/api/shop/transactions$', my_transactions),
    (HTTPMethod.POST, r'^/api/shop/use$', use_item),
    # 恋爱
    (HTTPMethod.GET, r'^/api/romance/profiles$', list_profiles),
    (HTTPMethod.GET, r'^/api/romance/profile$', my_profile),
    (HTTPMethod.POST, r'^/api/romance/profile$', save_profile),
    (HTTPMethod.GET, r'^/api/romance/profile/(?P<uid>\d+)$', view_profile),
    (HTTPMethod.POST, r'^/api/romance/link$', link),
    (HTTPMethod.GET, r'^/api/romance/links$', my_links),
    (HTTPMethod.POST, r'^/api/romance/task$', create_task),
    (HTTPMethod.GET, r'^/api/romance/tasks$', list_tasks),
    # 爆料/树洞
    (HTTPMethod.GET, r'^/api/gossip$', list_gossip),
    (HTTPMethod.POST, r'^/api/gossip$', post_gossip),
    (HTTPMethod.GET, r'^/api/gossip/(?P<gid>\d+)$', get_one_gossip),
    (HTTPMethod.POST, r'^/api/gossip/(?P<gid>\d+)/like$', like_gossip),
    (HTTPMethod.POST, r'^/api/gossip/(?P<gid>\d+)/comment$', comment_gossip),
    # 交易
    (HTTPMethod.GET, r'^/api/trade$', list_trades),
    (HTTPMethod.POST, r'^/api/trade$', create_trade),
    (HTTPMethod.PUT, r'^/api/trade/(?P<tid>\d+)/status$', update_status),
    # 看板娘
    (HTTPMethod.GET, r'^/api/kanban/message$', kanban_message),
    # 推荐/搜索
    (HTTPMethod.GET, r'^/api/recommend/posts$', recommended_posts),
    (HTTPMethod.GET, r'^/api/recommend/interests$', interests),
    (HTTPMethod.GET, r'^/api/recommend/search$', search),
    # 管理后台
    (HTTPMethod.GET, r'^/api/admin/stats$', stats),
    (HTTPMethod.GET, r'^/api/admin/stats/series$', stats_series),
    (HTTPMethod.GET, r'^/api/admin/users$', users),
    (HTTPMethod.PUT, r'^/api/admin/users/(?P<uid>\d+)$', update_user),
    (HTTPMethod.GET, r'^/api/admin/stations$', stations),
    (HTTPMethod.GET, r'^/api/admin/posts$', posts),
    (HTTPMethod.POST, r'^/api/admin/posts/(?P<pid>\d+)/pin$', pin_post),
    (HTTPMethod.DELETE, r'^/api/admin/posts/(?P<pid>\d+)$', delete_post),
    (HTTPMethod.GET, r'^/api/admin/logs$', logs),
    (HTTPMethod.POST, r'^/api/admin/shop/items$', create_shop_item),
    (HTTPMethod.GET, r'^/api/admin/identity-groups$', identity_groups),
    (HTTPMethod.GET, r'^/api/admin/announcements$', admin_announcements),
    (HTTPMethod.POST, r'^/api/admin/announcements$', admin_create_announcement),
    (HTTPMethod.PUT, r'^/api/admin/announcements/(?P<aid>\d+)$', admin_update_announcement),
    (HTTPMethod.POST, r'^/api/admin/announcements/(?P<aid>\d+)/toggle$', admin_toggle_announcement),
    (HTTPMethod.DELETE, r'^/api/admin/announcements/(?P<aid>\d+)$', admin_delete_announcement),
    # 公开公告
    (HTTPMethod.GET, r'^/api/announcements$', public_announcements),
    # 收藏
    (HTTPMethod.POST, r'^/api/favorites/(?P<target_type>post|station)/(?P<target_id>\d+)$', toggle_fav),
    (HTTPMethod.GET, r'^/api/favorites/status/(?P<target_type>post|station)/(?P<target_id>\d+)$', fav_status),
    (HTTPMethod.GET, r'^/api/favorites$', my_favorites),
    # 举报
    (HTTPMethod.POST, r'^/api/reports$', report),
    (HTTPMethod.GET, r'^/api/reports$', list_reports),
    (HTTPMethod.PUT, r'^/api/reports/(?P<rid>\d+)$', handle),
]