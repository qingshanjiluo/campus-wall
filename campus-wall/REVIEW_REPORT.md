# CampusWall 全面审查报告 — 错误修复 · 功能完善 · UI 加强

> 审查日期：2026-07-18 | 基于完整代码逐行审查

---

## 一、错误修复（Bugs）

### BUG-01：`toggleLike` 函数先操作 DOM 再覆写 `innerHTML`，前序操作无效

**文件**：`app/static/js/app.js` → `toggleLike()`

**问题**：函数先设置 `btn.classList` 和 `icon.textContent`，然后用 `btn.innerHTML` 整体覆写，导致前两行操作白做。且 `textContent` 解析数字不可靠（可能拿到 emoji 数字混合）。

```javascript
// 当前代码（有 bug）
function toggleLike(postId, btn) {
    api.post('/api/posts/' + postId + '/like')
        .then(data => {
            const icon = btn.querySelector('.icon');
            if (data.liked) { btn.classList.add('liked'); if (icon) icon.textContent = '❤️'; }
            else { btn.classList.remove('liked'); if (icon) icon.textContent = '🤍'; }
            const text = btn.textContent.trim();          // ← 可能拿到 "❤️ 128"
            const num = parseInt(text.replace(/[^\d]/g, '')) || 0;  // ← 解析可能出错
            btn.innerHTML = `...${newNum}`;               // ← 覆写了上面的 classList 操作
        })
}
```

**修复**：直接用 `innerHTML` 一次性完成，不进行中间 DOM 操作。同时用 `data-likes` 属性存储精确数值。

```javascript
function toggleLike(postId, btn) {
    if (!requireAuth()) return;
    const currentLikes = parseInt(btn.dataset.likes || btn.textContent.replace(/[^\d]/g, '')) || 0;
    api.post('/api/posts/' + postId + '/like')
        .then(data => {
            const newLikes = data.liked ? currentLikes + 1 : Math.max(0, currentLikes - 1);
            btn.dataset.likes = newLikes;
            btn.className = 'post-action' + (data.liked ? ' liked' : '');
            btn.innerHTML = `<span class="icon">${data.liked ? '❤️' : '🤍'}</span> ${newLikes}`;
        })
        .catch(e => showToast(e.message || '操作失败'));
}
```

同样修复 `toggleCommentLike`。

---

### BUG-02：发帖成功后 `loadPosts()` 调用无效

**文件**：`app/static/js/app.js` → `handleCreatePost()`

**问题**：`handleCreatePost` 中 `if (typeof loadPosts === 'function') loadPosts()` — 但 `loadPosts` 定义在 `index.html` 的 `<script>` 块中，非全局作用域（在 `DOMContentLoaded` 回调内定义的函数不可全局访问）。从弹窗发帖成功后帖子列表不会刷新。

**修复**：将 `loadPosts` 等函数挂到 `window` 上，或在 app.js 中用自定义事件通知。

```javascript
// index.html 中：
window.loadPosts = loadPosts;  // 暴露到全局

// 或 app.js 中使用事件：
function handleCreatePost(e) {
    // ...
    .then(res => {
        closeModal('createPostModal');
        showToast('发帖成功！');
        form.reset();
        document.dispatchEvent(new CustomEvent('postCreated'));  // ← 通知
    })
}

// index.html 中监听：
document.addEventListener('postCreated', () => loadPosts());
```

---

### BUG-03：`leave_station` 不检查是否为成员就减少计数

**文件**：`app/models.py` → `leave_station()`

**问题**：直接执行 DELETE + `user_count - 1`，如果用户不是成员，DELETE 影响 0 行但计数仍然 -1。

```python
def leave_station(user_id, station_id):
    execute_db('DELETE FROM station_members WHERE user_id = ? AND station_id = ?', ...)
    execute_db('UPDATE stations SET user_count = MAX(0, user_count - 1) WHERE id = ?', ...)
```

**修复**：检查 DELETE 是否实际删除了行。

```python
def leave_station(user_id, station_id):
    conn = get_db()
    cur = conn.execute('DELETE FROM station_members WHERE user_id = ? AND station_id = ?',
                       (user_id, station_id))
    deleted = cur.rowcount
    conn.commit()
    if deleted > 0:
        conn.execute('UPDATE stations SET user_count = MAX(0, user_count - 1) WHERE id = ?',
                     (station_id,))
        conn.commit()
    conn.close()
```

---

### BUG-04：`delete_post` 不减少 `station.post_count`

**文件**：`app/models.py` → `delete_post()`

**问题**：软删除帖子后，子站的 `post_count` 不减少，导致计数虚高。

**修复**：

```python
def delete_post(pid):
    post = query_db('SELECT station_id FROM posts WHERE id = ?', (pid,), one=True)
    execute_db('UPDATE posts SET is_deleted = 1 WHERE id = ?', (pid,))
    if post:
        execute_db('UPDATE stations SET post_count = MAX(0, post_count - 1) WHERE id = ?',
                   (post['station_id'],))
```

---

### BUG-05：评论删除不检查权限

**文件**：`app/routes/posts.py` → `del_comment()`

**问题**：任何已登录用户都能删除任何人的评论，没有权限检查。

```python
@posts_bp.route('/comments/<int:cid>', methods=['DELETE'])
@token_required
def del_comment(cid):
    delete_comment(cid)  # ← 没检查是不是评论作者或管理员
    return jsonify({'message': '已删除'})
```

**修复**：

```python
@posts_bp.route('/comments/<int:cid>', methods=['DELETE'])
@token_required
def del_comment(cid):
    from app.models import query_db
    comment = query_db('SELECT author_id FROM comments WHERE id = ?', (cid,), one=True)
    if not comment:
        return jsonify({'error': '评论不存在'}), 404
    if comment['author_id'] != g.current_user['id'] and g.current_user['role'] != 'admin':
        return jsonify({'error': '无权删除'}), 403
    delete_comment(cid)
    return jsonify({'message': '已删除'})
```

---

### BUG-06：`request.get_json()` 不处理 `None` 返回值

**文件**：`app/routes/auth.py`, `stations.py`, `posts.py`

**问题**：如果请求体不是合法 JSON，`get_json()` 返回 `None`，后续 `.get()` 会抛 `AttributeError`。

**修复**（所有 POST 路由统一处理）：

```python
@auth_bp.route('/login', methods=['POST'])
def login():
    data = request.get_json(silent=True) or {}  # ← 加 silent=True 和 or {}
    # ...
```

---

### BUG-07：`formatTime` 使用客户端本地时间与 SQLite `CURRENT_TIMESTAMP`（UTC）比较

**文件**：`app/static/js/app.js` → `formatTime()`

**问题**：SQLite 的 `CURRENT_TIMESTAMP` 存储 UTC 时间，但 `new Date()` 和 `new Date(timestamp)` 在没有时区信息时会按本地时间解析，导致时间差计算错误（东八区会差 8 小时）。

**修复**：在 Python 端返回 ISO 格式带时区，或 JS 端统一处理。

```python
# models.py — init_db 中所有 TIMESTAMP 字段加默认值时用 ISO 格式
# 或在路由中格式化时间
from datetime import datetime, timezone

def format_timestamp(ts_str):
    if not ts_str:
        return ''
    try:
        dt = datetime.fromisoformat(str(ts_str).replace('Z', '+00:00'))
        if dt.tzinfo is None:
            dt = dt.replace(tzinfo=timezone.utc)
        return dt.isoformat()
    except:
        return str(ts_str)
```

```javascript
// app.js — formatTime 修正
function formatTime(timestamp) {
    if (!timestamp) return '刚刚';
    try {
        let date;
        if (typeof timestamp === 'string' && !timestamp.includes('T')) {
            date = new Date(timestamp + 'Z');  // SQLite 无时区，加 Z 当 UTC
        } else {
            date = new Date(timestamp);
        }
        if (isNaN(date.getTime())) return '刚刚';
        const now = new Date();
        const diff = Math.floor((now - date) / 1000);
        // ...后续逻辑不变
    } catch { return '刚刚'; }
}
```

---

### BUG-08：404 错误处理器对 HTML 页面返回 JSON

**文件**：`app/__init__.py`

**问题**：访问不存在的页面（如 `/nonexistent`）返回 JSON `{"error": "页面未找到"}`，但浏览器期望 HTML。

**修复**：

```python
@app.errorhandler(404)
def not_found(e):
    # 如果是 API 请求返回 JSON，否则返回 HTML
    if request.path.startswith('/api/'):
        return jsonify({'error': '接口不存在'}), 404
    return render_template('base.html', error_code=404), 404
```

---

### BUG-09：`loadStationOptions` 不在打开模态框时自动调用

**文件**：`app/static/js/app.js`

**问题**：发帖模态框 (`createPostModal`) 中的子站选择 `<select>` 只在 `create.html` 页面的 `DOMContentLoaded` 中加载。从首页通过弹窗发帖时，下拉列表是空的（只有"请选择子站..."）。

**修复**：在 `openModal` 时自动加载。

```javascript
function openModal(id) {
    const modal = document.getElementById(id);
    if (modal) {
        modal.classList.add('show');
        document.body.style.overflow = 'hidden';
        // 如果是发帖弹窗，加载子站列表
        if (id === 'createPostModal') {
            loadStationOptions('postStationSelect');
        }
    }
}
```

---

### BUG-10：首页 `initReveal` 时机问题 — 动态内容淡入失效

**文件**：`app/templates/index.html`

**问题**：`initAnimations()` 在 `app.js` 的 `DOMContentLoaded` 中调用，此时帖子/子站数据还没加载完成。加载完成后 `renderStations` / `renderPosts` 调用了 `initReveal()`，但如果 `IntersectionObserver` 已经初始化过但没覆盖新元素，可能会有遗漏。

**确认**：当前实现每次 `initReveal()` 都重新查询 `.reveal:not(.visible)` 并创建新 observer，这是正确的。但旧 observer 不会被 GC 回收（因为回调闭包引用），可能导致内存泄漏。

**修复**：复用同一个 observer。

```javascript
let revealObserver = null;

function initReveal() {
    if (!revealObserver) {
        revealObserver = new IntersectionObserver((entries) => {
            entries.forEach(entry => {
                if (entry.isIntersecting) {
                    entry.target.classList.add('visible');
                    revealObserver.unobserve(entry.target);
                }
            });
        }, { threshold: 0.1, rootMargin: '0px 0px -30px 0px' });
    }
    document.querySelectorAll('.reveal:not(.visible)').forEach(el => {
        revealObserver.observe(el);
    });
}
```

---

## 二、功能完善（Feature Completion）

### FEAT-01：图片上传功能

**现状**：帖子和用户头像的 `image`/`avatar` 字段只支持 URL 字符串，无实际上传能力。

**实现方案**：

```python
# routes/auth.py — 头像上传
@auth_bp.route('/avatar', methods=['POST'])
@token_required
def upload_avatar():
    if 'file' not in request.files:
        return jsonify({'error': '请选择文件'}), 400
    file = request.files['file']
    if not file.filename or not allowed_file(file.filename):
        return jsonify({'error': '不支持的文件格式'}), 400

    import uuid
    ext = file.filename.rsplit('.', 1)[-1].lower()
    filename = f"avatar_{g.current_user['id']}_{uuid.uuid4().hex[:8]}.{ext}"
    filepath = os.path.join(current_app.config['UPLOAD_FOLDER'], filename)
    file.save(filepath)

    avatar_url = f'/static/uploads/{filename}'
    update_user(g.current_user['id'], avatar=avatar_url)
    return jsonify({'url': avatar_url})

def allowed_file(filename):
    return '.' in filename and filename.rsplit('.', 1)[1].lower() in {'png', 'jpg', 'jpeg', 'gif', 'webp'}
```

**前端**：在个人中心和发帖表单中添加文件选择器。

---

### FEAT-02：个人资料编辑功能

**现状**：`profile.html` 中编辑按钮显示"编辑资料功能开发中"。

**实现**：在 `app.js` 中添加编辑模态框逻辑。

```javascript
function openEditProfile() {
    if (!currentUser) return;
    // 填充现有数据
    const modal = document.getElementById('editProfileModal');
    if (modal) {
        modal.querySelector('[name="username"]').value = currentUser.username;
        modal.querySelector('[name="bio"]').value = currentUser.bio || '';
        openModal('editProfileModal');
    }
}

function handleEditProfile(e) {
    e.preventDefault();
    const form = e.target;
    const data = {
        username: form.username.value.trim(),
        bio: form.bio.value.trim()
    };
    api.put('/api/auth/me', data)
        .then(res => {
            currentUser = res.user;
            closeModal('editProfileModal');
            updateNavRight();
            showToast('资料更新成功');
            loadProfile();  // 刷新页面
        })
        .catch(err => showToast(err.message || '更新失败'));
    return false;
}
```

**新增模态框**（在 `modals.html` 中）：

```html
<div class="modal-overlay" id="editProfileModal">
    <div class="modal" style="position:relative;">
        <button class="modal-close" onclick="closeModal('editProfileModal')">&times;</button>
        <div class="modal-title">✏️ 编辑资料</div>
        <form onsubmit="return handleEditProfile(event)">
            <div class="form-group">
                <label class="form-label">用户名</label>
                <input class="input" type="text" name="username" required minlength="2">
            </div>
            <div class="form-group">
                <label class="form-label">个人简介</label>
                <textarea class="input" name="bio" rows="3" placeholder="介绍一下自己..."></textarea>
            </div>
            <div class="form-group">
                <label class="form-label">头像</label>
                <input class="input" type="file" name="avatar" accept="image/*">
            </div>
            <button type="submit" class="btn btn-primary btn-lg" style="width:100%;">保存</button>
        </form>
    </div>
</div>
```

---

### FEAT-03：帖子编辑功能

**现状**：后端有 `PUT /api/posts/<pid>` 但前端无入口。

**实现**：在帖子详情页添加"编辑"按钮，弹出编辑模态框。

---

### FEAT-04：加载动画仅首次显示

**现状**：每次页面刷新都播放 3 秒打字机动画，严重影响体验。

**修复**：用 `sessionStorage` 记录，非首次访问跳过。

```javascript
async function initAnimations() {
    generateDoodles();
    if (sessionStorage.getItem('loadingPlayed')) {
        // 非首次：直接隐藏
        const screen = document.getElementById('loadingScreen');
        if (screen) screen.classList.add('hidden');
    } else {
        await initLoadingAnimation();
        sessionStorage.setItem('loadingPlayed', '1');
    }
    showNavbar();
    initReveal();
}
```

---

### FEAT-05：导航栏滚动变色

**现状**：导航栏始终半透明，滚动到内容区后文字可能与背景混淆。

**修复**：监听滚动事件，添加 `scrolled` 类。

```javascript
// app.js
window.addEventListener('scroll', () => {
    const nav = document.getElementById('navbar');
    if (nav) {
        nav.classList.toggle('scrolled', window.scrollY > 60);
    }
}, { passive: true });
```

```css
/* style.css */
.navbar.scrolled {
    background: rgba(255, 248, 240, 0.92);
    box-shadow: 0 4px 20px rgba(139, 125, 107, 0.15);
}
```

---

### FEAT-06：子站详情页 "加入/退出" 按钮需要登录态刷新

**现状**：`is_member` 状态在页面加载时确定，但如果用户刚登录，页面不刷新就看不到正确的按钮状态。

**修复**：在 `loadStation` 时检查当前用户状态。

---

### FEAT-07：搜索防抖

**现状**：搜索只在按 Enter 时触发。应添加输入防抖实时搜索。

```javascript
let searchTimer = null;
function handleStationSearch(e) {
    clearTimeout(searchTimer);
    if (e.key === 'Enter') { doSearch(); return; }
    searchTimer = setTimeout(doSearch, 400);
}
function doSearch() {
    const q = document.getElementById('searchInput').value.trim();
    if (!q) { loadStations(); return; }
    api.get('/api/stations/search?q=' + encodeURIComponent(q))
        .then(data => renderStations(data));
}
```

---

### FEAT-08：子站成员管理（站长踢人/转让）

**现状**：`station_members` 表有 `role` 字段但无任何管理接口。

**建议添加**：
- `GET /api/stations/<id>/members` — 成员列表
- `DELETE /api/stations/<id>/members/<uid>` — 踢人（仅站长）
- `POST /api/stations/<id>/transfer` — 转让站长

---

### FEAT-09：帖子置顶功能

**现状**：数据模型有 `is_pinned` 字段，后端 `update_post` 支持，但前端无入口。

**实现**：子站详情页中，置顶帖子显示置顶标识并排在最前。

---

### FEAT-10：密码修改

**现状**：无修改密码功能。

**实现**：`POST /api/auth/change-password` + 个人中心入口。

---

## 三、UI 加强优化

### UI-01：加载动画优化

**问题**：
1. 每次刷新都播放（已在 FEAT-04 修复）
2. 安全超时 3.5 秒太长
3. 打字机效果在子页面显得多余

**优化**：
- 首次访问播放完整动画（~2.5 秒）
- 非首次直接跳过
- 安全超时缩短到 2 秒
- 子页面可考虑不显示加载动画（仅首页）

```javascript
async function initAnimations() {
    generateDoodles();
    const isHome = window.location.pathname === '/';
    const played = sessionStorage.getItem('loadingPlayed');
    const screen = document.getElementById('loadingScreen');

    if (!isHome || played) {
        if (screen) screen.classList.add('hidden');
    } else {
        await initLoadingAnimation();
        sessionStorage.setItem('loadingPlayed', '1');
    }
    showNavbar();
    initReveal();
}
```

---

### UI-02：子站卡片视觉多样性

**问题**：所有子站卡片样式完全相同，缺乏辨识度。

**优化**：根据子站 index 给不同卡片不同的渐变色和圆角变形。

```javascript
function renderStations(stations) {
    const gradients = [
        'linear-gradient(135deg, #FFB5BA, #FFD6A5)',
        'linear-gradient(135deg, #C9E4F5, #E2D5F5)',
        'linear-gradient(135deg, #B5EAD7, #C9E4F5)',
        'linear-gradient(135deg, #FDFFB6, #FFD6A5)',
        'linear-gradient(135deg, #E2D5F5, #FFB5BA)',
        'linear-gradient(135deg, #FFD6A5, #B5EAD7)',
    ];
    // ...在 renderStations 中：
    grid.innerHTML = stations.map((s, i) => `
        <div class="station-card reveal" onclick="..." style="--card-gradient:${gradients[i % gradients.length]}">
            <div class="station-icon" style="background:${gradients[i % gradients.length]}">
                ${s.icon || '🏫'}
            </div>
            ...
        </div>
    `).join('');
}
```

```css
.station-card:nth-child(6n+1) { border-radius: 24px 24px 24px 8px; }
.station-card:nth-child(6n+2) { border-radius: 24px 24px 8px 24px; }
.station-card:nth-child(6n+3) { border-radius: 8px 24px 24px 24px; }
.station-card:nth-child(6n+4) { border-radius: 24px 8px 24px 24px; }
.station-card:nth-child(6n+5) { border-radius: 24px 16px 24px 16px; }
.station-card:nth-child(6n+6) { border-radius: 16px 24px 16px 24px; }
```

---

### UI-03：帖子卡片对话气泡尾巴优化

**问题**：`::before` 伪元素用 CSS border 画三角形，在 `backdrop-filter` 下渲染异常（毛玻璃穿透）。

**优化**：改用 `clip-path` 或直接去掉气泡尾巴（在移动端已隐藏），改用左侧彩色竖线。

```css
.post-card::before {
    content: '';
    position: absolute; left: 0; top: 0;
    width: 4px; height: 100%;
    background: linear-gradient(to bottom, var(--pink-3), var(--orange));
    border-radius: 4px 0 0 4px;
    border: none;  /* 覆盖之前的三角形 */
}
```

---

### UI-04：Select 下拉框样式

**问题**：`<select class="input">` 继承了 input 的样式，但下拉箭头仍是浏览器默认，与整体风格不协调。

**优化**：

```css
select.input {
    appearance: none;
    -webkit-appearance: none;
    background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' viewBox='0 0 12 12'%3E%3Cpath d='M6 8L1 3h10z' fill='%238B7D6B'/%3E%3C/svg%3E");
    background-repeat: no-repeat;
    background-position: right 14px center;
    padding-right: 36px;
    cursor: pointer;
}
```

---

### UI-05：Toast 样式增强

**问题**：Toast 是纯黑底白字，与 Galgame 风格不协调。

**优化**：

```css
.toast {
    background: linear-gradient(135deg, var(--text-dark), #5a4a3a);
    border: 2px solid var(--pink-3);
    box-shadow: 0 6px 20px rgba(251, 111, 146, 0.25);
    font-family: 'ZCOOL KuaiLe', cursive;
    font-size: 0.95rem;
}
.toast.success { border-color: var(--mint); }
.toast.error { border-color: #ff6b6b; }
```

---

### UI-06：页面间导航体验

**问题**：点击子站/帖子链接是整页跳转，白屏闪烁。

**优化（短期）**：添加页面过渡动画。

```css
/* style.css */
main {
    animation: page-enter 0.4s ease;
}
@keyframes page-enter {
    from { opacity: 0; transform: translateY(12px); }
    to { opacity: 1; transform: translateY(0); }
}
```

**优化（长期）**：考虑 SPA 化（Turbo/HTMX/pjax）。

---

### UI-07：评论区交互增强

**问题**：评论区没有"回复"按钮，无法进行楼中楼对话。

**优化**：添加回复按钮 + 引用显示。

```javascript
function replyToComment(commentId, authorName) {
    const textarea = document.getElementById('commentContent');
    if (textarea) {
        textarea.value = `@${authorName} `;
        textarea.focus();
        textarea.dataset.parentId = commentId;
    }
}
```

---

### UI-08：个人中心页响应式

**问题**：`profile.html` 使用 `grid-template-columns: 280px 1fr`，在移动端没有 override（虽然有 media query 但 selector 不匹配实际结构）。

**修复**：

```html
<!-- profile.html 中添加 style -->
<style>
@media (max-width: 768px) {
    #profileLayout { grid-template-columns: 1fr !important; }
}
</style>
```

这个已存在但需要确认 `#profileLayout` ID 正确绑定。

---

### UI-09：鼠标视差效果（首页 Hero）

**问题**：参考 kimi.txt 有鼠标跟随视差，当前实现没有。

**优化**：为首页 Hero 区的巨型装饰添加鼠标跟随。

```javascript
// 仅首页 Hero 区
if (document.getElementById('hero')) {
    document.addEventListener('mousemove', (e) => {
        const x = (e.clientX / window.innerWidth - 0.5) * 20;
        const y = (e.clientY / window.innerHeight - 0.5) * 20;
        document.querySelectorAll('.giant-deco').forEach((el, i) => {
            const factor = (i + 1) * 0.3;
            el.style.transform = `translate(${x * factor}px, ${y * factor}px)`;
        });
    });
}
```

---

### UI-10：404 页面美化

**现状**：404 返回 JSON。

**优化**：创建专门的 404 模板。

```html
<!-- templates/404.html -->
{% extends "base.html" %}
{% block title %}404 · 校园墙{% endblock %}
{% block content %}
<div class="hero-section" style="min-height:60vh;">
    <div style="font-size:6rem;margin-bottom:16px;">😢</div>
    <h1 style="font-family:'ZCOOL KuaiLe',cursive;font-size:3rem;">404</h1>
    <p style="font-family:'Ma Shan Zheng',cursive;font-size:1.3rem;color:var(--text-secondary);">
        迷路了？这个页面走丢了...
    </p>
    <a href="/" class="btn btn-primary btn-lg" style="margin-top:24px;">回到首页</a>
</div>
{% endblock %}
```

---

### UI-11：Doodle 背景性能优化

**问题**：`doodle-star` 的 CSS 动画即使在标签页不可见时也在运行，浪费 GPU。

**优化**：用 `document.hidden` 暂停动画。

```javascript
document.addEventListener('visibilitychange', () => {
    const layer = document.getElementById('doodleLayer');
    if (layer) {
        layer.style.animationPlayState = document.hidden ? 'paused' : 'running';
        layer.querySelectorAll('*').forEach(el => {
            el.style.animationPlayState = document.hidden ? 'paused' : 'running';
        });
    }
});
```

---

### UI-12：输入框 placeholder 动画

**优化**：给搜索框添加聚焦时 placeholder 上浮效果（参考现代 UI）。

```css
.search-box .input:focus + .search-icon {
    color: var(--pink-4);
    transform: translateY(-50%) scale(1.1);
}
```

---

## 四、安全加固

### SEC-01：JWT Secret 硬编码

**现状**：`JWT_SECRET = 'jwt-dev-secret-change-in-production'` 在 `__init__.py` 中。

**风险**：如果部署时忘记修改，任何人都能伪造 token。

**修复**：启动时检查是否为默认值。

```python
def create_app():
    # ...
    jwt_secret = os.environ.get('JWT_SECRET')
    if not jwt_secret or jwt_secret == 'jwt-dev-secret-change-in-production':
        if os.environ.get('FLASK_ENV') == 'production':
            raise RuntimeError('生产环境必须设置 JWT_SECRET 环境变量！')
        jwt_secret = 'dev-only-jwt-secret'
    app.config['JWT_SECRET'] = jwt_secret
```

---

### SEC-02：CORS 过于宽松

**现状**：`CORS(app, supports_credentials=True)` 允许所有来源。

**修复**：生产环境限制来源。

```python
allowed_origins = os.environ.get('CORS_ORIGINS', '*').split(',')
CORS(app, supports_credentials=True, origins=allowed_origins)
```

---

### SEC-03：输入长度限制

**现状**：后端只检查了最小长度，没有最大长度限制。恶意用户可以提交超长内容。

**修复**：

```python
# routes/auth.py
if len(username) > 30:
    return jsonify({'error': '用户名最多30个字符'}), 400
if len(password) > 128:
    return jsonify({'error': '密码过长'}), 400

# routes/posts.py
if len(title) > 100:
    return jsonify({'error': '标题最多100个字符'}), 400
if len(content) > 10000:
    return jsonify({'error': '内容最多10000个字符'}), 400
```

---

### SEC-04：XSS 防护确认

**现状**：前端 `escHtml()` 函数转义了 `& < > "`。但 Jinja2 模板中使用 `{{ }}` 已自动转义。

**确认**：✅ 前后端均有 XSS 防护，无额外问题。

---

## 五、优先级排序

### P0（必须立即修复 — 影响核心功能）
1. BUG-01：toggleLike DOM 覆写
2. BUG-02：loadPosts 全局作用域
3. BUG-06：get_json None 检查
4. BUG-09：发帖弹窗子站列表为空
5. FEAT-04：加载动画每次刷新都播放

### P1（应该修复 — 影响用户体验）
6. BUG-03：leave_station 计数
7. BUG-04：delete_post 计数
8. BUG-05：评论删除权限
9. BUG-07：时区问题
10. UI-03：气泡尾巴在毛玻璃下异常
11. UI-04：Select 样式
12. FEAT-02：个人资料编辑

### P2（建议修复 — 增强体验）
13. UI-02：子站卡片多样性
14. UI-05：Toast 样式
15. UI-06：页面过渡
16. UI-09：鼠标视差
17. UI-10：404 页面
18. FEAT-01：图片上传
19. FEAT-07：搜索防抖

### P3（长期改进）
20. SEC-01/02：安全加固
21. FEAT-08：成员管理
22. FEAT-10：密码修改
23. UI-11：性能优化
