/**
 * CampusWall 认证模块
 */

let currentUser = null;
let _userReady = false;
const _userReadyCbs = [];

function onUserReady(cb) {
    if (_userReady) { cb(); return; }
    _userReadyCbs.push(cb);
}

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

function requireAuth() {
    if (!currentUser) {
        openModal('loginModal');
        return false;
    }
    return true;
}

async function handleLogin(event) {
    event.preventDefault();
    const form = event.target;
    const data = {
        username: form.username.value.trim(),
        password: form.password.value
    };
    if (!data.username || !data.password) {
        CampusUtils.showToast('请填写完整信息', 'error');
        return false;
    }
    try {
        const result = await api.post('/api/auth/login', data);
        api.setToken(result.token);
        currentUser = result.user;
        closeModal('loginModal');
        CampusUtils.showToast('登录成功！', 'success');
        updateNavRight();
        form.reset();
    } catch (e) {
        CampusUtils.showToast(e.message || '登录失败', 'error');
    }
    return false;
}

async function handleRegister(event) {
    event.preventDefault();
    const form = event.target;
    const data = {
        username: form.username.value.trim(),
        email: form.email.value.trim(),
        password: form.password.value
    };
    if (!data.username || !data.email || !data.password) {
        CampusUtils.showToast('请填写完整信息', 'error');
        return false;
    }
    try {
        await api.post('/api/auth/register', data);
        CampusUtils.showToast('注册成功！请登录', 'success');
        switchModal('registerModal', 'loginModal');
        form.reset();
    } catch (e) {
        CampusUtils.showToast(e.message || '注册失败', 'error');
    }
    return false;
}

// openForgotPassword / handleForgotPassword 的唯一定义在 app.js
// （支持开发模式直接展示重置链接）；此处重复实现靠加载顺序被覆盖，已删除。

async function handleLogout() {
    api.setToken(null);
    currentUser = null;
    updateNavRight();
    CampusUtils.showToast('已退出登录', 'info');
    if (window.location.pathname !== '/') {
        window.location.href = '/';
    }
}

function updateNavRight() {
    const navRight = document.getElementById('navRight');
    if (!navRight) return;

    const themeToggleHTML = navRight.querySelector('#themeToggle')?.outerHTML || '';

    if (currentUser) {
        const coins = currentUser.coins || 0;
        const points = currentUser.points || 0;
        navRight.innerHTML = `
            ${themeToggleHTML}
            <div class="user-assets">
                <span class="asset-item" title="金币"><i data-lucide="coins" class="icon icon-sm"></i> ${CampusUtils.formatNumber(coins)}</span>
                <span class="asset-item" title="积分"><i data-lucide="star" class="icon icon-sm"></i> ${CampusUtils.formatNumber(points)}</span>
            </div>
            <a href="/create" class="btn btn-primary btn-sm" style="text-decoration:none;">
                <i data-lucide="pencil" class="icon icon-sm"></i> 发帖
            </a>
            <a href="/notifications" class="icon-btn" style="position:relative;text-decoration:none;" id="notifLink">
                <i data-lucide="bell" class="icon icon-md"></i>
                <span class="badge" id="notifBadge" style="display:none;"></span>
            </a>
            <div class="user-chip" onclick="toggleUserMenu()">
                <img src="${currentUser.avatar || '/static/images/default-avatar.svg'}" onerror="this.src='/static/images/default-avatar.svg'" style="width:28px;height:28px;border-radius:50%;">
                <span style="font-size:0.85rem;font-weight:500;">${CampusUtils.escHtml(currentUser.username)}</span>
            </div>
            <div id="userMenu" style="display:none;position:absolute;top:100%;right:0;margin-top:8px;background:var(--bg-primary);border:1px solid var(--border-primary);border-radius:var(--radius-lg);padding:8px;box-shadow:var(--shadow-lg);min-width:180px;z-index:var(--z-dropdown,200);">
                <a href="/profile/${currentUser.id}" style="display:flex;align-items:center;gap:8px;padding:8px 12px;border-radius:var(--radius-md);font-size:0.85rem;color:var(--text-primary);text-decoration:none;">
                    <i data-lucide="user" class="icon icon-sm"></i> 个人中心
                </a>
                <a href="/notifications" style="display:flex;align-items:center;gap:8px;padding:8px 12px;border-radius:var(--radius-md);font-size:0.85rem;color:var(--text-primary);text-decoration:none;">
                    <i data-lucide="bell" class="icon icon-sm"></i> 通知
                </a>
                <a href="/messages" style="display:flex;align-items:center;gap:8px;padding:8px 12px;border-radius:var(--radius-md);font-size:0.85rem;color:var(--text-primary);text-decoration:none;">
                    <i data-lucide="mail" class="icon icon-sm"></i> 私信
                </a>
                <a href="/favorites" style="display:flex;align-items:center;gap:8px;padding:8px 12px;border-radius:var(--radius-md);font-size:0.85rem;color:var(--text-primary);text-decoration:none;">
                    <i data-lucide="bookmark" class="icon icon-sm"></i> 我的收藏
                </a>
                <a href="/create-station" style="display:flex;align-items:center;gap:8px;padding:8px 12px;border-radius:var(--radius-md);font-size:0.85rem;color:var(--text-primary);text-decoration:none;">
                    <i data-lucide="building-2" class="icon icon-sm"></i> 创建子站
                </a>
                <hr style="border:none;border-top:1px solid var(--border-primary);margin:4px 0;">
                <a href="#" onclick="openEditProfile();return false;" style="display:flex;align-items:center;gap:8px;padding:8px 12px;border-radius:var(--radius-md);font-size:0.85rem;color:var(--text-primary);text-decoration:none;">
                    <i data-lucide="pencil" class="icon icon-sm"></i> 编辑资料
                </a>
                <a href="#" onclick="openChangePassword();return false;" style="display:flex;align-items:center;gap:8px;padding:8px 12px;border-radius:var(--radius-md);font-size:0.85rem;color:var(--text-primary);text-decoration:none;">
                    <i data-lucide="key" class="icon icon-sm"></i> 修改密码
                </a>
                <hr style="border:none;border-top:1px solid var(--border-primary);margin:4px 0;">
                <a href="#" onclick="handleLogout();return false;" style="display:flex;align-items:center;gap:8px;padding:8px 12px;border-radius:var(--radius-md);font-size:0.85rem;color:var(--danger);text-decoration:none;">
                    <i data-lucide="log-out" class="icon icon-sm"></i> 退出登录
                </a>
            </div>
        `;
    } else {
        navRight.innerHTML = `
            ${themeToggleHTML}
            <button class="btn btn-sm" onclick="openModal('loginModal')">登录</button>
            <button class="btn btn-primary btn-sm" onclick="openModal('registerModal')">注册</button>
        `;
    }

    if (typeof lucide !== 'undefined') lucide.createIcons();
}

function toggleUserMenu() {
    const menu = document.getElementById('userMenu');
    if (menu) menu.style.display = menu.style.display === 'none' ? '' : 'none';
}

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

window.CampusAuth = {
    currentUser: () => currentUser,
    onUserReady,
    loadCurrentUser,
    requireAuth,
    handleLogin,
    handleRegister,
    handleForgotPassword,
    handleLogout,
    openForgotPassword,
    updateNavRight,
    toggleUserMenu,
    updateNotifBadge
};

// 兼容全局变量
window.currentUser = currentUser;
window.onUserReady = onUserReady;
window.requireAuth = requireAuth;
window.handleLogin = handleLogin;
window.handleRegister = handleRegister;
window.handleForgotPassword = handleForgotPassword;
window.handleLogout = handleLogout;
window.openForgotPassword = openForgotPassword;
window.updateNavRight = updateNavRight;
window.toggleUserMenu = toggleUserMenu;
window.updateNotifBadge = updateNotifBadge;
