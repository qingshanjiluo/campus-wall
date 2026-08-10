import os
import uuid
from flask import Blueprint, request, jsonify, g, current_app
from app.models import (
    create_post, get_post_by_id, get_posts, get_post_count,
    update_post, delete_post, increment_views,
    create_comment, get_comments, delete_comment,
    toggle_like, is_liked, create_notification, get_station_by_id
)
from app.utils.auth import token_required, optional_auth

posts_bp = Blueprint('posts', __name__)


def _anonymize_post(post, viewer=None):
    """匿名帖：向普通观众隐藏作者真实身份"""
    if not post:
        return post
    if post.get('is_anonymous'):
        is_owner = viewer and post.get('author_id') == viewer['id']
        is_admin = viewer and viewer.get('role') == 'admin'
        if not is_owner and not is_admin:
            post['author_name'] = '匿名用户'
            post['author_avatar'] = '/static/images/default-avatar.svg'
    return post


@posts_bp.route('', methods=['GET'])
@optional_auth
def list_posts():
    limit = request.args.get('limit', 20, type=int)
    offset = request.args.get('offset', 0, type=int)
    sort = request.args.get('sort', 'newest')
    station_id = request.args.get('station_id', type=int)
    author_id = request.args.get('author_id', type=int)
    post_type = request.args.get('type')

    posts = get_posts(station_id=station_id, author_id=author_id, limit=limit, offset=offset, sort=sort, post_type=post_type)
    total = get_post_count(station_id=station_id, author_id=author_id, post_type=post_type)

    if g.current_user:
        for p in posts:
            p['is_liked'] = is_liked(g.current_user['id'], 'post', p['id'])
            _anonymize_post(p, g.current_user)
    else:
        for p in posts:
            _anonymize_post(p)
    return jsonify({'posts': posts, 'total': total})


@posts_bp.route('/<int:pid>', methods=['GET'])
@optional_auth
def get_post(pid):
    post = get_post_by_id(pid)
    if not post:
        return jsonify({'error': '帖子不存在'}), 404
    increment_views(pid)
    post['views'] += 1
    if g.current_user:
        post['is_liked'] = is_liked(g.current_user['id'], 'post', pid)
    else:
        post['is_liked'] = False
    _anonymize_post(post, g.current_user)
    return jsonify(post)


@posts_bp.route('', methods=['POST'])
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

    pid = create_post(title, content, g.current_user['id'], station_id, image,
                      is_anonymous=1 if is_anonymous else 0)
    if not pid:
        return jsonify({'error': '发帖失败'}), 500

    post = get_post_by_id(pid)
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
    # 置顶/加精仅管理员可操作，防止普通用户越权
    if 'is_pinned' in data:
        if g.current_user['role'] != 'admin':
            return jsonify({'error': '无权置顶帖子'}), 403
        update_fields['is_pinned'] = data['is_pinned']
    if not update_fields:
        return jsonify({'error': '没有可更新的字段'}), 400
    update_post(pid, **update_fields)
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
    return jsonify({'message': '已删除'})


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
    return jsonify({'liked': liked, 'message': '已点赞' if liked else '已取消点赞'})


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
        for c in comments:
            c['is_liked'] = is_liked(g.current_user['id'], 'comment', c['id'])
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
