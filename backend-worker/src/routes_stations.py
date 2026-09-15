"""子站路由：列表 / 详情 / 创建 / 加入 / 退出 / 成员管理 / 统计。"""
import json

import web as httpmod
import auth as authmod
from models import (
    create_station, get_stations, get_station_by_id, search_stations,
    join_station, leave_station, is_station_member, get_station_member_role,
    get_station_membership, get_station_memberships,
    get_station_members, get_user_stations, remove_station_member,
    transfer_station_ownership, update_station, delete_station,
    get_station_stats, get_station_categories, get_posts, get_post_count,
    is_liked, is_liked_batch, shape_post,
)
from uploads import save_upload

from http import HTTPMethod


async def _require_user(request):
    user = await authmod.require_user(request)
    if not user:
        return None, httpmod.error('缺少认证令牌', 401)
    return user, None


async def _optional_user(request):
    return await authmod.optional_user(request)


async def _require_owner(user, sid):
    """校验当前用户是否为子站 owner，返回 (station, error_response)。"""
    station = await get_station_by_id(sid)
    if not station:
        return None, httpmod.error('子站不存在', 404)
    role = await get_station_member_role(user['id'], sid)
    if role != 'owner' and user['role'] != 'admin':
        return None, httpmod.error('仅子站长可操作', 403)
    return station, None


async def list_stations(request, params):
    user = await _optional_user(request)
    q = httpmod.get_query_params(request)
    limit = int(q.get('limit', 20))
    offset = int(q.get('offset', 0))
    sort = q.get('sort', 'newest')
    tag = q.get('tag', '').strip() or None
    category = q.get('category', '').strip() or None

    stations = await get_stations(limit=limit, offset=offset, sort=sort, tag=tag, category=category)
    # 批量成员关系：一条查询取代原来每站 2 次扫描（读预算关键热路径）
    roles = await get_station_memberships(user['id'], [s['id'] for s in stations]) if user else {}
    for s in stations:
        s['tags'] = json.loads(s['tags']) if s.get('tags') else []
        role = roles.get(s['id'])
        s['is_member'] = role is not None
        s['is_owner'] = role == 'owner'
    return httpmod.jsonify(stations)


async def categories(request, params):
    return httpmod.jsonify(await get_station_categories())


async def get_station(request, params):
    user = await _optional_user(request)
    sid = int(params['sid'])
    station = await get_station_by_id(sid)
    if not station:
        return httpmod.error('子站不存在', 404)
    station['tags'] = json.loads(station['tags']) if station.get('tags') else []
    if user:
        # 单查询同时得出 is_member / is_owner（原为同一行的 2 次扫描）
        role = await get_station_membership(user['id'], sid)
        station['is_member'] = role is not None
        station['is_owner'] = role == 'owner'
    else:
        station['is_member'] = False
        station['is_owner'] = False
    return httpmod.jsonify(station)


async def create(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    data = await httpmod.get_json_body(request) or {}
    name = (data.get('name') or '').strip()
    description = (data.get('description') or '').strip()
    icon = data.get('icon', 'school')
    tags = data.get('tags', []) if isinstance(data.get('tags'), list) else []

    if not name or len(name) < 2:
        return httpmod.error('子站名称至少2个字符', 400)

    sid = await create_station(
        name, description, icon, tags, user['id'],
        category=(data.get('category') or '').strip())
    if not sid:
        return httpmod.error('创建失败', 500)

    station = await get_station_by_id(sid)
    station['tags'] = json.loads(station['tags']) if station.get('tags') else []
    return httpmod.jsonify({'message': '创建成功', 'station': station}, status=201)


async def join(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    sid = int(params['sid'])
    station = await get_station_by_id(sid)
    if not station:
        return httpmod.error('子站不存在', 404)
    if await is_station_member(user['id'], sid):
        return httpmod.jsonify({'message': '已经是成员了'})
    await join_station(user['id'], sid)
    return httpmod.jsonify({'message': '加入成功'})


async def leave(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    sid = int(params['sid'])
    station = await get_station_by_id(sid)
    if not station:
        return httpmod.error('子站不存在', 404)
    await leave_station(user['id'], sid)
    return httpmod.jsonify({'message': '已退出'})


async def station_posts(request, params):
    sid = int(params['sid'])
    station = await get_station_by_id(sid)
    if not station:
        return httpmod.error('子站不存在', 404)
    q = httpmod.get_query_params(request)
    limit = int(q.get('limit', 20))
    offset = int(q.get('offset', 0))
    sort = q.get('sort', 'newest')
    user = await authmod.optional_user(request)
    posts = await get_posts(station_id=sid, limit=limit, offset=offset, sort=sort)
    total = await get_post_count(station_id=sid)
    liked = await is_liked_batch(user['id'], 'post', [p['id'] for p in posts]) if (user and posts) else set()
    for p in posts:
        if user:
            p['is_liked'] = p['id'] in liked
        shape_post(p, user)  # 匿名脱敏 + vote/link 展开
    return httpmod.jsonify({'posts': posts, 'total': total})


async def search(request, params):
    q = httpmod.get_query_params(request)
    keyword = q.get('q', '').strip()
    if not keyword:
        return httpmod.jsonify([])
    stations = await search_stations(keyword)
    for s in stations:
        s['tags'] = json.loads(s['tags']) if s.get('tags') else []
    return httpmod.jsonify(stations)


async def my_stations(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    stations = await get_user_stations(user['id'])
    for s in stations:
        s['tags'] = json.loads(s['tags']) if s.get('tags') else []
    return httpmod.jsonify(stations)


async def user_stations(request, params):
    """GET /api/stations/by/<uid>：他人主页的子站 Tab。
    隐私：仅返回公开子站的加入关系。"""
    uid = int(params['uid'])
    stations = await get_user_stations(uid)
    out = []
    for s in stations:
        if s.get('is_public') == 0:
            continue
        s['tags'] = json.loads(s['tags']) if s.get('tags') else []
        s.pop('role', None)
        out.append(s)
    return httpmod.jsonify(out)


async def upload_cover(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    url, err = await save_upload(request, prefix='cover', subdir='covers')
    if err:
        return httpmod.error(err, 400)
    return httpmod.jsonify({'url': url})


async def station_stats(request, params):
    sid = int(params['sid'])
    if not await get_station_by_id(sid):
        return httpmod.error('子站不存在', 404)
    q = httpmod.get_query_params(request)
    days = int(q.get('days', 7))
    return httpmod.jsonify(await get_station_stats(sid, days))


async def members(request, params):
    user = await _optional_user(request)
    sid = int(params['sid'])
    station = await get_station_by_id(sid)
    if not station:
        return httpmod.error('子站不存在', 404)
    my_role = None
    if user:
        my_role = await get_station_member_role(user['id'], sid)
    return httpmod.jsonify({'members': await get_station_members(sid), 'my_role': my_role})


async def edit(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    sid = int(params['sid'])
    station, err = await _require_owner(user, sid)
    if err:
        return err
    data = await httpmod.get_json_body(request) or {}
    if 'tags' in data and isinstance(data['tags'], list):
        data['tags'] = json.dumps(data['tags'], ensure_ascii=False)
    if not data:
        return httpmod.error('没有可更新的字段', 400)
    await update_station(sid, **data)
    return httpmod.jsonify({'message': '更新成功'})


async def delete(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    sid = int(params['sid'])
    station, err = await _require_owner(user, sid)
    if err:
        return err
    await delete_station(sid)
    return httpmod.jsonify({'message': '已删除'})


async def remove_member(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    sid = int(params['sid'])
    uid = int(params['uid'])
    station, err = await _require_owner(user, sid)
    if err:
        return err
    if uid == user['id']:
        return httpmod.error('不能移除自己', 400)
    if not await remove_station_member(uid, sid):
        return httpmod.error('成员不存在或为子站长', 400)
    return httpmod.jsonify({'message': '已移除'})


async def transfer(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    sid = int(params['sid'])
    station, err = await _require_owner(user, sid)
    if err:
        return err
    data = await httpmod.get_json_body(request) or {}
    new_owner_id = data.get('new_owner_id')
    if not new_owner_id:
        return httpmod.error('缺少新子站长ID', 400)
    try:
        new_owner_id = int(new_owner_id)
    except (ValueError, TypeError):
        return httpmod.error('无效的子站长ID', 400)
    if not await is_station_member(new_owner_id, sid):
        return httpmod.error('新子站长必须是子站成员', 400)
    await transfer_station_ownership(sid, new_owner_id)
    return httpmod.jsonify({'message': '转让成功'})


ROUTES = [
    (HTTPMethod.GET, r'^/api/stations$', list_stations),
    (HTTPMethod.GET, r'^/api/stations/categories$', categories),
    (HTTPMethod.POST, r'^/api/stations$', create),
    (HTTPMethod.GET, r'^/api/stations/(?P<sid>\d+)$', get_station),
    (HTTPMethod.PUT, r'^/api/stations/(?P<sid>\d+)$', edit),
    (HTTPMethod.DELETE, r'^/api/stations/(?P<sid>\d+)$', delete),
    (HTTPMethod.POST, r'^/api/stations/(?P<sid>\d+)/join$', join),
    (HTTPMethod.POST, r'^/api/stations/(?P<sid>\d+)/leave$', leave),
    (HTTPMethod.GET, r'^/api/stations/(?P<sid>\d+)/posts$', station_posts),
    (HTTPMethod.GET, r'^/api/stations/(?P<sid>\d+)/stats$', station_stats),
    (HTTPMethod.GET, r'^/api/stations/(?P<sid>\d+)/members$', members),
    (HTTPMethod.DELETE, r'^/api/stations/(?P<sid>\d+)/members/(?P<uid>\d+)$', remove_member),
    (HTTPMethod.POST, r'^/api/stations/(?P<sid>\d+)/transfer$', transfer),
    (HTTPMethod.GET, r'^/api/stations/search$', search),
    (HTTPMethod.GET, r'^/api/stations/mine$', my_stations),
    (HTTPMethod.GET, r'^/api/stations/by/(?P<uid>\d+)$', user_stations),
    (HTTPMethod.POST, r'^/api/stations/upload-cover$', upload_cover),
]