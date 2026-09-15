"""上传图片后处理：内容校验（防伪造扩展名）+ EXIF 剥离 + 长边压缩到 max_side。

设计：校验失败必须删文件并让调用方返回 400；压缩失败静默保留原图（可用性优先）。
"""
import os

_MAX_SIDE = 1600
_EXTS = {'.png', '.jpg', '.jpeg', '.gif', '.webp', '.bmp'}


def compress_image(path, max_side=_MAX_SIDE):
    """就地压缩图片。返回 True=已处理或已优化，False=跳过/失败保留原图。"""
    ext = os.path.splitext(path)[1].lower()
    if ext not in _EXTS:
        return False
    try:
        from PIL import Image, ImageOps
        with Image.open(path) as im:
            im = ImageOps.exif_transpose(im)  # 按 EXIF 方向摆正，随后不再写 EXIF
            w, h = im.size
            if max(w, h) <= max_side and ext not in ('.bmp',):
                # 尺寸达标：仍重存一次以剥离元数据（GIF 保留动图，跳过）
                if im.format == 'GIF' or im.n_frames > 1:
                    return False
                im.save(path, optimize=True)
                return True
            im.thumbnail((max_side, max_side), Image.LANCZOS)
            save_kw = {'optimize': True}
            if ext in ('.jpg', '.jpeg'):
                save_kw['quality'] = 82
            elif ext == '.png':
                save_kw['optimize'] = True
            im.save(path, **save_kw)
            return True
    except Exception:
        return False


def finalize_upload(filepath):
    """校验文件内容确为图片并就地优化。失败时删除文件、返回 False。"""
    try:
        from PIL import Image
        with Image.open(filepath) as im:
            im.verify()
    except Exception:
        try:
            os.remove(filepath)
        except OSError:
            pass
        return False
    compress_image(filepath)
    return True
