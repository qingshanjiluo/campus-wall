"""本地联调服务器：让 Flask 后端从 frontend/ 读取静态页面 + 静态资源
用于验证静态前端与后端 API 协同（无需部署到 Cloudflare）。

用法：
    python dev-server.py
    浏览器访问 http://127.0.0.1:5001
"""
import os
import sys

# 确保能 import 后端
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from app import create_app

FRONTEND = os.path.join(os.path.dirname(os.path.abspath(__file__)))

app = create_app()
app.template_folder = os.path.join(FRONTEND, 'pages')
app.static_folder = os.path.join(FRONTEND, 'static')

if __name__ == '__main__':
    app.run(host='127.0.0.1', port=5001, debug=False)
