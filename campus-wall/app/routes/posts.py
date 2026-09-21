import os
import uuid
from flask import Blueprint, request, jsonify, g, current_app
from app.models import (
    create_post, get_post_by_id, get_posts, get_post_count,
    query_db, execute_db,
    update_post, delete_post, increment_views, get_liked_posts,
    record_post_version, get_post_versions,
    create_comment, get_comments, delete_comment,
    toggle_like, is_liked, is_liked_batch, create_notification, get_station_by_id,
    ALLOWED_POST_TYPES, build_post_extra, shape_post, cast_vote, parse_post_extra
)
from app.utils.auth import token_required, optional_auth
from app.utils.limiter import limiter
from app.utils.media import finalize_upload
from app.utils.sensitive import scan_text
from app.models_ext import attach_post_topics, get_topics_for_posts, get_post_topics, normalize_topics, visitor_mode_open


def _visitor_gate():
    """访客模式（R9）：closed 时未登录用户不能浏览内容，返回 401 响应或 None。"""
    if g.get('current_user') is None and not visitor_mode_open():
        return jsonify({'error': '当前为登录可见模式，请先登录'}), 401
    return None

posts_bp = Blueprint('posts', __name__)


@posts_bp.route('/liked', methods=['GET'])
@token_required
def liked_posts():
    limit = request.args.get('limit', 50, type=int)
    offset = request.args.get('offset', 0, type=int)
    posts = get_liked_posts(g.current_user['id'], limit, offset)
    for p in posts:
        p['is_liked'] = True
        _anonymize_post(p, g.current_user)
    return jsonify({'posts': posts})


def _anonymize_post(post, viewer=None):
    """兼容旧名：匿名脱敏已并入 shape_post（同时展开 vote/link 载荷）。"""
    return shape_post(post, viewer)


@posts_bp.route('', methods=['GET'])
@optional_auth
def list_posts():
    gate = _visitor_gate()
    if gate:
        return gate
    limit = request.args.get('limit', 20, type=int)
    offset = request.args.get('offset', 0, type=int)
    sort = request.args.get('sort', 'newest')
    station_id = request.args.get('station_id', type=int)
    author_id = request.args.get('author_id', type=int)
    # 参数名兼容：type 为契约正名；post_type 为历史别名（/expose 页曾用它导致筛选失效）
    post_type = request.args.get('type') or request.args.get('post_type')

    posts = get_posts(station_id=station_id, author_id=author_id, limit=limit, offset=offset, sort=sort, post_type=post_type)
    total = get_post_count(station_id=station_id, author_id=author_id, post_type=post_type)

    liked = is_liked_batch(g.current_user['id'], 'post', [p['id'] for p in posts]) if (g.current_user and posts) else set()
    tmap = get_topics_for_posts([p['id'] for p in posts]) if posts else {}
    for p in posts:
        if g.current_user:
            p['is_liked'] = p['id'] in liked
        p['topics'] = tmap.get(p['id'], [])
        shape_post(p, g.current_user)
    return jsonify({'posts': posts, 'total': total})


@posts_bp.route('/<int:pid>/versions', methods=['GET'])
@token_required
def post_versions(pid):
    post = get_post_by_id(pid)
    if not post:
        return jsonify({'error': '帖子不存在'}), 404
    if post['author_id'] != g.current_user['id'] and g.current_user['role'] != 'admin':
        return jsonify({'error': '无权查看'}), 403
    return jsonify({'versions': get_post_versions(pid)})


@posts_bp.route('/<int:pid>', methods=['GET'])
@optional_auth
def get_post(pid):
    gate = _visitor_gate()
    if gate:
        return gate
    post = get_post_by_id(pid)
    if not post:
        return jsonify({'error': '帖子不存在'}), 404
    # 审核闸：pending/rejected 仅作者本人与管理员可见（对外表现为不存在，不泄露审核状态）
    if post.get('status') in ('pending', 'rejected'):
        is_owner = g.current_user and post['author_id'] == g.current_user['id']
        is_admin = g.current_user and g.current_user['role'] == 'admin'
        if not (is_owner or is_admin):
            return jsonify({'error': '帖子不存在'}), 404
    increment_views(pid)
    post['views'] += 1
    if g.current_user:
        post['is_liked'] = is_liked(g.current_user['id'], 'post', pid)
    else:
        post['is_liked'] = False
    post['topics'] = get_post_topics(pid)
    _anonymize_post(post, g.current_user)
    return jsonify(post)


@posts_bp.route('', methods=['POST'])
@limiter.limit('20/minute')
@token_required
def create():
    data = request.get_json(silent=True) or {}
    title = (data.get('title') or '').strip()
    content = (data.get('content') or '').strip()
    station_id = data.get('station_id')
    if station_id is not None:
        try:
            station_id = int(station_id)
        except (ValueError, TypeError):
            station_id = None
    image = data.get('image', '')
    is_anonymous = data.get('is_anonymous', 0)
    post_type = data.get('post_type', 'text')
    if post_type not in ALLOWED_POST_TYPES:
        post_type = 'text'
    # 爆料帖恒匿名 + 落到默认公开子站（无需前端指定）
    is_expose = (post_type == 'expose')
    if is_expose:
        is_anonymous = 1
        if not station_id:
            from app.models_ext import get_first_public_station
            station_id = get_first_public_station()

    if not title:
        return jsonify({'error': '标题不能为空'}), 400
    if len(title) > 100:
        return jsonify({'error': '标题最多100个字符'}), 400
    if not content:
        return jsonify({'error': '内容不能为空'}), 400
    if len(content) > 10000:
        return jsonify({'error': '内容最多10000个字符'}), 400
    if not station_id:
        return jsonify({'error': '请选择子站'}), 400

    station = get_station_by_id(station_id)
    if not station:
        return jsonify({'error': '子站不存在'}), 404

    # 仅站长可发帖的子站，普通成员/游客禁止发帖
    if station.get('only_owner_posts') and station.get('owner_id') != g.current_user['id']:
        return jsonify({'error': '该子站仅站长可发帖'}), 403

    # 私密切入限制：非成员不能发帖
    if station.get('is_public') == 0:
        from app.models import is_station_member
        if not is_station_member(g.current_user['id'], station_id):
            return jsonify({'error': '私密子站仅成员可发帖，请先加入'}), 403

    extra = build_post_extra(post_type, data)
    if post_type == 'vote' and 'options' not in extra:
        return jsonify({'error': '投票帖至少需要2个选项'}), 400
    if post_type == 'link' and not extra.get('link_url'):
        return jsonify({'error': '请填写有效链接'}), 400

    # 敏感词三级判定：block 拒绝 / review 进人工审核队列 / 否则直接发布
    # （爆料帖恒走 pending 人工审核，先审后发，防匿名诽谤）
    level = scan_text(title) or scan_text(content)
    if level == 'block':
        return jsonify({'error': '内容包含违规信息，已被拒绝发布'}), 400
    status = 'pending' if (level == 'review' or is_expose) else 'approved'

    pid = create_post(title, content, g.current_user['id'], station_id, image,
                      is_anonymous=1 if is_anonymous else 0,
                      post_type=post_type,
                      images=data.get('images') if isinstance(data.get('images'), list) else [],
                      extra=extra, status=status)
    if not pid:
        return jsonify({'error': '发帖失败'}), 500

    # 付费内容（R11 暗阁）：0-999 积分，爆料帖不可付费
    pay_points = data.get('pay_points')
    if isinstance(pay_points, (int, float)) and not is_expose:
        pay_points = max(0, min(int(pay_points), 999))
        if pay_points > 0:
            execute_db('UPDATE posts SET pay_points = ? WHERE id = ?', (pay_points, pid))

    # 话题：清洗→敏感词 block 拒→挂载（上限 5 个，编辑即替换）
    topics = normalize_topics(data.get('topics') if isinstance(data.get('topics'), list) else [])
    if topics:
        for t in topics:
            if scan_text(t) == 'block':
                delete_post(pid)
                return jsonify({'error': f'话题「{t}」包含违规信息'}), 400
        attach_post_topics(pid, topics, g.current_user['id'])

    if status == 'pending':
        return jsonify({'message': '已提交，内容正在审核，通过后自动展示', 'post_id': pid, 'status': 'pending'}), 202

    post = get_post_by_id(pid)
    if topics:
        post['topics'] = topics
    _anonymize_post(post, g.current_user)
    return jsonify({'message': '发帖成功', 'post': post}), 201


@posts_bp.route('/<int:pid>', methods=['PUT'])
@token_required
def update(pid):
    post = get_post_by_id(pid)
    if not post:
        return jsonify({'error': '帖子不存在'}), 404
    if post['author_id'] != g.current_user['id'] and g.current_user['role'] != 'admin':
        return jsonify({'error': '无权编辑'}), 403

    data = request.get_json(silent=True) or {}
    update_fields = {k: data[k] for k in ('title', 'content', 'image') if k in data}
    if 'images' in data and isinstance(data['images'], list):
        import json as _json
        update_fields['images'] = _json.dumps(data['images'], ensure_ascii=False)
    # 编辑时允许重建 link/vote 载荷；选项未变则保留票数（与 Worker 语义一致）
    if post.get('post_type') in ('link', 'vote') and ('link_url' in data or 'vote_options' in data):
        import json as _json
        new_extra = build_post_extra(post['post_type'], data)
        old_extra = parse_post_extra(post)
        if post['post_type'] == 'vote' and new_extra.get('options') == old_extra.get('options'):
            new_extra['counts'] = old_extra.get('counts') or {}
            new_extra['voters'] = old_extra.get('voters') or {}
        update_fields['extra'] = _json.dumps(new_extra, ensure_ascii=False)
    # 置顶/加精仅管理员可操作，防止普通用户越权
    if 'is_pinned' in data:
        if g.current_user['role'] != 'admin':
            return jsonify({'error': '无权置顶帖子'}), 403
        update_fields['is_pinned'] = data['is_pinned']
    # 话题编辑＝整体替换（传了 topics 字段才动；否则保留原关联）。
    # 注意需在「无可更新字段」判定之前处理：只传 topics 的编辑也是合法更新。
    if isinstance(data.get('topics'), list):
        topics = normalize_topics(data['topics'])
        for t in topics:
            if scan_text(t) == 'block':
                return jsonify({'error': f'话题「{t}」包含违规信息'}), 400
        attach_post_topics(pid, topics, g.current_user['id'])
        if not update_fields:
            return jsonify({'message': '更新成功'})
    if not update_fields:
        return jsonify({'error': '没有可更新的字段'}), 400
    # 防"先发干净帖再编辑塞违规内容"绕过：内容编辑一律复扫；
    # review 级命中或作者修改被驳回帖 → 重新进审核队列
    if 'title' in update_fields or 'content' in update_fields:
        _t = update_fields.get('title', post['title'])
        _c = update_fields.get('content', post['content'])
        level = scan_text(_t) or scan_text(_c)
        if level == 'block':
            return jsonify({'error': '内容包含违规信息，无法保存'}), 400
        if level == 'review' or post.get('status') == 'rejected':
            update_fields['status'] = 'pending'
    # 记录编辑历史（保存编辑前的旧版本）
    if any(k in update_fields for k in ('title', 'content', 'image')):
        record_post_version(pid, post['title'], post['content'], post.get('image') or '', g.current_user['id'])
    update_post(pid, **update_fields)
    if update_fields.get('status') == 'pending':
        return jsonify({'message': '已保存，内容重新进入审核'})
    return jsonify({'message': '更新成功'})


@posts_bp.route('/<int:pid>', methods=['DELETE'])
@token_required
def delete(pid):
    post = get_post_by_id(pid)
    if not post:
        return jsonify({'error': '帖子不存在'}), 404
    if post['author_id'] != g.current_user['id'] and g.current_user['role'] != 'admin':
        return jsonify({'error': '无权删除'}), 403
    delete_post(pid)
    execute_db('DELETE FROM post_topics WHERE post_id = ?', (pid,))
    return jsonify({'message': '已删除'})


@posts_bp.route('/<int:pid>/unlock', methods=['POST'])
@token_required
def unlock_post_route(pid):
    """付费内容解锁（R11 暗阁）：买家付积分，作者收款。"""
    from app.models_ext import unlock_post
    post = get_post_by_id(pid)
    if not post or post['is_deleted']:
        return jsonify({'error': '帖子不存在'}), 404
    if post['author_id'] == g.current_user['id']:
        return jsonify({'error': '自己的帖子无需解锁'}), 400
    res, err = unlock_post(g.current_user['id'], post)
    if err:
        return jsonify({'error': err}), 400
    return jsonify({'message': f"已解锁，消耗 {res['paid']} 积分"})


@posts_bp.route('/<int:pid>/boost', methods=['POST'])
@token_required
def boost_post_route(pid):
    """积分推流（R11 暗阁）：作者花 10 积分/天置前推荐流 1-7 天。"""
    from app.models_ext import boost_post
    post = get_post_by_id(pid)
    if not post or post['is_deleted']:
        return jsonify({'error': '帖子不存在'}), 404
    if post['author_id'] != g.current_user['id'] and g.current_user['role'] != 'admin':
        return jsonify({'error': '仅作者可推流自己的帖子'}), 403
    data = request.get_json(silent=True) or {}
    res, err = boost_post(pid, g.current_user['id'], data.get('days', 1))
    if err:
        return jsonify({'error': err}), 400
    return jsonify({'message': f"已推流 {res['days']} 天（消耗 {res['cost']} 积分）", **res})


@posts_bp.route('/<int:pid>/like', methods=['POST'])
@token_required
def like_post(pid):
    post = get_post_by_id(pid)
    if not post:
        return jsonify({'error': '帖子不存在'}), 404
    liked = toggle_like(g.current_user['id'], 'post', pid)
    if liked and post['author_id'] != g.current_user['id']:
        create_notification(
            post['author_id'], g.current_user['id'], 'like',
            f'{g.current_user["username"]} 赞了你的帖子「{post["title"]}」',
            f'/post/{pid}'
        )
    # 返回真实计数，前端不再靠解析按钮文本（P2-9）
    row = query_db('SELECT likes_count FROM posts WHERE id = ?', (pid,), one=True)
    return jsonify({'liked': liked, 'likes_count': (row or {}).get('likes_count', 0),
                    'message': '已点赞' if liked else '已取消点赞'})


@posts_bp.route('/<int:pid>/vote', methods=['POST'])
@token_required
def vote_post(pid):
    """POST /api/posts/<pid>/vote {option_index:int}：一人一票。"""
    post = get_post_by_id(pid)
    if not post:
        return jsonify({'error': '帖子不存在'}), 404
    if post.get('post_type') != 'vote':
        return jsonify({'error': '该帖子不是投票帖'}), 400
    data = request.get_json(silent=True) or {}
    try:
        idx = int(data.get('option_index'))
    except (TypeError, ValueError):
        return jsonify({'error': '选项无效'}), 400
    extra, err = cast_vote(post, g.current_user['id'], idx)
    if err:
        return jsonify({'error': err}), 400
    shaped = shape_post(dict(post, extra=__import__('json').dumps(extra, ensure_ascii=False)), g.current_user)
    return jsonify({'message': '投票成功',
                    'vote_counts': shaped.get('vote_counts'),
                    'user_voted': True})


@posts_bp.route('/<int:pid>/comments', methods=['GET'])
@optional_auth
def list_comments(pid):
    post = get_post_by_id(pid)
    if not post:
        return jsonify({'error': '帖子不存在'}), 404
    limit = request.args.get('limit', 50, type=int)
    offset = request.args.get('offset', 0, type=int)
    comments = get_comments(pid, limit=limit, offset=offset)
    if g.current_user:
        liked = is_liked_batch(g.current_user['id'], 'comment', [c['id'] for c in comments]) if comments else set()
        for c in comments:
            c['is_liked'] = c['id'] in liked
    return jsonify(comments)


@posts_bp.route('/<int:pid>/comments', methods=['POST'])
@token_required
def add_comment(pid):
    post = get_post_by_id(pid)
    if not post:
        return jsonify({'error': '帖子不存在'}), 404

    data = request.get_json(silent=True) or {}
    content = (data.get('content') or '').strip()
    parent_id = data.get('parent_id')

    if not content:
        return jsonify({'error': '评论不能为空'}), 400
    if len(content) > 2000:
        return jsonify({'error': '评论最多2000个字符'}), 400
    if scan_text(content):
        return jsonify({'error': '评论包含违规信息，请修改后再发布'}), 400

    cid = create_comment(content, g.current_user['id'], pid, parent_id)
    if not cid:
        return jsonify({'error': '评论失败'}), 500

    if post['author_id'] != g.current_user['id']:
        create_notification(
            post['author_id'], g.current_user['id'], 'comment',
            f'{g.current_user["username"]} 评论了你的帖子「{post["title"]}」',
            f'/post/{pid}'
        )

    return jsonify({'message': '评论成功', 'comment_id': cid}), 201


@posts_bp.route('/upload-image', methods=['POST'])
@limiter.limit('15/minute')
@token_required
def upload_image():
    if 'file' not in request.files:
        return jsonify({'error': '请选择文件'}), 400
    file = request.files['file']
    if not file.filename:
        return jsonify({'error': '请选择文件'}), 400

    ext = file.filename.rsplit('.', 1)[-1].lower() if '.' in file.filename else ''
    if ext not in ('png', 'jpg', 'jpeg', 'gif', 'webp'):
        return jsonify({'error': '不支持的格式'}), 400

    filename = f"post_{g.current_user['id']}_{uuid.uuid4().hex[:8]}.{ext}"
    filepath = os.path.join(current_app.config['UPLOAD_FOLDER'], filename)
    file.save(filepath)
    if not finalize_upload(filepath):
        return jsonify({'error': '图片内容无效'}), 400
    return jsonify({'url': f'/static/uploads/{filename}'})


@posts_bp.route('/comments/<int:cid>', methods=['DELETE'])
@token_required
def del_comment(cid):
    from app.models import query_db
    comment = query_db('SELECT author_id FROM comments WHERE id = ?', (cid,), one=True)
    if not comment:
        return jsonify({'error': '评论不存在'}), 404
    if comment['author_id'] != g.current_user['id'] and g.current_user['role'] != 'admin':
        return jsonify({'error': '无权删除'}), 403
    delete_comment(cid)
    return jsonify({'message': '已删除'})
