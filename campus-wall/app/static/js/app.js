/**
 * 应用主逻辑 — 用户状态、模态框、表单处理、全部交互功能
 */

let currentUser = null;
let _userReady = false;
const _userReadyCbs = [];

// 用户状态就绪后回调（解决 DOMContentLoaded 异步加载竞态）
function onUserReady(cb) {
    if (_userReady) { cb(); return; }
    _userReadyCbs.push(cb);
}

// ── 初始化 ──
document.addEventListener('DOMContentLoaded', async function() {
    await initAnimations();
    await loadCurrentUser();
    _userReady = true;
    _userReadyCbs.forEach(cb => { try { cb(); } catch (e) {} });
    _userReadyCbs.length = 0;
    updateNavRight();
    updateNotifBadge();
    initMouseParallax();
});

// ── 滚动时导航栏变实 ──
window.addEventListener('scroll', () => {
    const nav = document.getElementById('navbar');
    if (nav) nav.classList.toggle('scrolled', window.scrollY > 60);
}, { passive: true });

// ── 用户状态 ──
async function loadCurrentUser() {
    const token = api.getToken();
    if (!token) return;
    try {
        const data = await api.get('/api/auth/me');
        currentUser = data;
    } catch (e) {
        api.setToken(null);
        currentUser = null;
    }
}

function updateNavRight() {
    const navRight = document.getElementById('navRight');
    if (!navRight) return;
    if (currentUser) {
        navRight.innerHTML = `
            <a href="/create" class="btn btn-primary btn-sm" style="text-decoration:none;">✍️ 发帖</a>
            <a href="/notifications" class="btn btn-ghost btn-sm" style="position:relative;text-decoration:none;" id="notifLink">
                🔔<span class="badge" id="notifBadge" style="display:none;"></span>
            </a>
            <div class="user-chip" onclick="toggleUserMenu()">
                <img src="${currentUser.avatar || '/static/images/default-avatar.svg'}" onerror="this.src='/static/images/default-avatar.svg'" style="width:28px;height:28px;border-radius:50%;">
                <span style="font-size:0.85rem;font-weight:500;">${escHtml(currentUser.username)}</span>
            </div>
            <div id="userMenu" style="display:none;position:absolute;top:100%;right:0;margin-top:8px;background:var(--cream);border:2px solid var(--text-dark);border-radius:16px;padding:8px;box-shadow:5px 5px 0 var(--pink-2);min-width:160px;z-index:200;">
                <a href="/profile/${currentUser.id}" style="display:block;padding:8px 12px;border-radius:10px;font-size:0.85rem;color:var(--text-dark);">👤 个人中心</a>
                <a href="/notifications" style="display:block;padding:8px 12px;border-radius:10px;font-size:0.85rem;color:var(--text-dark);">🔔 通知</a>
                <a href="/create-station" style="display:block;padding:8px 12px;border-radius:10px;font-size:0.85rem;color:var(--text-dark);">🏗️ 创建子站</a>
                <a href="#" onclick="openEditProfile();return false;" style="display:block;padding:8px 12px;border-radius:10px;font-size:0.85rem;color:var(--text-dark);">✏️ 编辑资料</a>
                <a href="#" onclick="openChangePassword();return false;" style="display:block;padding:8px 12px;border-radius:10px;font-size:0.85rem;color:var(--text-dark);">🔑 修改密码</a>
                <hr style="border:none;border-top:1px solid rgba(139,125,107,0.1);margin:4px 0;">
                <a href="#" onclick="handleLogout();return false;" style="display:block;padding:8px 12px;border-radius:10px;font-size:0.85rem;color:var(--pink-4);">🚪 退出登录</a>
            </div>
        `;
    } else {
        navRight.innerHTML = `
            <button class="btn btn-sm" onclick="openModal('loginModal')">登录</button>
            <button class="btn btn-primary btn-sm" onclick="openModal('registerModal')">注册</button>
        `;
    }
}

function toggleUserMenu() {
    const menu = document.getElementById('userMenu');
    if (menu) menu.style.display = menu.style.display === 'none' ? '' : 'none';
}

// 点击外部关闭菜单
document.addEventListener('click', function(e) {
    const menu = document.getElementById('userMenu');
    const chip = e.target.closest('.user-chip');
    if (menu && !chip && !menu.contains(e.target)) {
        menu.style.display = 'none';
    }
});

async function updateNotifBadge() {
    if (!currentUser) return;
    try {
        const data = await api.get('/api/social/notifications/unread-count');
        const badge = document.getElementById('notifBadge');
        if (badge) {
            if (data.count > 0) {
                badge.textContent = data.count > 99 ? '99+' : data.count;
                badge.style.display = '';
            } else {
                badge.style.display = 'none';
            }
        }
    } catch (e) {}
}

// ── 模态框 ──
function openModal(id) {
    const modal = document.getElementById(id);
    if (modal) {
        modal.classList.add('show');
        document.body.style.overflow = 'hidden';
        if (id === 'createPostModal') loadStationOptions('postStationSelect');
    }
}

function closeModal(id) {
    const modal = document.getElementById(id);
    if (modal) {
        modal.classList.remove('show');
        document.body.style.overflow = '';
    }
}

function switchModal(from, to) {
    closeModal(from);
    setTimeout(() => openModal(to), 200);
}

// 点击遮罩关闭
document.addEventListener('click', function(e) {
    if (e.target.classList.contains('modal-overlay') && e.target.classList.contains('show')) {
        e.target.classList.remove('show');
        document.body.style.overflow = '';
    }
});

// ESC 关闭
document.addEventListener('keydown', function(e) {
    if (e.key === 'Escape') {
        document.querySelectorAll('.modal-overlay.show').forEach(m => m.classList.remove('show'));
        document.body.style.overflow = '';
    }
});

// ── 认证表单 ──
function handleLogin(e) {
    e.preventDefault();
    const form = e.target;
    api.post('/api/auth/login', {
        username: form.username.value.trim(),
        password: form.password.value
    }).then(res => {
        api.setToken(res.token);
        currentUser = res.user;
        closeModal('loginModal');
        updateNavRight();
        showToast('欢迎回来，' + res.user.username + '！', 'success');
        form.reset();
    }).catch(err => showToast(err.message || '登录失败', 'error'));
    return false;
}

function handleRegister(e) {
    e.preventDefault();
    const form = e.target;
    api.post('/api/auth/register', {
        username: form.username.value.trim(),
        email: form.email.value.trim(),
        password: form.password.value
    }).then(res => {
        api.setToken(res.token);
        currentUser = res.user;
        closeModal('registerModal');
        updateNavRight();
        showToast('注册成功，欢迎 ' + res.user.username + '！', 'success');
        form.reset();
    }).catch(err => showToast(err.message || '注册失败', 'error'));
    return false;
}

function handleLogout() {
    api.setToken(null);
    currentUser = null;
    updateNavRight();
    showToast('已退出登录');
    if (window.location.pathname.startsWith('/profile/') ||
        window.location.pathname.startsWith('/notifications')) {
        window.location.href = '/';
    }
}

function requireAuth() {
    if (!currentUser) {
        openModal('loginModal');
        return false;
    }
    return true;
}

// ── 发帖表单 ──
function handleCreatePost(e) {
    e.preventDefault();
    const form = e.target;
    const data = {
        station_id: parseInt(form.station_id.value),
        title: form.title.value.trim(),
        content: form.content.value.trim()
    };
    if (!data.station_id || !data.title || !data.content) {
        showToast('请填写完整信息', 'error');
        return false;
    }
    api.post('/api/posts', data).then(res => {
        closeModal('createPostModal');
        showToast('发帖成功！', 'success');
        form.reset();
        document.dispatchEvent(new CustomEvent('postCreated'));
    }).catch(err => showToast(err.message || '发帖失败', 'error'));
    return false;
}

// ── 创建子站表单 ──
function handleCreateStation(e) {
    e.preventDefault();
    const form = e.target;
    const tags = form.tags.value.split(',').map(t => t.trim()).filter(Boolean);
    const data = {
        name: form.name.value.trim(),
        icon: form.icon.value.trim() || '🏫',
        description: form.description.value.trim(),
        tags: tags
    };
    if (!data.name) { showToast('请输入子站名称', 'error'); return false; }
    api.post('/api/stations', data).then(res => {
        closeModal('createStationModal');
        showToast('创建成功！', 'success');
        form.reset();
        window.location.href = '/station/' + res.station.id;
    }).catch(err => showToast(err.message || '创建失败', 'error'));
    return false;
}

// ── 编辑资料 ──
function openEditProfile() {
    if (!requireAuth()) return;
    const modal = document.getElementById('editProfileModal');
    if (!modal) return;
    closeUserMenu();
    modal.querySelector('[name="username"]').value = currentUser.username;
    modal.querySelector('[name="bio"]').value = currentUser.bio || '';
    const avatarPreview = document.getElementById('editAvatarPreview');
    if (avatarPreview) avatarPreview.src = currentUser.avatar || '/static/images/default-avatar.svg';
    openModal('editProfileModal');
}

function handleEditProfile(e) {
    e.preventDefault();
    const form = e.target;
    const data = {
        username: form.username.value.trim(),
        bio: form.bio.value.trim()
    };
    // Handle avatar upload if file selected
    const fileInput = form.avatar;
    if (fileInput && fileInput.files.length > 0) {
        const formData = new FormData();
        formData.append('file', fileInput.files[0]);
        fetch('/api/auth/avatar', {
            method: 'POST',
            headers: { 'Authorization': 'Bearer ' + api.getToken() },
            body: formData
        }).then(r => r.json()).then(res => {
            if (res.url) {
                data.avatar = res.url;
                saveProfile(data, form);
            } else {
                showToast(res.error || '头像上传失败', 'error');
            }
        }).catch(() => showToast('头像上传失败', 'error'));
    } else {
        saveProfile(data, form);
    }
    return false;
}

function saveProfile(data, form) {
    api.put('/api/auth/me', data).then(res => {
        currentUser = res.user;
        closeModal('editProfileModal');
        updateNavRight();
        showToast('资料更新成功！', 'success');
        form.reset();
        if (typeof loadProfile === 'function') loadProfile();
    }).catch(err => showToast(err.message || '更新失败', 'error'));
}

// ── 修改密码 ──
function openChangePassword() {
    if (!requireAuth()) return;
    closeUserMenu();
    const form = document.getElementById('changePasswordForm');
    if (form) form.reset();
    openModal('changePasswordModal');
}

function handleChangePassword(e) {
    e.preventDefault();
    const form = e.target;
    const oldPw = form.old_password.value;
    const newPw = form.new_password.value;
    const confirmPw = form.confirm_password.value;
    if (newPw !== confirmPw) { showToast('两次密码不一致', 'error'); return false; }
    api.post('/api/auth/change-password', {
        old_password: oldPw,
        new_password: newPw
    }).then(res => {
        closeModal('changePasswordModal');
        showToast('密码修改成功！', 'success');
        form.reset();
    }).catch(err => showToast(err.message || '修改失败', 'error'));
    return false;
}

// ── 编辑帖子 ──
function openEditPost(postId) {
    if (!requireAuth()) return;
    api.get('/api/posts/' + postId).then(post => {
        const modal = document.getElementById('editPostModal');
        if (!modal) return;
        modal.querySelector('[name="post_id"]').value = postId;
        modal.querySelector('[name="title"]').value = post.title;
        modal.querySelector('[name="content"]').value = post.content;
        openModal('editPostModal');
    }).catch(() => showToast('加载失败', 'error'));
}

function handleEditPost(e) {
    e.preventDefault();
    const form = e.target;
    const postId = form.post_id.value;
    const data = {};
    if (form.title.value.trim()) data.title = form.title.value.trim();
    if (form.content.value.trim()) data.content = form.content.value.trim();
    api.put('/api/posts/' + postId, data).then(() => {
        closeModal('editPostModal');
        showToast('更新成功！', 'success');
        if (typeof loadPost === 'function') loadPost();
    }).catch(err => showToast(err.message || '更新失败', 'error'));
    return false;
}

// ── 图片上传（帖子内）──
function uploadPostImage(fileInput) {
    if (!fileInput.files.length) return;
    const file = fileInput.files[0];
    if (file.size > 16 * 1024 * 1024) { showToast('文件不能超过16MB', 'error'); return; }
    const formData = new FormData();
    formData.append('file', file);
    showToast('上传中...');
    fetch('/api/posts/upload-image', {
        method: 'POST',
        headers: { 'Authorization': 'Bearer ' + api.getToken() },
        body: formData
    }).then(r => r.json()).then(res => {
        if (res.url) {
            showToast('上传成功！', 'success');
            // Insert image URL into content textarea
            const textarea = document.querySelector('#createPostForm [name="content"], #editPostForm [name="content"]');
            if (textarea) {
                textarea.value += (textarea.value ? '\n' : '') + '![图片](' + res.url + ')';
            }
        } else {
            showToast(res.error || '上传失败', 'error');
        }
    }).catch(() => showToast('上传失败', 'error'));
}

// ── 点赞 ──
function toggleLike(postId, btn) {
    if (!requireAuth()) return;
    const currentLikes = parseInt(btn.dataset.likes || btn.textContent.replace(/[^\d]/g, '')) || 0;
    api.post('/api/posts/' + postId + '/like').then(data => {
        const newLikes = data.liked ? currentLikes + 1 : Math.max(0, currentLikes - 1);
        btn.dataset.likes = newLikes;
        btn.className = 'post-action' + (data.liked ? ' liked' : '');
        btn.innerHTML = `<span class="icon">${data.liked ? '❤️' : '🤍'}</span> ${newLikes}`;
    }).catch(e => showToast(e.message || '操作失败', 'error'));
}

function toggleCommentLike(commentId, btn) {
    if (!requireAuth()) return;
    const currentLikes = parseInt(btn.dataset.likes || btn.textContent.replace(/[^\d]/g, '')) || 0;
    api.post('/api/social/comment/' + commentId + '/like').then(data => {
        const newLikes = data.liked ? currentLikes + 1 : Math.max(0, currentLikes - 1);
        btn.dataset.likes = newLikes;
        btn.className = 'post-action btn-ghost' + (data.liked ? ' liked' : '');
        btn.innerHTML = `<span class="icon">${data.liked ? '❤️' : '🤍'}</span> ${newLikes}`;
    }).catch(e => showToast(e.message || '操作失败', 'error'));
}

// ── 评论回复 ──
function replyToCommentById(el) {
    replyToComment(parseInt(el.dataset.cid, 10), el.dataset.author || '');
}

function replyToComment(commentId, authorName) {
    const textarea = document.getElementById('commentContent');
    if (!textarea) return;
    textarea.value = `@${authorName} `;
    textarea.dataset.parentId = commentId;
    textarea.focus();
    textarea.scrollIntoView({ behavior: 'smooth', block: 'center' });
}

function submitComment(e) {
    e.preventDefault();
    if (!requireAuth()) return false;
    const textarea = document.getElementById('commentContent');
    const content = textarea.value.trim();
    if (!content) return false;
    const postId = typeof POST_ID !== 'undefined' ? POST_ID : null;
    if (!postId) return false;
    const body = { content };
    if (textarea.dataset.parentId) body.parent_id = parseInt(textarea.dataset.parentId);
    api.post('/api/posts/' + postId + '/comments', body).then(() => {
        showToast('评论成功', 'success');
        textarea.value = '';
        delete textarea.dataset.parentId;
        if (typeof loadComments === 'function') loadComments();
    }).catch(err => showToast(err.message || '评论失败', 'error'));
    return false;
}

// ── 子站选择器加载 ──
function loadStationOptions(selectId) {
    api.get('/api/stations?limit=100').then(data => {
        const sel = document.getElementById(selectId);
        if (!sel) return;
        const current = sel.value;
        sel.innerHTML = '<option value="">请选择子站...</option>';
        (data || []).forEach(s => {
            sel.innerHTML += `<option value="${s.id}">${s.icon||'🏫'} ${escHtml(s.name)}</option>`;
        });
        if (current) sel.value = current;
    }).catch(() => {});
}

// ── 搜索防抖 ──
let _searchTimer = null;
function debouncedSearch(inputId, callback, delay) {
    delay = delay || 350;
    const input = document.getElementById(inputId);
    if (!input) return;
    input.addEventListener('input', () => {
        clearTimeout(_searchTimer);
        _searchTimer = setTimeout(() => callback(input.value.trim()), delay);
    });
    input.addEventListener('keydown', (e) => {
        if (e.key === 'Enter') {
            clearTimeout(_searchTimer);
            callback(input.value.trim());
        }
    });
}

// ── 看板娘 ──
function toggleKanban() {
    const bubble = document.getElementById('kanbanBubble');
    if (!bubble) return;
    if (bubble.style.display === 'none') {
        api.get('/api/kanban/message').then(data => {
            document.getElementById('kanbanText').textContent = data.message;
            bubble.style.display = '';
            setTimeout(() => { bubble.style.display = 'none'; }, 8000);
        });
    } else {
        bubble.style.display = 'none';
    }
}

// 页面加载后延迟显示看板娘
setTimeout(() => {
    const bubble = document.getElementById('kanbanBubble');
    if (bubble) {
        api.get('/api/kanban/message').then(data => {
            document.getElementById('kanbanText').textContent = data.message;
            bubble.style.display = '';
            setTimeout(() => { bubble.style.display = 'none'; }, 6000);
        }).catch(() => {});
    }
}, 3000);

// ── 鼠标视差（首页 Hero）──
function initMouseParallax() {
    const hero = document.getElementById('hero');
    if (!hero) return;
    document.addEventListener('mousemove', (e) => {
        const x = (e.clientX / window.innerWidth - 0.5) * 20;
        const y = (e.clientY / window.innerHeight - 0.5) * 20;
        hero.querySelectorAll('.giant-deco').forEach((el, i) => {
            const factor = (i + 1) * 0.3;
            el.style.transform = `translate(${x * factor}px, ${y * factor}px)`;
        });
    });
}

// ── 辅助 ──
function closeUserMenu() {
    const menu = document.getElementById('userMenu');
    if (menu) menu.style.display = 'none';
}

// ── 工具函数 ──
function escHtml(str) {
    if (!str) return '';
    return String(str).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

// 渲染帖子内容：转义 HTML，并将 Markdown 图片 ![alt](url) 转为安全的 <img>
function renderContent(content, full) {
    if (!content) return '';
    let html = escHtml(content);
    // 将 ![alt](url) 转为 <img>
    html = html.replace(/!\[([^\]]*)\]\(([^)]+)\)/g, function(m, alt, url) {
        const u = String(url).replace(/&amp;/g, '&').trim();
        // 只允许站内静态图片路径
        if (!/^\/static\//.test(u)) return m;
        return '<img src="' + escHtml(u) + '" alt="' + escHtml(alt) + '" loading="lazy" class="post-image" style="max-height:400px;border-radius:14px;margin:8px 0;display:block;">';
    });
    if (full) {
        return html;
    }
    // 摘要模式：转成纯文本（去掉图片语法）
    return html.replace(/<img[^>]*>/g, '');
}

// 帖子卡片（全局复用：首页 / 子站页 / 个人主页等）
function renderPostCard(p) {
    const time = formatTime(p.created_at);
    const imgMatch = String(p.content || '').match(/!\[[^\]]*\]\(([^)]+)\)/);
    const coverImg = imgMatch && /^\/static\//.test(imgMatch[1]) ? imgMatch[1] : '';
    return `
        <article class="post-card reveal" onclick="window.location.href='/post/${p.id}'">
            <div class="post-header">
                <img class="post-avatar" src="${p.author_avatar || '/static/images/default-avatar.svg'}" alt="" onerror="this.src='/static/images/default-avatar.svg'">
                <div class="post-meta">
                    <div class="post-author">${escHtml(p.author_name)}</div>
                    <div class="post-time">${time}</div>
                </div>
                <span class="post-station">${p.station_icon||'🏫'} ${escHtml(p.station_name||'')}</span>
            </div>
            <div class="post-title">${escHtml(p.title)}</div>
            ${coverImg ? '<img class="post-image" src="'+escHtml(coverImg)+'" alt="" loading="lazy" style="max-height:260px;width:100%;object-fit:cover;border-radius:14px;margin:8px 0;">' : ''}
            <div class="post-content">${renderContent(p.content, false)}</div>
            <div class="post-actions">
                <button class="post-action ${p.is_liked?'liked':''}" onclick="event.stopPropagation();toggleLike(${p.id},this)">
                    <span class="icon">${p.is_liked?'❤️':'🤍'}</span> ${p.likes_count||0}
                </button>
                <span class="post-action"><span class="icon">💬</span> ${p.comments_count||0}</span>
                <span class="post-action"><span class="icon">👁️</span> ${p.views||0}</span>
            </div>
        </article>
    `;
}

function formatTime(timestamp) {
    if (!timestamp) return '刚刚';
    try {
        let date;
        if (typeof timestamp === 'string' && !timestamp.includes('T')) {
            date = new Date(timestamp + 'Z');
        } else {
            date = new Date(timestamp);
        }
        if (isNaN(date.getTime())) return '刚刚';
        const now = new Date();
        const diff = Math.floor((now - date) / 1000);
        if (diff < 60) return '刚刚';
        if (diff < 3600) return Math.floor(diff / 60) + '分钟前';
        if (diff < 86400) return Math.floor(diff / 3600) + '小时前';
        if (diff < 2592000) return Math.floor(diff / 86400) + '天前';
        return date.toLocaleDateString('zh-CN');
    } catch { return '刚刚'; }
}

function showToast(message, type) {
    type = type || '';
    const toast = document.getElementById('toast');
    if (!toast) return;
    toast.textContent = message;
    toast.className = 'toast show' + (type ? ' ' + type : '');
    clearTimeout(toast._timer);
    toast._timer = setTimeout(() => toast.classList.remove('show'), 2500);
}
