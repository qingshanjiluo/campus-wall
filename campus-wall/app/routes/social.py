from flask import Blueprint, request, jsonify, g
from app.models import (
    toggle_follow, is_following, get_followers, get_following,
    get_user_by_id, get_user_stats,
    get_notifications, get_unread_count, mark_notifications_read,
    create_notification, toggle_like, is_liked
)
from app.utils.auth import token_required

social_bp = Blueprint('social', __name__)


# ── Follow ──

@social_bp.route('/follow/<int:uid>', methods=['POST'])
@token_required
def follow(uid):
    if uid == g.current_user['id']:
        return jsonify({'error': '不能关注自己'}), 400
    target = get_user_by_id(uid)
    if not target:
        return jsonify({'error': '用户不存在'}), 404

    result = toggle_follow(g.current_user['id'], uid)
    if result is None:
        return jsonify({'error': '不能关注自己'}), 400
    if result:
        create_notification(
            uid, g.current_user['id'], 'follow',
            f'{g.current_user["username"]} 关注了你',
            f'/profile/{g.current_user["id"]}'
        )
    return jsonify({'following': result, 'message': '已关注' if result else '已取消关注'})


@social_bp.route('/followers/<int:uid>', methods=['GET'])
def followers(uid):
    limit = request.args.get('limit', 20, type=int)
    offset = request.args.get('offset', 0, type=int)
    return jsonify(get_followers(uid, limit, offset))


@social_bp.route('/following/<int:uid>', methods=['GET'])
def following(uid):
    limit = request.args.get('limit', 20, type=int)
    offset = request.args.get('offset', 0, type=int)
    return jsonify(get_following(uid, limit, offset))


@social_bp.route('/is-following/<int:uid>', methods=['GET'])
@token_required
def check_following(uid):
    return jsonify({'following': is_following(g.current_user['id'], uid)})


# ── Notifications ──

@social_bp.route('/notifications', methods=['GET'])
@token_required
def list_notifications():
    limit = request.args.get('limit', 20, type=int)
    offset = request.args.get('offset', 0, type=int)
    unread_only = request.args.get('unread', 'false') == 'true'
    notifs = get_notifications(g.current_user['id'], limit, offset, unread_only)
    unread = get_unread_count(g.current_user['id'])
    return jsonify({'notifications': notifs, 'unread_count': unread})


@social_bp.route('/notifications/read', methods=['POST'])
@token_required
def read_notifications():
    data = request.get_json(silent=True) or {}
    ids = data.get('ids')
    mark_notifications_read(g.current_user['id'], ids)
    return jsonify({'message': '已标记已读'})


@social_bp.route('/notifications/unread-count', methods=['GET'])
@token_required
def unread_count():
    count = get_unread_count(g.current_user['id'])
    return jsonify({'count': count})


# ── Comment likes ──

@social_bp.route('/comment/<int:cid>/like', methods=['POST'])
@token_required
def like_comment(cid):
    liked = toggle_like(g.current_user['id'], 'comment', cid)
    return jsonify({'liked': liked, 'message': '已点赞' if liked else '已取消点赞'})
