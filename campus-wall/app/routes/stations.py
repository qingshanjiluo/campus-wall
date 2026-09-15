import json
import os
import uuid
from flask import Blueprint, request, jsonify, g, current_app
from app.models import (
    create_station, get_stations, get_station_by_id, search_stations,
    join_station, leave_station, is_station_member, get_station_member_role,
    get_station_membership, get_station_memberships,
    get_station_members, get_user_stations, remove_station_member, transfer_station_ownership,
    update_station, delete_station, get_station_stats, get_station_categories,
    get_posts, get_post_count, shape_post, is_liked_batch
)
from app.utils.auth import token_required, optional_auth

stations_bp = Blueprint('stations', __name__)


def _require_owner(sid):
    """校验当前用户是否为子站 owner，返回 (error_response, None) 或 (None, station)"""
    station = get_station_by_id(sid)
    if not station:
        return (jsonify({'error': '子站不存在'}), 404), None
    role = get_station_member_role(g.current_user['id'], sid)
    if role != 'owner' and g.current_user['role'] != 'admin':
        return (jsonify({'error': '仅子站长可操作'}), 403), None
    return None, station


@stations_bp.route('', methods=['GET'])
@optional_auth
def list_stations():
    limit = request.args.get('limit', 20, type=int)
    offset = request.args.get('offset', 0, type=int)
    sort = request.args.get('sort', 'newest')
    tag = request.args.get('tag', '').strip() or None
    category = request.args.get('category', '').strip() or None

    stations = get_stations(limit=limit, offset=offset, sort=sort, tag=tag, category=category)
    roles = get_station_memberships(g.current_user['id'], [s['id'] for s in stations]) if (g.current_user and stations) else {}
    for s in stations:
        s['tags'] = json.loads(s['tags']) if s['tags'] else []
        role = roles.get(s['id'])
        s['is_member'] = role is not None
        s['is_owner'] = role == 'owner'
    return jsonify(stations)


@stations_bp.route('/categories', methods=['GET'])
def categories():
    return jsonify(get_station_categories())


@stations_bp.route('/<int:sid>', methods=['GET'])
@optional_auth
def get_station(sid):
    station = get_station_by_id(sid)
    if not station:
        return jsonify({'error': '子站不存在'}), 404
    station['tags'] = json.loads(station['tags']) if station['tags'] else []
    if g.current_user:
        role = get_station_membership(g.current_user['id'], sid)
        station['is_member'] = role is not None
        station['is_owner'] = role == 'owner'
    else:
        station['is_member'] = False
        station['is_owner'] = False
    return jsonify(station)


@stations_bp.route('', methods=['POST'])
@token_required
def create():
    data = request.get_json(silent=True) or {}
    name = (data.get('name') or '').strip()
    description = (data.get('description') or '').strip()
    icon = data.get('icon', 'school')
    tags = data.get('tags', [])

    if not name or len(name) < 2:
        return jsonify({'error': '子站名称至少2个字符'}), 400

    sid = create_station(name, description, icon, tags, g.current_user['id'],
                         category=(data.get('category') or '').strip())
    if not sid:
        return jsonify({'error': '创建失败'}), 500

    station = get_station_by_id(sid)
    station['tags'] = json.loads(station['tags']) if station['tags'] else []
    return jsonify({'message': '创建成功', 'station': station}), 201


@stations_bp.route('/<int:sid>/join', methods=['POST'])
@token_required
def join(sid):
    station = get_station_by_id(sid)
    if not station:
        return jsonify({'error': '子站不存在'}), 404
    if is_station_member(g.current_user['id'], sid):
        return jsonify({'message': '已经是成员了'})
    join_station(g.current_user['id'], sid)
    return jsonify({'message': '加入成功'})


@stations_bp.route('/<int:sid>/leave', methods=['POST'])
@token_required
def leave(sid):
    station = get_station_by_id(sid)
    if not station:
        return jsonify({'error': '子站不存在'}), 404
    leave_station(g.current_user['id'], sid)
    return jsonify({'message': '已退出'})


@stations_bp.route('/<int:sid>/posts', methods=['GET'])
@optional_auth
def station_posts(sid):
    station = get_station_by_id(sid)
    if not station:
        return jsonify({'error': '子站不存在'}), 404
    limit = request.args.get('limit', 20, type=int)
    offset = request.args.get('offset', 0, type=int)
    sort = request.args.get('sort', 'newest')
    posts = get_posts(station_id=sid, limit=limit, offset=offset, sort=sort)
    total = get_post_count(station_id=sid)
    # 与 /api/posts 对齐：登录态批量点赞 + 匿名脱敏 + vote/link shaping（防匿名帖身份泄露）
    liked = is_liked_batch(g.current_user['id'], 'post', [p['id'] for p in posts]) if (g.current_user and posts) else set()
    for p in posts:
        if g.current_user:
            p['is_liked'] = p['id'] in liked
        shape_post(p, g.current_user)
    return jsonify({'posts': posts, 'total': total})


@stations_bp.route('/by/<int:uid>', methods=['GET'])
def user_stations(uid):
    """GET /api/stations/by/<uid>：他人主页子站 Tab，仅公开子站关系、不含 role。"""
    stations = get_user_stations(uid)
    out = []
    for s in stations:
        if s.get('is_public') == 0:
            continue
        s['tags'] = json.loads(s['tags']) if s.get('tags') else []
        s.pop('role', None)
        out.append(s)
    return jsonify(out)


@stations_bp.route('/search', methods=['GET'])
def search():
    keyword = request.args.get('q', '').strip()
    if not keyword:
        return jsonify([])
    stations = search_stations(keyword)
    for s in stations:
        s['tags'] = json.loads(s['tags']) if s['tags'] else []
    return jsonify(stations)


@stations_bp.route('/mine', methods=['GET'])
@token_required
def my_stations():
    stations = get_user_stations(g.current_user['id'])
    for s in stations:
        s['tags'] = json.loads(s['tags']) if s['tags'] else []
    return jsonify(stations)


@stations_bp.route('/upload-cover', methods=['POST'])
@token_required
def upload_cover():
    if 'file' not in request.files:
        return jsonify({'error': '请选择文件'}), 400
    file = request.files['file']
    if not file.filename:
        return jsonify({'error': '请选择文件'}), 400

    ext = file.filename.rsplit('.', 1)[-1].lower() if '.' in file.filename else ''
    if ext not in ('png', 'jpg', 'jpeg', 'gif', 'webp'):
        return jsonify({'error': '不支持的格式'}), 400

    filename = f"cover_{g.current_user['id']}_{uuid.uuid4().hex[:8]}.{ext}"
    filepath = os.path.join(current_app.config['UPLOAD_FOLDER'], filename)
    file.save(filepath)
    return jsonify({'url': f'/static/uploads/{filename}'})


@stations_bp.route('/<int:sid>/stats', methods=['GET'])
@optional_auth
def station_stats(sid):
    if not get_station_by_id(sid):
        return jsonify({'error': '子站不存在'}), 404
    days = request.args.get('days', 7, type=int)
    return jsonify(get_station_stats(sid, days))


@stations_bp.route('/<int:sid>/members', methods=['GET'])
@optional_auth
def members(sid):
    station = get_station_by_id(sid)
    if not station:
        return jsonify({'error': '子站不存在'}), 404
    if g.current_user:
        station['my_role'] = get_station_member_role(g.current_user['id'], sid)
    return jsonify({'members': get_station_members(sid), 'my_role': station.get('my_role')})


@stations_bp.route('/<int:sid>', methods=['PUT'])
@token_required
def edit(sid):
    err, station = _require_owner(sid)
    if err:
        return err
    data = request.get_json(silent=True) or {}
    if 'tags' in data and isinstance(data['tags'], list):
        data['tags'] = json.dumps(data['tags'], ensure_ascii=False)
    if not data:
        return jsonify({'error': '没有可更新的字段'}), 400
    update_station(sid, **data)
    return jsonify({'message': '更新成功'})


@stations_bp.route('/<int:sid>', methods=['DELETE'])
@token_required
def delete(sid):
    err, station = _require_owner(sid)
    if err:
        return err
    delete_station(sid)
    return jsonify({'message': '已删除'})


@stations_bp.route('/<int:sid>/members/<int:uid>', methods=['DELETE'])
@token_required
def remove_member(sid, uid):
    err, station = _require_owner(sid)
    if err:
        return err
    if uid == g.current_user['id']:
        return jsonify({'error': '不能移除自己'}), 400
    if not remove_station_member(uid, sid):
        return jsonify({'error': '成员不存在或为子站长'}), 400
    return jsonify({'message': '已移除'})


@stations_bp.route('/<int:sid>/transfer', methods=['POST'])
@token_required
def transfer(sid):
    err, station = _require_owner(sid)
    if err:
        return err
    data = request.get_json(silent=True) or {}
    new_owner_id = data.get('new_owner_id')
    if not new_owner_id:
        return jsonify({'error': '缺少新子站长ID'}), 400
    if not is_station_member(int(new_owner_id), sid):
        return jsonify({'error': '新子站长必须是子站成员'}), 400
    transfer_station_ownership(sid, int(new_owner_id))
    return jsonify({'message': '转让成功'})
