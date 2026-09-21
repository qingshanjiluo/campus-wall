import os
from flask import Flask, request, render_template, jsonify, send_from_directory
from flask_cors import CORS


def create_app():
    _base = os.path.abspath(os.path.dirname(__file__))          # campus-wall/app
    _root = os.path.dirname(_base)                              # campus-wall
    frontend = os.environ.get('FRONTEND_DIR') or os.path.join(_root, 'frontend')

    app = Flask(__name__,
                template_folder='templates',
                static_folder=os.path.join(frontend, 'static'),
                static_url_path='/static')

    # 线上前端 = campus-wall/frontend 静态 21 页（templates 旧版已冻结）
    app.config['FRONTEND_PAGES'] = os.path.join(frontend, 'pages')

    app.config['SECRET_KEY'] = os.environ.get('SECRET_KEY', 'dev-secret-change-in-production')
    app.config['JWT_EXPIRATION_DAYS'] = 7
    app.config['UPLOAD_FOLDER'] = os.path.join(app.static_folder, 'uploads')
    app.config['MAX_CONTENT_LENGTH'] = 16 * 1024 * 1024  # 16MB

    # JWT Secret: 生产环境必须设置
    jwt_secret = os.environ.get('JWT_SECRET', '')
    if not jwt_secret or jwt_secret == 'jwt-dev-secret-change-in-production':
        if os.environ.get('FLASK_ENV') == 'production':
            raise RuntimeError('生产环境必须设置 JWT_SECRET 环境变量！')
        jwt_secret = 'dev-only-jwt-secret-please-change'
    app.config['JWT_SECRET'] = jwt_secret

    allowed_origins = os.environ.get('CORS_ORIGINS', '*').split(',')
    CORS(app, supports_credentials=True, origins=allowed_origins)

    os.makedirs(app.config['UPLOAD_FOLDER'], exist_ok=True)
    os.makedirs(os.path.join(app.static_folder, 'images'), exist_ok=True)

    # 限流（RATELIMIT_ENABLED=1 时生效；生产 compose 开启，开发/E2E 默认关）
    from app.utils.limiter import limiter, init_app as init_limiter
    init_limiter(app)

    @app.errorhandler(429)
    def _rate_limited(e):
        if request.path.startswith('/api/'):
            return jsonify({'error': '操作过于频繁，请稍后再试'}), 429
        return render_template('404.html'), 429

    from app.routes.auth import auth_bp
    from app.routes.stations import stations_bp
    from app.routes.posts import posts_bp
    from app.routes.social import social_bp
    from app.routes.dm import dm_bp
    from app.routes.pages import pages_bp
    from app.routes.extended import (
        identity_bp, checkin_bp, shop_bp, romance_bp, gossip_bp,
        trade_bp, kanban_bp, recommend_bp, admin_bp,
        favorites_bp, reports_bp, announcements_bp, world_bp, site_bp, topics_bp, tasks_bp, user_bp, chat_bp, events_bp, chat_bp
    )

    app.register_blueprint(auth_bp, url_prefix='/api/auth')
    app.register_blueprint(stations_bp, url_prefix='/api/stations')
    app.register_blueprint(posts_bp, url_prefix='/api/posts')
    app.register_blueprint(social_bp, url_prefix='/api/social')
    app.register_blueprint(dm_bp, url_prefix='/api/dm')
    app.register_blueprint(identity_bp, url_prefix='/api/identity')
    app.register_blueprint(checkin_bp, url_prefix='/api/checkin')
    app.register_blueprint(shop_bp, url_prefix='/api/shop')
    app.register_blueprint(romance_bp, url_prefix='/api/romance')
    app.register_blueprint(gossip_bp, url_prefix='/api/gossip')
    app.register_blueprint(trade_bp, url_prefix='/api/trade')
    app.register_blueprint(kanban_bp, url_prefix='/api/kanban')
    app.register_blueprint(recommend_bp, url_prefix='/api/recommend')
    app.register_blueprint(admin_bp, url_prefix='/api/admin')
    app.register_blueprint(favorites_bp, url_prefix='/api/favorites')
    app.register_blueprint(reports_bp, url_prefix='/api/reports')
    app.register_blueprint(announcements_bp, url_prefix='/api/announcements')
    app.register_blueprint(world_bp, url_prefix='/api/world')
    app.register_blueprint(site_bp, url_prefix='/api/site')
    app.register_blueprint(topics_bp)
    app.register_blueprint(tasks_bp)
    app.register_blueprint(user_bp, url_prefix='/api/user')
    app.register_blueprint(chat_bp, url_prefix='/api/chat')
    app.register_blueprint(events_bp, url_prefix='/api/events')
    app.register_blueprint(pages_bp)

    # 插件钩子（R13）：注册内置插件
    from app.utils.plugins import register_builtin_plugins
    register_builtin_plugins()

    @app.errorhandler(404)
    def not_found(e):
        if request.path.startswith('/api/'):
            return jsonify({'error': '接口不存在'}), 404
        return send_from_directory(app.config['FRONTEND_PAGES'], '404.html'), 404

    @app.errorhandler(500)
    def server_error(e):
        if request.path.startswith('/api/'):
            return jsonify({'error': '服务器内部错误'}), 500
        return send_from_directory(app.config['FRONTEND_PAGES'], '404.html'), 500

    return app
