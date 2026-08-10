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
    }
    openModal('editProfileModal');
}

function openChangePassword() {
    openModal('changePasswordModal');
}

async function handleEditProfile(event) {
    event.preventDefault();
    const form = event.target;
    const data = {
        username: form.username.value.trim(),
        bio: form.bio.value.trim()
    };
    if (!data.username) {
        CampusUtils.showToast('用户名不能为空', 'error');
        return false;
    }
    try {
        await api.put('/api/auth/profile', data);
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
        await api.put('/api/auth/password', data);
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
