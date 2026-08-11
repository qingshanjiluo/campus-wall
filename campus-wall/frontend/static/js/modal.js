/**
 * CampusWall 模态框管理模块
 */

function openModal(id) {
    const modal = document.getElementById(id);
    if (!modal) return;
    modal.classList.add('active');
    document.body.style.overflow = 'hidden';

    const firstInput = modal.querySelector('input:not([type="hidden"]), textarea');
    if (firstInput) setTimeout(() => firstInput.focus(), 100);
}

function closeModal(id) {
    const modal = document.getElementById(id);
    if (!modal) return;
    modal.classList.remove('active');
    document.body.style.overflow = '';
}

function switchModal(fromId, toId) {
    closeModal(fromId);
    setTimeout(() => openModal(toId), 200);
}

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

function openReportModal(targetType, targetId) {
    if (!window.CampusAuth?.currentUser()) {
        CampusUtils.showToast('请先登录', 'error');
        return;
    }
    reportTargetData = { target_type: targetType, target_id: targetId };
    const hint = document.getElementById('reportHint');
    if (hint) hint.textContent = '举报内容 ID：' + targetType + ' #' + targetId;
    const reasons = document.querySelectorAll('#reportReasons .tag');
    if (reasons.length) reasons[0].classList.add('active');
    openModal('reportModal');
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
            detail: detail
        });
        CampusUtils.showToast('举报成功，感谢反馈', 'success');
        closeModal('reportModal');
        form.reset();
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

// 点击遮罩关闭模态框
document.addEventListener('click', function(e) {
    if (e.target.classList.contains('modal-overlay') && e.target.classList.contains('active')) {
        e.target.classList.remove('active');
        document.body.style.overflow = '';
    }
});

window.CampusModal = {
    openModal,
    closeModal,
    switchModal,
    openEditProfile,
    openChangePassword,
    handleEditProfile,
    handleChangePassword
};

// 兼容全局变量
window.openModal = openModal;
window.closeModal = closeModal;
window.switchModal = switchModal;
window.openEditProfile = openEditProfile;
window.openChangePassword = openChangePassword;
window.handleEditProfile = handleEditProfile;
window.handleChangePassword = handleChangePassword;
