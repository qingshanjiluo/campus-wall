"""私信 API（R4-M4）：/api/dm —— 发送、会话列表、单线会话、未读总数。"""
from flask import Blueprint, request, jsonify, g
from app.models import get_user_by_id, create_notification
from app.models_ext import (
    send_dm, get_dm_threads, get_dm_thread, mark_dm_read, dm_unread_total
)
from app.utils.auth import token_required
from app.utils.limiter import limiter
from app.utils.sensitive import scan_text

dm_bp = Blueprint('dm', __name__)

MAX_DM_LEN = 1000


@dm_bp.route('', methods=['POST'])
@limiter.limit('30/minute')
@token_required
def send():
    """POST /api/dm {to:int, content:str}  私信（私聊场景敏感词命中即拒，不做队列）"""
    data = request.get_json(silent=True) or {}
    try:
        to_uid = int(data.get('to'))
    except (TypeError, ValueError):
        return jsonify({'error': '缺少收件人'}), 400
    content = (data.get('content') or '').strip()
    if not content:
        return jsonify({'error': '消息不能为空'}), 400
    if len(content) > MAX_DM_LEN:
        return jsonify({'error': f'消息最多{MAX_DM_LEN}个字符'}), 400
    if to_uid == g.current_user['id']:
        return jsonify({'error': '不能给自己发私信'}), 400
    peer = get_user_by_id(to_uid)
    if not peer:
        return jsonify({'error': '收件人不存在'}), 404
    if scan_text(content):
        return jsonify({'error': '消息包含违规信息，发送失败'}), 400

    mid = send_dm(g.current_user['id'], to_uid, content)
    create_notification(
        to_uid, g.current_user['id'], 'dm',
        f'{g.current_user["username"]} 给你发来一条私信',
        f'/messages?with={g.current_user["id"]}'
    )
    return jsonify({'message': '已发送', 'id': mid}), 201


@dm_bp.route('/threads', methods=['GET'])
@token_required
def threads():
    limit = request.args.get('limit', 30, type=int)
    return jsonify(get_dm_threads(g.current_user['id'], limit=min(limit, 50)))


@dm_bp.route('/unread', methods=['GET'])
@token_required
def unread():
    return jsonify({'count': dm_unread_total(g.current_user['id'])})


@dm_bp.route('/<int:peer_id>', methods=['GET'])
@token_required
def thread(peer_id):
    """单线会话：返回正序消息 + 对方资料，并顺带把对方来信标记已读。"""
    if not get_user_by_id(peer_id):
        return jsonify({'error': '用户不存在'}), 404
    limit = request.args.get('limit', 100, type=int)
    msgs = list(reversed(get_dm_thread(g.current_user['id'], peer_id, limit=min(limit, 200))))
    mark_dm_read(g.current_user['id'], peer_id)
    peer = get_user_by_id(peer_id)
    return jsonify({
        'peer': {
            'id': peer['id'], 'username': peer['username'], 'avatar': peer['avatar'],
            'identity_group': peer.get('identity_group') or ''
        },
        'messages': msgs
    })
