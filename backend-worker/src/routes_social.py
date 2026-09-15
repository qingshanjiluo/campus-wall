"""社交路由：关注 / 粉丝 / 通知 / 评论点赞。"""
import web as httpmod
import auth as authmod
from models import (
    toggle_follow, is_following, get_followers, get_following,
    get_user_by_id,
    get_notifications, get_unread_count, mark_notifications_read,
    create_notification, toggle_like,
)

from http import HTTPMethod


async def _require_user(request):
    user = await authmod.require_user(request)
    if not user:
        return None, httpmod.error('缺少认证令牌', 401)
    return user, None


async def follow(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    uid = int(params['uid'])
    if uid == user['id']:
        return httpmod.error('不能关注自己', 400)
    target = await get_user_by_id(uid)
    if not target:
        return httpmod.error('用户不存在', 404)

    result = await toggle_follow(user['id'], uid)
    if result is None:
        return httpmod.error('不能关注自己', 400)
    if result:
        await create_notification(
            uid, user['id'], 'follow',
            f'{user["username"]} 关注了你',
            f'/profile/{user["id"]}')
    return httpmod.jsonify({'following': result, 'message': '已关注' if result else '已取消关注'})


async def followers(request, params):
    uid = int(params['uid'])
    q = httpmod.get_query_params(request)
    limit = int(q.get('limit', 20))
    offset = int(q.get('offset', 0))
    return httpmod.jsonify(await get_followers(uid, limit, offset))


async def following(request, params):
    uid = int(params['uid'])
    q = httpmod.get_query_params(request)
    limit = int(q.get('limit', 20))
    offset = int(q.get('offset', 0))
    return httpmod.jsonify(await get_following(uid, limit, offset))


async def check_following(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    uid = int(params['uid'])
    return httpmod.jsonify({'following': await is_following(user['id'], uid)})


async def list_notifications(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    q = httpmod.get_query_params(request)
    limit = int(q.get('limit', 20))
    offset = int(q.get('offset', 0))
    unread_only = q.get('unread', 'false') == 'true'
    notifs = await get_notifications(user['id'], limit, offset, unread_only)
    unread = await get_unread_count(user['id'])
    return httpmod.jsonify({'notifications': notifs, 'unread_count': unread})


async def read_notifications(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    data = await httpmod.get_json_body(request) or {}
    ids = data.get('ids')
    await mark_notifications_read(user['id'], ids)
    return httpmod.jsonify({'message': '已标记已读'})


async def unread_count(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    count = await get_unread_count(user['id'])
    return httpmod.jsonify({'count': count})


async def like_comment(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    cid = int(params['cid'])
    liked = await toggle_like(user['id'], 'comment', cid)
    return httpmod.jsonify({'liked': liked, 'message': '已点赞' if liked else '已取消点赞'})


ROUTES = [
    (HTTPMethod.POST, r'^/api/social/follow/(?P<uid>\d+)$', follow),
    (HTTPMethod.GET, r'^/api/social/followers/(?P<uid>\d+)$', followers),
    (HTTPMethod.GET, r'^/api/social/following/(?P<uid>\d+)$', following),
    (HTTPMethod.GET, r'^/api/social/is-following/(?P<uid>\d+)$', check_following),
    (HTTPMethod.GET, r'^/api/social/notifications$', list_notifications),
    (HTTPMethod.POST, r'^/api/social/notifications/read$', read_notifications),
    (HTTPMethod.GET, r'^/api/social/notifications/unread-count$', unread_count),
    (HTTPMethod.POST, r'^/api/social/comment/(?P<cid>\d+)/like$', like_comment),
]