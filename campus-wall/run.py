import os
import sys
sys.path.insert(0, os.path.dirname(__file__))

from app import create_app
from app.models import init_db
from app.models_ext import init_extended_db
from app.seed import seed

app = create_app()

with app.app_context():
    init_db()
    init_extended_db()
    seed()

if __name__ == '__main__':
    port = int(os.environ.get('PORT', 5000))
    debug = os.environ.get('FLASK_ENV') != 'production'
    app.run(host='0.0.0.0', port=port, debug=debug)
