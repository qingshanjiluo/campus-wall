import os
from flask import Flask, request, render_template, jsonify
from flask_cors import CORS


def create_app():
    app = Flask(__name__,
                template_folder='templates',
                static_folder='static')

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
    from app.routes.pages import pages_bp
    from app.routes.extended import (
        identity_bp, checkin_bp, shop_bp, romance_bp, gossip_bp,
        trade_bp, kanban_bp, recommend_bp, admin_bp,
        favorites_bp, reports_bp, announcements_bp
    )

    app.register_blueprint(auth_bp, url_prefix='/api/auth')
    app.register_blueprint(stations_bp, url_prefix='/api/stations')
    app.register_blueprint(posts_bp, url_prefix='/api/posts')
    app.register_blueprint(social_bp, url_prefix='/api/social')
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
    app.register_blueprint(pages_bp)

    @app.errorhandler(404)
    def not_found(e):
        if request.path.startswith('/api/'):
            return jsonify({'error': '接口不存在'}), 404
        return render_template('404.html'), 404

    @app.errorhandler(500)
    def server_error(e):
        if request.path.startswith('/api/'):
            return jsonify({'error': '服务器内部错误'}), 500
        return render_template('base.html', error_code=500), 500

    return app
