# CampusWall Flask 应用镜像
FROM python:3.12-slim

ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    TZ=Asia/Shanghai

WORKDIR /srv/campuswall

# 依赖层先缓存
COPY campus-wall/requirements.txt /srv/campuswall/requirements.txt
RUN pip install --no-cache-dir -r requirements.txt

# 应用代码 + 前端静态
COPY campus-wall/app        /srv/campuswall/app
COPY campus-wall/frontend   /srv/campuswall/frontend
COPY campus-wall/run.py     /srv/campuswall/run.py

# 数据目录（SQLite + 上传件）挂 volume
RUN mkdir -p /data && chown -R www-data:www-data /data /srv/campuswall/frontend/static/uploads 2>/dev/null || true
ENV DATABASE_PATH=/data/campus.db \
    FRONTEND_DIR=/srv/campuswall/frontend

EXPOSE 8000

# 健康检查：API 根探活
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
  CMD python -c "import urllib.request,sys; sys.exit(0 if urllib.request.urlopen('http://127.0.0.1:8000/api/stations',timeout=4).status==200 else 1)"

# --preload: 多 worker 共享 init_db/seed；threads=2 兜底慢请求
CMD ["gunicorn", "-w", "4", "--threads", "2", "--preload", "-b", "0.0.0.0:8000", "--timeout", "60", "run:app"]
