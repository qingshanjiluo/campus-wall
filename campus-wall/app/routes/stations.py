import json
from flask import Blueprint, request, jsonify, g
from app.models import (
    create_station, get_stations, get_station_by_id, search_stations,
    join_station, leave_station, is_station_member, get_posts, get_post_count
)
from app.utils.auth import token_required, optional_auth

stations_bp = Blueprint('stations', __name__)


@stations_bp.route('', methods=['GET'])
@optional_auth
def list_stations():
    limit = request.args.get('limit', 20, type=int)
    offset = request.args.get('offset', 0, type=int)
    sort = request.args.get('sort', 'newest')
    tag = request.args.get('tag', '').strip() or None

    stations = get_stations(limit=limit, offset=offset, sort=sort, tag=tag)
    for s in stations:
        s['tags'] = json.loads(s['tags']) if s['tags'] else []
        if g.current_user:
            s['is_member'] = is_station_member(g.current_user['id'], s['id'])
        else:
            s['is_member'] = False
    return jsonify(stations)


@stations_bp.route('/<int:sid>', methods=['GET'])
@optional_auth
def get_station(sid):
    station = get_station_by_id(sid)
    if not station:
        return jsonify({'error': '子站不存在'}), 404
    station['tags'] = json.loads(station['tags']) if station['tags'] else []
    if g.current_user:
        station['is_member'] = is_station_member(g.current_user['id'], sid)
    else:
        station['is_member'] = False
    return jsonify(station)


@stations_bp.route('', methods=['POST'])
@token_required
def create():
    data = request.get_json(silent=True) or {}
    name = (data.get('name') or '').strip()
    description = (data.get('description') or '').strip()
    icon = data.get('icon', '🏫')
    tags = data.get('tags', [])

    if not name or len(name) < 2:
        return jsonify({'error': '子站名称至少2个字符'}), 400

    sid = create_station(name, description, icon, tags, g.current_user['id'])
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
    return jsonify({'posts': posts, 'total': total})


@stations_bp.route('/search', methods=['GET'])
def search():
    keyword = request.args.get('q', '').strip()
    if not keyword:
        return jsonify([])
    stations = search_stations(keyword)
    for s in stations:
        s['tags'] = json.loads(s['tags']) if s['tags'] else []
    return jsonify(stations)
