"""帖子路由：列表 / 详情 / 发布 / 编辑 / 删除 / 点赞 / 评论 / 上传图片。"""
import json

import web as httpmod
import auth as authmod
import db as _db
from models import (
    create_post, get_post_by_id, get_posts, get_post_count,
    update_post, delete_post, increment_views, get_liked_posts,
    record_post_version, get_post_versions,
    create_comment, get_comments, delete_comment,
    toggle_like, is_liked, create_notification, get_station_by_id,
    is_station_member,
    parse_post_extra, build_post_extra, shape_post,
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


def _anonymize_post(post, viewer=None):
    """匿名帖：向普通观众隐藏作者真实身份。"""
    if not post:
        return post
    if post.get('is_anonymous'):
        is_owner = viewer and post.get('author_id') == viewer['id']
        is_admin = viewer and viewer.get('role') == 'admin'
        if not is_owner and not is_admin:
            post['author_name'] = '匿名用户'
            post['author_avatar'] = '/static/images/default-avatar.svg'
    return post


ALLOWED_POST_TYPES = ('text', 'image', 'link', 'vote')


def _parse_extra(post):
    return parse_post_extra(post)


def _build_extra(post_type, data):
    return build_post_extra(post_type, data)


def _shape_post(post, viewer=None):
    """models.shape_post 统一实现（含匿名脱敏 + vote/link 展开）。"""
    return shape_post(post, viewer)


async def vote(request, params):
    """POST /api/posts/<pid>/vote  {option_index:int}，一人一票。"""
    user, resp = await _require_user(request)
    if resp:
        return resp
    pid = int(params['pid'])
    post = await get_post_by_id(pid)
    if not post:
        return httpmod.error('帖子不存在', 404)
    if post.get('post_type') != 'vote':
        return httpmod.error('该帖子不是投票帖', 400)
    extra = _parse_extra(post)
    options = extra.get('options') or []
    if not options:
        return httpmod.error('投票选项缺失', 400)
    data = await httpmod.get_json_body(request) or {}
    try:
        idx = int(data.get('option_index'))
    except (TypeError, ValueError):
        return httpmod.error('选项无效', 400)
    if idx < 0 or idx >= len(options):
        return httpmod.error('选项无效', 400)
    voters = extra.get('voters') or {}
    if str(user['id']) in voters:
        return httpmod.error('你已经投过票了', 400)
    counts = extra.get('counts') or {}
    counts[str(idx)] = int(counts.get(str(idx), 0)) + 1
    voters[str(user['id'])] = idx
    extra['counts'] = counts
    extra['voters'] = voters
    await _db.execute('UPDATE posts SET extra = ? WHERE id = ?',
                      (json.dumps(extra, ensure_ascii=False), pid))
    shaped = _shape_post(dict(post, extra=json.dumps(extra, ensure_ascii=False)), user)
    return httpmod.jsonify({'message': '投票成功',
                            'vote_counts': shaped.get('vote_counts'),
                            'user_voted': True})


async def liked_posts(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    q = httpmod.get_query_params(request)
    limit = int(q.get('limit', 50))
    offset = int(q.get('offset', 0))
    posts = await get_liked_posts(user['id'], limit, offset)
    for p in posts:
        p['is_liked'] = True
        _anonymize_post(p, user)
        _shape_post(p, user)
    return httpmod.jsonify({'posts': posts})


async def list_posts(request, params):
    user = await _optional_user(request)
    q = httpmod.get_query_params(request)
    limit = int(q.get('limit', 20))
    offset = int(q.get('offset', 0))
    sort = q.get('sort', 'newest')
    station_id = int(q['station_id']) if q.get('station_id') and q['station_id'].isdigit() else None
    author_id = int(q['author_id']) if q.get('author_id') and q['author_id'].isdigit() else None
    post_type = q.get('type')

    posts = await get_posts(station_id=station_id, author_id=author_id,
                            limit=limit, offset=offset, sort=sort, post_type=post_type)
    total = await get_post_count(station_id=station_id, author_id=author_id, post_type=post_type)

    for p in posts:
        if user:
            p['is_liked'] = await is_liked(user['id'], 'post', p['id'])
        _anonymize_post(p, user)
        _shape_post(p, user)
    return httpmod.jsonify({'posts': posts, 'total': total})


async def post_versions(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    pid = int(params['pid'])
    post = await get_post_by_id(pid)
    if not post:
        return httpmod.error('帖子不存在', 404)
    if post['author_id'] != user['id'] and user['role'] != 'admin':
        return httpmod.error('无权查看', 403)
    versions = await get_post_versions(pid)
    return httpmod.jsonify({'versions': versions})


async def get_post(request, params):
    user = await _optional_user(request)
    pid = int(params['pid'])
    post = await get_post_by_id(pid)
    if not post:
        return httpmod.error('帖子不存在', 404)
    await increment_views(pid)
    post['views'] += 1
    post['is_liked'] = await is_liked(user['id'], 'post', pid) if user else False
    _anonymize_post(post, user)
    _shape_post(post, user)
    return httpmod.jsonify(post)


async def create(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    data = await httpmod.get_json_body(request) or {}
    title = (data.get('title') or '').strip()
    content = (data.get('content') or '').strip()
    station_id = data.get('station_id')
    if station_id is not None:
        try:
            station_id = int(station_id)
        except (ValueError, TypeError):
            station_id = None
    image = data.get('image', '')
    images = data.get('images', [])
    is_anonymous = data.get('is_anonymous', 0)

    if not title:
        return httpmod.error('标题不能为空', 400)
    if len(title) > 100:
        return httpmod.error('标题最大100个字符', 400)
    if not content:
        return httpmod.error('内容不能为空', 400)
    if len(content) > 10000:
        return httpmod.error('内容最大10000个字符', 400)
    if not station_id:
        return httpmod.error('请选择子站', 400)

    station = await get_station_by_id(station_id)
    if not station:
        return httpmod.error('子站不存在', 404)

    if station.get('only_owner_posts') and station.get('owner_id') != user['id']:
        return httpmod.error('该子站仅站长可发帖', 403)

    if station.get('is_public') == 0:
        if not await is_station_member(user['id'], station_id):
            return httpmod.error('私密子站仅成员可发帖，请先加入', 403)

    post_type = data.get('post_type', 'text')
    if post_type not in ALLOWED_POST_TYPES:
        post_type = 'text'
    extra = _build_extra(post_type, data)
    if post_type == 'vote' and 'options' not in extra:
        return httpmod.error('投票至少需要2个选项', 400)
    if post_type == 'link' and not extra.get('link_url'):
        return httpmod.error('请填写有效链接', 400)
    pid = await create_post(
        title, content, user['id'], station_id, image,
        is_anonymous=1 if is_anonymous else 0,
        post_type=post_type,
        images=images if isinstance(images, list) else [],
        extra=extra,
    )
    if not pid:
        return httpmod.error('发帖失败', 500)

    post = await get_post_by_id(pid)
    _anonymize_post(post, user)
    _shape_post(post, user)
    return httpmod.jsonify({'message': '发帖成功', 'post': post}, status=201)


async def update(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    pid = int(params['pid'])
    post = await get_post_by_id(pid)
    if not post:
        return httpmod.error('帖子不存在', 404)
    if post['author_id'] != user['id'] and user['role'] != 'admin':
        return httpmod.error('无权编辑', 403)

    data = await httpmod.get_json_body(request) or {}
    update_fields = {k: data[k] for k in ('title', 'content', 'image') if k in data}
    if 'images' in data and isinstance(data['images'], list):
        update_fields['images'] = json.dumps(data['images'], ensure_ascii=False)
    # 编辑时允许重建 link/vote 载荷；选项变化则重置票数
    if post.get('post_type') in ('link', 'vote') and ('link_url' in data or 'vote_options' in data):
        new_extra = _build_extra(post['post_type'], data)
        old_extra = _parse_extra(post)
        if post['post_type'] == 'vote' and new_extra.get('options') == old_extra.get('options'):
            new_extra['counts'] = old_extra.get('counts') or {}
            new_extra['voters'] = old_extra.get('voters') or {}
        update_fields['extra'] = json.dumps(new_extra, ensure_ascii=False)
    if 'is_pinned' in data:
        if user['role'] != 'admin':
            return httpmod.error('无权置顶帖子', 403)
        update_fields['is_pinned'] = data['is_pinned']
    if not update_fields:
        return httpmod.error('没有可更新的字段', 400)

    if any(k in update_fields for k in ('title', 'content', 'image')):
        await record_post_version(pid, post['title'], post['content'],
                                  post.get('image') or '', user['id'])
    await update_post(pid, **update_fields)
    return httpmod.jsonify({'message': '更新成功'})


async def delete(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    pid = int(params['pid'])
    post = await get_post_by_id(pid)
    if not post:
        return httpmod.error('帖子不存在', 404)
    if post['author_id'] != user['id'] and user['role'] != 'admin':
        return httpmod.error('无权删除', 403)
    await delete_post(pid)
    return httpmod.jsonify({'message': '已删除'})


async def like_post(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    pid = int(params['pid'])
    post = await get_post_by_id(pid)
    if not post:
        return httpmod.error('帖子不存在', 404)
    liked = await toggle_like(user['id'], 'post', pid)
    if liked and post['author_id'] != user['id']:
        await create_notification(
            post['author_id'], user['id'], 'like',
            f'{user["username"]} 赞了你的帖子《{post["title"]}》。',
            f'/post/{pid}')
    return httpmod.jsonify({'liked': liked, 'message': '已点赞' if liked else '已取消点赞'})


async def list_comments(request, params):
    user = await _optional_user(request)
    pid = int(params['pid'])
    post = await get_post_by_id(pid)
    if not post:
        return httpmod.error('帖子不存在', 404)
    q = httpmod.get_query_params(request)
    limit = int(q.get('limit', 50))
    offset = int(q.get('offset', 0))
    comments = await get_comments(pid, limit=limit, offset=offset)
    for c in comments:
        c['is_liked'] = await is_liked(user['id'], 'comment', c['id']) if user else False
    return httpmod.jsonify(comments)


async def add_comment(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    pid = int(params['pid'])
    post = await get_post_by_id(pid)
    if not post:
        return httpmod.error('帖子不存在', 404)

    data = await httpmod.get_json_body(request) or {}
    content = (data.get('content') or '').strip()
    parent_id = data.get('parent_id')

    if not content:
        return httpmod.error('评论不能为空', 400)
    if len(content) > 2000:
        return httpmod.error('评论最大2000个字符', 400)

    cid = await create_comment(content, user['id'], pid, parent_id)
    if not cid:
        return httpmod.error('评论失败', 500)

    if post['author_id'] != user['id']:
        await create_notification(
            post['author_id'], user['id'], 'comment',
            f'{user["username"]} 评论了你的帖子《{post["title"]}》。',
            f'/post/{pid}')

    return httpmod.jsonify({'message': '评论成功', 'comment_id': cid}, status=201)


async def upload_image(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    url, err = await save_upload(request, prefix='post', subdir='posts')
    if err:
        return httpmod.error(err, 400)
    return httpmod.jsonify({'url': url})


async def del_comment(request, params):
    user, resp = await _require_user(request)
    if resp:
        return resp
    cid = int(params['cid'])
    comment = await _db.query('SELECT author_id FROM comments WHERE id = ?', (cid,), one=True)
    if not comment:
        return httpmod.error('评论不存在', 404)
    if comment['author_id'] != user['id'] and user['role'] != 'admin':
        return httpmod.error('无权删除', 403)
    await delete_comment(cid)
    return httpmod.jsonify({'message': '已删除'})


ROUTES = [
    (HTTPMethod.GET, r'^/api/posts/liked$', liked_posts),
    (HTTPMethod.GET, r'^/api/posts$', list_posts),
    (HTTPMethod.GET, r'^/api/posts/(?P<pid>\d+)/versions$', post_versions),
    (HTTPMethod.GET, r'^/api/posts/(?P<pid>\d+)$', get_post),
    (HTTPMethod.POST, r'^/api/posts$', create),
    (HTTPMethod.POST, r'^/api/posts/(?P<pid>\d+)/vote$', vote),
    (HTTPMethod.PUT, r'^/api/posts/(?P<pid>\d+)$', update),
    (HTTPMethod.DELETE, r'^/api/posts/(?P<pid>\d+)$', delete),
    (HTTPMethod.POST, r'^/api/posts/(?P<pid>\d+)/like$', like_post),
    (HTTPMethod.GET, r'^/api/posts/(?P<pid>\d+)/comments$', list_comments),
    (HTTPMethod.POST, r'^/api/posts/(?P<pid>\d+)/comments$', add_comment),
    (HTTPMethod.POST, r'^/api/posts/upload-image$', upload_image),
    (HTTPMethod.DELETE, r'^/api/posts/comments/(?P<cid>\d+)$', del_comment),
]