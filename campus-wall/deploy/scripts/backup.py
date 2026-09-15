#!/usr/bin/env python3
"""CampusWall 备份：SQLite 在线快照（sqlite3 backup API，WAL 下安全）+ uploads 打包，滚动保留 N 份。

运行于 backup sidecar（cron 每日调用）：
  /usr/local/bin/python /scripts/backup.py
环境变量：DB_PATH(默认 /data/campushub.db) UPLOADS_DIR(默认 /uploads) BACKUP_DIR(默认 /backups) KEEP(默认 7)
"""
import os
import sqlite3
import tarfile
import tempfile
from datetime import datetime

DB_PATH = os.environ.get('DB_PATH', '/data/campushub.db')
UPLOADS_DIR = os.environ.get('UPLOADS_DIR', '/uploads')
BACKUP_DIR = os.environ.get('BACKUP_DIR', '/backups')
KEEP = int(os.environ.get('KEEP', '7'))


def main():
    os.makedirs(BACKUP_DIR, exist_ok=True)
    stamp = datetime.now().strftime('%Y%m%d_%H%M%S')
    out = os.path.join(BACKUP_DIR, f'campuswall_backup_{stamp}.tar.gz')

    with tempfile.TemporaryDirectory() as tmp:
        stage = os.path.join(tmp, f'backup_{stamp}')
        os.makedirs(stage)
        # 在线一致快照（而非 cp 热文件——WAL 模式下 cp 可能拿到撕裂数据）
        if os.path.exists(DB_PATH):
            src = sqlite3.connect(DB_PATH)
            dst = sqlite3.connect(os.path.join(stage, 'campushub.db'))
            with dst:
                src.backup(dst)
            src.close(); dst.close()
        if os.path.isdir(UPLOADS_DIR):
            up_stage = os.path.join(stage, 'uploads')
            os.makedirs(up_stage)
            import shutil
            for fn in os.listdir(UPLOADS_DIR):
                p = os.path.join(UPLOADS_DIR, fn)
                if os.path.isfile(p):
                    shutil.copy2(p, os.path.join(up_stage, fn))
        with tarfile.open(out, 'w:gz') as tar:
            tar.add(stage, arcname=f'backup_{stamp}')

    # 滚动清理
    backups = sorted(
        (os.path.join(BACKUP_DIR, f) for f in os.listdir(BACKUP_DIR)
         if f.startswith('campuswall_backup_') and f.endswith('.tar.gz')),
        key=os.path.getmtime)
    for old in backups[:-KEEP] if KEEP > 0 else []:
        os.remove(old)
    print(f'backup ok: {out} ({os.path.getsize(out)} bytes, keep={KEEP})')


if __name__ == '__main__':
    main()
