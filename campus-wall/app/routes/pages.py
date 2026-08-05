from flask import Blueprint, render_template

pages_bp = Blueprint('pages', __name__)

@pages_bp.route('/')
def index():
    return render_template('index.html')

@pages_bp.route('/station/<int:sid>')
def station_detail(sid):
    return render_template('station.html', station_id=sid)

@pages_bp.route('/post/<int:pid>')
def post_detail(pid):
    return render_template('post.html', post_id=pid)

@pages_bp.route('/profile/<int:uid>')
def profile(uid):
    return render_template('profile.html', user_id=uid)

@pages_bp.route('/create')
def create_page():
    return render_template('create.html')

@pages_bp.route('/create-station')
def create_station_page():
    return render_template('create_station.html')

@pages_bp.route('/notifications')
def notifications_page():
    return render_template('notifications.html')

@pages_bp.route('/search')
def search_page():
    return render_template('search.html')

# ── 新增页面 ──

@pages_bp.route('/checkin')
def checkin_page():
    return render_template('checkin.html')

@pages_bp.route('/shop')
def shop_page():
    return render_template('shop.html')

@pages_bp.route('/romance')
def romance_page():
    return render_template('romance.html')

@pages_bp.route('/gossip')
def gossip_page():
    return render_template('gossip.html')

@pages_bp.route('/trade')
def trade_page():
    return render_template('trade.html')

@pages_bp.route('/admin')
def admin_page():
    return render_template('admin.html')

@pages_bp.route('/waterfall')
def waterfall_page():
    return render_template('waterfall.html')
