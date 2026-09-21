"""服务器版页面路由：托管 campus-wall/frontend/pages 静态 21 页。

与已线上验证的 frontend/_worker.js 语义同构：干净路由 → 具体 html 文件；
/post|/station|/profile/<id> 前缀 → 对应页；未知路径 → 404.html(404)。
（templates/ 服务端渲染旧版已冻结，仅存档。）
"""
import os

from flask import Blueprint, current_app, send_from_directory

pages_bp = Blueprint('pages', __name__)

# 干净路由 → pages 目录文件名（与 _worker.js ROUTES 一致）
ROUTES = {
    '/': 'index.html',
    '/waterfall': 'waterfall.html',
    '/world': 'world.html',
    '/expose': 'expose.html',
    '/forum': 'forum.html',
    '/tasks': 'tasks.html',
    '/chat': 'chat.html',
    '/events': 'events.html',
    '/qna': 'qna.html',
    '/trade': 'trade.html',
    '/romance': 'romance.html',
    '/gossip': 'gossip.html',
    '/shop': 'shop.html',
    '/search': 'search.html',
    '/checkin': 'checkin.html',
    '/favorites': 'favorites.html',
    '/notifications': 'notifications.html',
    '/messages': 'messages.html',
    '/create': 'create.html',
    '/create-station': 'create_station.html',
    '/admin': 'admin.html',
    '/reset-password': 'reset_password.html',
    '/about': 'about.html',
    '/terms': 'terms.html',
    '/privacy': 'privacy.html',
}

# 前缀路由 → 详情页
PREFIXES = {
    '/post/': 'post.html',
    '/station/': 'station.html',
    '/profile/': 'profile.html',
}


def _pages_dir():
    return current_app.config['FRONTEND_PAGES']


def _serve(fname):
    resp = send_from_directory(_pages_dir(), fname)
    resp.headers['Cache-Control'] = 'no-cache'
    return resp


def _make_view(fname):
    def view(**_kwargs):
        return _serve(fname)
    view.__name__ = 'page_' + fname.replace('.html', '').replace('/', '_')
    return view


@pages_bp.route('/')
def index():
    return _serve(ROUTES['/'])


@pages_bp.route('/post/<path:ident>')
def post_page(ident):
    return _serve('post.html')


@pages_bp.route('/station/<path:ident>')
def station_page(ident):
    return _serve('station.html')


@pages_bp.route('/profile/<path:ident>')
def profile_page(ident):
    return _serve('profile.html')


def init_routes():
    """把 ROUTES 注册到 blueprint（排除 /，其有专用 view）。"""
    for path, fname in ROUTES.items():
        if path == '/':
            continue
        pages_bp.add_url_rule(path, view_func=_make_view(fname))


init_routes()
