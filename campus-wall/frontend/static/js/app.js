/**
 * 应用主逻辑 — 用户状态、模态框、表单处理、全部交互功能
 *
 * 注意：currentUser / _userReady / _userReadyCbs / onUserReady / loadCurrentUser
 * 等全局状态与函数由 auth.js（先加载）提供，此处不得重复声明，否则 let/const
 * 重复声明会触发 SyntaxError 导致整个脚本不执行。
 */

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

// ── 模态框 ──
function openModal(id) {
    const modal = document.getElementById(id);
    if (modal) {
        modal.classList.add('show');
        document.body.style.overflow = 'hidden';
        if (id === 'createPostModal') {
            loadStationOptions('postStationSelect');
            initPostTypeSelector();
        }
        if (id === 'createStationModal') {
            initIconSelector();
        }
    }
}

function initPostTypeSelector() {
    const selector = document.getElementById('postTypeSelector');
    if (!selector) return;
    const options = selector.querySelectorAll('.post-type-option');
    const linkGroup = document.getElementById('linkUrlGroup');
    const voteGroup = document.getElementById('voteOptionsGroup');
    options.forEach(opt => {
        opt.onclick = function() {
            options.forEach(o => o.classList.remove('active'));
            this.classList.add('active');
            const type = this.dataset.type;
            if (linkGroup) linkGroup.style.display = type === 'link' ? '' : 'none';
            if (voteGroup) voteGroup.style.display = type === 'vote' ? '' : 'none';
        };
    });
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

// ── 发帖表单 ──
function handleCreatePost(e) {
    e.preventDefault();
    const form = e.target;
    const postType = form.querySelector('input[name="post_type"]:checked')?.value || 'text';
    const data = {
        station_id: parseInt(form.station_id.value),
        title: form.title.value.trim(),
        content: form.content.value.trim(),
        post_type: postType,
        is_anonymous: form.is_anonymous && form.is_anonymous.checked ? 1 : 0
    };
    if (postType === 'link') {
        data.link_url = form.link_url?.value.trim() || '';
    }
    if (postType === 'vote') {
        const options = form.vote_options?.value.split('\n').map(s => s.trim()).filter(Boolean) || [];
        data.vote_options = options;
    }
    if (!data.station_id || !data.title || !data.content) {
        CampusUtils.showToast('请填写完整信息', 'error');
        return false;
    }
    api.post('/api/posts', data).then(res => {
        closeModal('createPostModal');
        CampusUtils.showToast('发帖成功！', 'success');
        form.reset();
        document.dispatchEvent(new CustomEvent('postCreated'));
    }).catch(err => CampusUtils.showToast(err.message || '发帖失败', 'error'));
    return false;
}

// ── 封面预览 ──
function previewCover(input) {
    const preview = document.getElementById('coverPreview');
    if (!preview) return;
    if (input.files && input.files[0]) {
        const reader = new FileReader();
        reader.onload = function(e) {
            preview.src = e.target.result;
            preview.style.display = 'block';
        };
        reader.readAsDataURL(input.files[0]);
    }
}

// ── 图标选择器初始化 ──
function initIconSelector() {
    const selector = document.getElementById('iconSelector');
    if (!selector) return;
    const options = selector.querySelectorAll('.icon-option');
    options.forEach(opt => {
        opt.onclick = function() {
            options.forEach(o => o.classList.remove('active'));
            this.classList.add('active');
        };
    });
}

// ── 创建子站表单 ──
function handleCreateStation(e) {
    e.preventDefault();
    const form = e.target;
    const tags = form.tags.value.split(',').map(t => t.trim()).filter(Boolean);
    const iconInput = form.querySelector('input[name="icon"]:checked');
    const data = {
        name: form.name.value.trim(),
        icon: iconInput ? iconInput.value : 'school',
        description: form.description.value.trim(),
        tags: tags
    };
    if (!data.name) { CampusUtils.showToast('请输入子站名称', 'error'); return false; }
    api.post('/api/stations', data).then(res => {
        closeModal('createStationModal');
        CampusUtils.showToast('创建成功！', 'success');
        form.reset();
        window.location.href = '/station/' + res.station.id;
    }).catch(err => CampusUtils.showToast(err.message || '创建失败', 'error'));
    return false;
}

// ── 编辑资料 ──
// 编辑资料：委托给 modal.js 的实现（支持头像/签名/称号）
function openEditProfile() {
    if (!requireAuth()) return;
    closeUserMenu();
    window.CampusModal.openEditProfile();
}

async function handleEditProfile(event) {
    const done = await window.CampusModal.handleEditProfile(event);
    if (done !== false && typeof loadProfile === 'function') loadProfile();
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

function openForgotPassword() {
    closeModal('loginModal');
    setTimeout(() => openModal('forgotPasswordModal'), 200);
}

function handleForgotPassword(e) {
    e.preventDefault();
    const form = e.target;
    api.post('/api/auth/forgot-password', { email: form.email.value.trim() })
        .then(res => {
            closeModal('forgotPasswordModal');
            if (res.reset_token) {
                // 开发模式：显示重置链接
                const resetUrl = window.location.origin + '/reset-password?token=' + res.reset_token;
                showToast('已生成重置链接（开发模式）', 'success');
                setTimeout(() => {
                    const ok = confirm('重置链接已生成。\n\n开发模式下请复制此链接使用：\n' + resetUrl + '\n\n点击确定复制');
                    if (ok) {
                        const ta = document.createElement('textarea');
                        ta.value = resetUrl;
                        document.body.appendChild(ta);
                        ta.select();
                        try { document.execCommand('copy'); } catch (e) {}
                        document.body.removeChild(ta);
                        showToast('已复制重置链接', 'success');
                    }
                }, 500);
            } else {
                showToast(res.message || '如果该邮箱已注册，重置链接已发送', 'success');
            }
            form.reset();
        })
        .catch(err => showToast(err.message || '发送失败', 'error'));
    return false;
}

// ── 修改密码 ──
function openChangePassword() {
    if (!requireAuth()) return;
    closeUserMenu();
    window.CampusModal.openChangePassword();
}

function handleChangePassword(e) {
    window.CampusModal.handleChangePassword(e);
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
    if (file.size > 5 * 1024 * 1024) { showToast('图片不能超过5MB', 'error'); return; }
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
            // 优先定位触发上传的表单内的正文框（兼容弹窗与 /create 独立页）
            const textarea = (fileInput.closest('form') || document).querySelector('[name="content"]')
                || document.querySelector('#createPostForm [name="content"], #editPostForm [name="content"], #createPostPageForm [name="content"]');
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
        btn.innerHTML = `<i data-lucide="${data.liked ? 'heart' : 'heart'}" class="icon icon-sm"></i> ${newLikes}`;
        if (btn.classList.contains('liked')) btn.classList.add('liked');
        if (window.lucide) lucide.createIcons();
    }).catch(e => showToast(e.message || '操作失败', 'error'));
}

function toggleCommentLike(commentId, btn) {
    if (!requireAuth()) return;
    const currentLikes = parseInt(btn.dataset.likes || btn.textContent.replace(/[^\d]/g, '')) || 0;
    api.post('/api/social/comment/' + commentId + '/like').then(data => {
        const newLikes = data.liked ? currentLikes + 1 : Math.max(0, currentLikes - 1);
        btn.dataset.likes = newLikes;
        btn.className = 'post-action btn-ghost' + (data.liked ? ' liked' : '');
        btn.innerHTML = `<i data-lucide="heart" class="icon icon-sm"></i> ${newLikes}`;
        if (window.lucide) lucide.createIcons();
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
            const iconHtml = `<i data-lucide="${getStationIcon(s.icon)}" class="icon icon-sm"></i>`;
            sel.innerHTML += `<option value="${s.id}">${iconHtml} ${escHtml(s.name)}</option>`;
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
    return String(str).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;').replace(/'/g, '&#39;');
}

// 渲染帖子内容：转义 HTML，并将 Markdown 图片 ![alt](url) 转为安全的 <img>
function renderContent(content, full) {
    if (!content) return '';
    let html = CampusUtils.escHtml(content);
    // 将 ![alt](url) 转为 <img>
    html = html.replace(/!\[([^\]]*)\]\(([^)]+)\)/g, function(m, alt, url) {
        const u = String(url).replace(/&amp;/g, '&').trim();
        // 支持站内静态图片、上传图片、CDN图片
        if (/^\/static\/|^\/uploads\/|^https?:\/\//.test(u)) {
            return '<img src="' + CampusUtils.escHtml(u) + '" alt="' + CampusUtils.escHtml(alt) + '" loading="lazy" class="post-image" style="max-height:400px;width:100%;object-fit:cover;border-radius:14px;margin:8px 0;display:block;">';
        }
        return m;
    });
    // 将 HTML img 标签也处理（支持已有 HTML 格式）
    html = html.replace(/<img\s+[^>]*src=["']([^"']+)["'][^>]*>/gi, function(m, url) {
        const u = String(url).replace(/&amp;/g, '&').trim();
        if (/^\/static\/|^\/uploads\/|^https?:\/\//.test(u)) {
            return '<img src="' + CampusUtils.escHtml(u) + '" loading="lazy" class="post-image" style="max-height:400px;width:100%;object-fit:cover;border-radius:14px;margin:8px 0;display:block;">';
        }
        return m;
    });
    if (full) {
        return html;
    }
    // 摘要模式：转成纯文本（去掉图片语法）
    return html.replace(/<img[^>]*>/g, '');
}

// 帖子卡片（全局复用：首页 / 子站页 / 个人主页等）
function renderPostCard(p) {
    const time = CampusUtils.formatTime(p.created_at);
    const imgMatch = String(p.content || '').match(/!\[[^\]]*\]\(([^)]+)\)/);
    const coverImg = imgMatch && /^\/static\//.test(imgMatch[1]) ? imgMatch[1] : '';
    const cardClass = `post-card reveal${p.is_pinned ? ' pinned' : ''}${p.is_featured ? ' featured' : ''}`;

    const typeMap = {
        text: { label: '图文', color: 'var(--pink-4)', bg: 'rgba(255,181,186,0.08)', icon: 'file-text' },
        link: { label: '链接', color: 'var(--blue)', bg: 'rgba(147,197,253,0.08)', icon: 'link' },
        vote: { label: '投票', color: 'var(--orange)', bg: 'rgba(255,214,165,0.08)', icon: 'bar-chart-2' },
    };
    const postType = p.post_type || 'text';
    const typeInfo = typeMap[postType] || typeMap.text;

    const identityGroup = p.author_identity_group || '';
    const groupBadge = identityGroup ? `<span class="identity-badge" style="display:inline-flex;align-items:center;gap:2px;padding:1px 6px;border-radius:8px;font-size:0.65rem;font-weight:600;background:var(--lavender);color:var(--text-dark);"><i data-lucide="shield" class="icon" style="width:10px;height:10px;"></i> ${CampusUtils.escHtml(identityGroup)}</span>` : '';

    return `
        <article class="${cardClass}" style="${postType !== 'text' ? 'background:' + typeInfo.bg + ';' : ''}" onclick="window.location.href='/post/${p.id}'">
            <div class="post-header">
                <img class="post-avatar" src="${CampusUtils.safeUrl(p.author_avatar, '/static/images/default-avatar.svg')}" alt="" onerror="this.src='/static/images/default-avatar.svg'">
                <div class="post-meta">
                    <div class="post-author">${CampusUtils.escHtml(p.author_name)} ${groupBadge}</div>
                    <div class="post-time">${time}</div>
                </div>
                <div style="display:flex;align-items:center;gap:8px;">
                    <span class="post-type-badge" style="display:inline-flex;align-items:center;gap:3px;padding:2px 8px;border-radius:10px;font-size:0.7rem;font-weight:600;color:${typeInfo.color};background:${typeInfo.bg};border:1px solid ${typeInfo.color}30;"><i data-lucide="${typeInfo.icon}" class="icon" style="width:12px;height:12px;"></i> ${typeInfo.label}</span>
                    <span class="post-station"><i data-lucide="${getStationIcon(p.station_icon)}" class="icon icon-sm"></i> ${CampusUtils.escHtml(p.station_name||'')}</span>
                </div>
            </div>
            ${p.is_pinned ? '<div style="display:flex;align-items:center;gap:4px;margin-bottom:8px;"><i data-lucide="pin" class="icon icon-sm icon-danger"></i> <span style="font-size:0.75rem;color:var(--color-accent);font-weight:600;">置顶</span></div>' : ''}
            <div class="post-title">${CampusUtils.escHtml(p.title)}</div>
            ${coverImg ? '<img class="post-image" src="'+CampusUtils.escHtml(coverImg)+'" alt="" loading="lazy" style="max-height:260px;width:100%;object-fit:cover;border-radius:14px;margin:8px 0;">' : ''}
            <div class="post-content">${renderContent(p.content, false)}</div>
            <div class="post-actions">
                <button class="post-action ${p.is_liked?'liked':''}" onclick="event.stopPropagation();toggleLike(${p.id},this)">
                    <i data-lucide="heart" class="icon icon-sm ${p.is_liked ? 'icon-danger' : ''}"></i> ${p.likes_count||0}
                </button>
                <span class="post-action"><i data-lucide="message-circle" class="icon icon-sm"></i> ${p.comments_count||0}</span>
                <span class="post-action"><i data-lucide="eye" class="icon icon-sm"></i> ${p.views||0}</span>
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

// ── 投票功能 ──
function renderVoteSection(p) {
    const options = p.vote_options ? p.vote_options.split('\n').filter(o => o.trim()) : [];
    if (!options.length) return '';
    const totalVotes = p.vote_counts ? Object.values(p.vote_counts).reduce((a, b) => a + b, 0) : 0;
    const hasVoted = p.user_voted;
    return `
        <div class="vote-section" style="margin:16px 0;padding:16px;background:var(--bg-secondary);border-radius:var(--radius-lg);border:1px solid var(--border-primary);">
            <div style="display:flex;align-items:center;gap:8px;margin-bottom:12px;font-weight:600;">
                <i data-lucide="bar-chart-2" class="icon icon-sm"></i>
                <span>投票</span>
                <span style="font-size:0.75rem;color:var(--text-muted);font-weight:400;">${totalVotes} 票</span>
            </div>
            <div id="voteOptions">
                ${options.map((opt, i) => {
                    const votes = p.vote_counts ? (p.vote_counts[i] || 0) : 0;
                    const pct = totalVotes > 0 ? Math.round(votes / totalVotes * 100) : 0;
                    if (hasVoted) {
                        return `<div class="vote-option voted" style="position:relative;overflow:hidden;margin:6px 0;padding:10px 12px;border-radius:var(--radius-md);background:var(--bg-primary);border:1px solid var(--border-primary);">
                            <div style="position:absolute;left:0;top:0;bottom:0;width:${pct}%;background:linear-gradient(90deg,rgba(255,181,186,0.2),rgba(255,214,165,0.2));transition:width 0.5s;"></div>
                            <div style="position:relative;display:flex;justify-content:space-between;align-items:center;">
                                <span>${CampusUtils.escHtml(opt)}</span>
                                <span style="font-size:0.8rem;color:var(--text-muted);">${votes} 票 (${pct}%)</span>
                            </div>
                        </div>`;
                    }
                    return `<button class="vote-option" onclick="submitVote(${p.id},${i})" style="display:block;width:100%;text-align:left;margin:6px 0;padding:10px 12px;border-radius:var(--radius-md);background:var(--bg-primary);border:1px solid var(--border-primary);cursor:pointer;transition:all 0.2s;font-size:inherit;color:inherit;">
                        <i data-lucide="circle" class="icon icon-sm"></i> ${CampusUtils.escHtml(opt)}
                    </button>`;
                }).join('')}
            </div>
        </div>
    `;
}

function submitVote(postId, optionIndex) {
    if (!currentUser) { openModal('loginModal'); return; }
    api.post(`/api/posts/${postId}/vote`, { option_index: optionIndex }).then(() => {
        showToast('投票成功', 'success');
        loadPost();
    }).catch(err => showToast(err.message || '投票失败', 'error'));
}

// ── 分享功能 ──
function sharePost(postId) {
    const url = window.location.origin + '/post/' + postId;
    if (navigator.share) {
        navigator.share({ title: document.title, url: url });
    } else if (navigator.clipboard) {
        navigator.clipboard.writeText(url).then(() => showToast('链接已复制', 'success'));
    }
}
