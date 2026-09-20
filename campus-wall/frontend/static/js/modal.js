/**
 * CampusWall 模态框管理模块
 *
 * 注意：openModal / closeModal / switchModal 的唯一定义在 app.js（使用 .show 类，
 * 与 style.css 的 .modal-overlay.show 一致，且带页面初始化钩子）。
 * 本文件此前另有一份用 .active 的实现，永远是死代码；一旦脚本加载顺序变化就会
 * 让全站弹窗静默失效（.active 无任何样式），故删除以消除隐患。
 */

function openEditProfile() {
    const user = window.CampusAuth?.currentUser();
    if (!user) return;
    const form = document.getElementById('editProfileForm');
    if (form) {
        form.username.value = user.username || '';
        form.bio.value = user.bio || '';
        if (form.mood) form.mood.value = user.mood || '';
        if (form.title) form.title.value = user.title || '';
    }
    const preview = document.getElementById('editAvatarPreview');
    if (preview && user.avatar) preview.src = user.avatar;
    openModal('editProfileModal');
}

function previewAvatar(input) {
    if (!input.files || !input.files[0]) return;
    const reader = new FileReader();
    reader.onload = function(e) {
        const preview = document.getElementById('editAvatarPreview');
        if (preview) preview.src = e.target.result;
    };
    reader.readAsDataURL(input.files[0]);
}

function openChangePassword() {
    openModal('changePasswordModal');
}

async function handleEditProfile(event) {
    event.preventDefault();
    const form = event.target;
    const data = {
        username: form.username.value.trim(),
        bio: form.bio.value.trim(),
        mood: form.mood ? form.mood.value.trim() : '',
        title: form.title ? form.title.value.trim() : ''
    };
    if (!data.username) {
        CampusUtils.showToast('用户名不能为空', 'error');
        return false;
    }
    try {
        // 若选择了头像文件，先上传获取 URL
        const avatarInput = form.avatar;
        if (avatarInput && avatarInput.files && avatarInput.files[0]) {
            const fd = new FormData();
            fd.append('file', avatarInput.files[0]);
            const avRes = await fetch('/api/auth/avatar', {
                method: 'POST',
                headers: { 'Authorization': 'Bearer ' + (api.getToken() || '') },
                body: fd
            });
            const avData = await avRes.json();
            if (avData.url) data.avatar = avData.url;
        }
        await api.put('/api/auth/me', data);
        CampusUtils.showToast('资料更新成功', 'success');
        await window.CampusAuth.loadCurrentUser();
        window.CampusAuth.updateNavRight();
        closeModal('editProfileModal');
    } catch (e) {
        CampusUtils.showToast(e.message || '更新失败', 'error');
    }
    return false;
}

async function handleChangePassword(event) {
    event.preventDefault();
    const form = event.target;
    const data = {
        old_password: form.old_password.value,
        new_password: form.new_password.value,
        confirm_password: form.confirm_password.value
    };
    if (data.new_password !== data.confirm_password) {
        CampusUtils.showToast('两次密码不一致', 'error');
        return false;
    }
    if (data.new_password.length < 6) {
        CampusUtils.showToast('密码至少6个字符', 'error');
        return false;
    }
    try {
        await api.post('/api/auth/change-password', data);
        CampusUtils.showToast('密码修改成功', 'success');
        closeModal('changePasswordModal');
        form.reset();
    } catch (e) {
        CampusUtils.showToast(e.message || '修改失败', 'error');
    }
    return false;
}

// ESC 关闭模态框
document.addEventListener('keydown', function(e) {
    if (e.key === 'Escape') {
        document.querySelectorAll('.modal-overlay.active').forEach(m => {
            m.classList.remove('active');
        });
        document.body.style.overflow = '';
    }
});

// 举报弹窗
let reportTargetData = null;
let reportEvidence = [];   // 证据图 url（R8 举报增强）

function resetReportEvidence() {
    reportEvidence = [];
    const box = document.getElementById('reportEvidencePreview');
    if (box) box.innerHTML = '';
}

function openReportModal(targetType, targetId) {
    if (!window.CampusAuth?.currentUser()) {
        CampusUtils.showToast('请先登录', 'error');
        return;
    }
    reportTargetData = { target_type: targetType, target_id: targetId };
    resetReportEvidence();
    const hint = document.getElementById('reportHint');
    if (hint) hint.textContent = '举报内容 ID：' + targetType + ' #' + targetId;
    const reasons = document.querySelectorAll('#reportReasons .tag');
    if (reasons.length) reasons[0].classList.add('active');
    openModal('reportModal');
}

async function uploadReportEvidence(input) {
    const files = Array.from(input.files || []);
    input.value = '';
    for (const file of files) {
        if (reportEvidence.length >= 4) { CampusUtils.showToast('证据图最多 4 张', 'error'); break; }
        if (file.size > 5 * 1024 * 1024) { CampusUtils.showToast(file.name + ' 超过 5MB', 'error'); continue; }
        const fd = new FormData();
        fd.append('file', file);
        try {
            const res = await fetch('/api/posts/upload-image', {
                method: 'POST',
                headers: window.api.getToken() ? { 'Authorization': 'Bearer ' + window.api.getToken() } : {},
                body: fd
            });
            const data = await res.json().catch(() => ({}));
            if (!res.ok) throw new Error(data.error || '上传失败');
            if (data.url) { reportEvidence.push(data.url); renderReportEvidence(); }
        } catch (e) {
            CampusUtils.showToast(e.message || '证据图上传失败', 'error');
        }
    }
}

function removeReportEvidence(i) { reportEvidence.splice(i, 1); renderReportEvidence(); }

function renderReportEvidence() {
    const box = document.getElementById('reportEvidencePreview');
    if (!box) return;
    box.innerHTML = reportEvidence.map((u, i) => `
        <div class="image-preview-item">
            <img src="${CampusUtils.safeUrl(u, '')}" alt="">
            <button type="button" class="image-preview-remove" onclick="removeReportEvidence(${i})" aria-label="移除">&times;</button>
        </div>`).join('');
}

function selectReportReason(el) {
    document.querySelectorAll('#reportReasons .tag').forEach(t => t.classList.remove('active'));
    el.classList.add('active');
}

async function handleReport(event) {
    event.preventDefault();
    const active = document.querySelector('#reportReasons .tag.active');
    const reason = active ? active.getAttribute('data-reason') : '';
    if (!reason) {
        CampusUtils.showToast('请选择举报原因', 'error');
        return false;
    }
    if (!reportTargetData) return false;
    const form = event.target;
    const detail = form.detail ? form.detail.value.trim() : '';
    try {
        await api.post('/api/reports', {
            target_type: reportTargetData.target_type,
            target_id: reportTargetData.target_id,
            reason: reason,
            detail: detail,
            evidence: reportEvidence.slice()
        });
        CampusUtils.showToast('举报成功，感谢反馈', 'success');
        closeModal('reportModal');
        form.reset();
        resetReportEvidence();
    } catch (e) {
        CampusUtils.showToast(e.message || '举报失败', 'error');
    }
    return false;
}

// 兼容全局变量
window.openReportModal = openReportModal;
window.selectReportReason = selectReportReason;
window.handleReport = handleReport;
window.previewAvatar = previewAvatar;

// 点击遮罩关闭模态框（.show 与 app.js/style.css 统一；旧版写 .active 永不命中=死代码）
document.addEventListener('click', function(e) {
    if (e.target.classList.contains('modal-overlay') && e.target.classList.contains('show')) {
        e.target.classList.remove('show');
        document.body.style.overflow = '';
    }
});

// 命名空间导出（懒引用，防 ReferenceError，同 auth.js 注释）。
// openModal/closeModal/switchModal 的实现在 app.js（顶层 function 自动全局），
// 此处只转发，不再用裸标识符简写引用。
window.CampusModal = {
    openModal: (id) => window.openModal(id),
    closeModal: (id) => window.closeModal(id),
    switchModal: (from, to) => window.switchModal(from, to),
    openEditProfile,
    openChangePassword,
    handleEditProfile,
    handleChangePassword,
    previewAvatar,
    openReportModal,
    selectReportReason,
    handleReport
};
