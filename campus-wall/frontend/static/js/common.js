/**
 * CampusWall 静态版 — 公共框架
 * 注入导航栏、背景、加载动画、看板娘、Toast、模态框容器，
 * 并初始化用户状态。每个页面在 <body> 里放置 <div id="app-root"></div>，
 * 本脚本会自动填充公共结构。
 */
(function () {
  const NAV_HTML = `
    <div class="bg-glow bg-glow-1"></div>
    <div class="bg-glow bg-glow-2"></div>
    <div class="bg-glow bg-glow-3"></div>
    <div class="paper-texture"></div>

    <div class="loading-screen" id="loadingScreen">
        <div class="loading-logo" id="loadingLogo">🏫 校园墙</div>
        <div class="loading-text" id="loadingText"></div>
    </div>

    <nav class="navbar" id="navbar">
        <a href="/" class="nav-logo">
            <div class="nav-logo-icon">🏫</div>
            <span>校园墙</span>
        </a>
        <ul class="nav-links">
            <li><a href="/" id="navHome">首页</a></li>
            <li><a href="/waterfall" id="navWaterfall">瀑布流</a></li>
            <li><a href="/trade" id="navTrade">交易</a></li>
            <li><a href="/romance" id="navRomance">💕</a></li>
            <li><a href="/gossip" id="navGossip">树洞</a></li>
            <li><a href="/shop" id="navShop">商城</a></li>
            <li><a href="/search" id="navSearch">🔍</a></li>
        </ul>
        <div class="nav-right" id="navRight"></div>
    </nav>

    <main id="page-main"></main>

    <footer class="footer">
        <p>Made with <span class="footer-heart">❤</span> for every student · 校园墙 CampusWall</p>
        <div class="footer-links">
            <a href="#">关于我们</a>
            <a href="#">用户协议</a>
            <a href="#">隐私政策</a>
        </div>
    </footer>

    <div id="toast" class="toast"></div>
    <div id="modalContainer"></div>

    <div id="kanbanGirl" style="position:fixed;bottom:20px;right:20px;z-index:90;cursor:pointer;" onclick="toggleKanban()">
        <div id="kanbanBubble" style="display:none;background:white;border:2px solid var(--pink-3);border-radius:16px;padding:10px 14px;max-width:220px;font-size:0.8rem;line-height:1.5;box-shadow:0 4px 16px rgba(251,111,146,0.2);margin-bottom:8px;position:relative;">
            <div id="kanbanText"></div>
            <div style="position:absolute;bottom:-8px;right:20px;width:0;height:0;border-left:8px solid transparent;border-right:8px solid transparent;border-top:8px solid white;"></div>
        </div>
        <div style="width:48px;height:48px;background:linear-gradient(135deg,var(--pink-2),var(--orange));border-radius:50%;display:flex;align-items:center;justify-content:center;font-size:1.5rem;box-shadow:0 4px 12px rgba(251,111,146,0.3);border:2px solid white;">🌸</div>
    </div>
  `;

  // 模态框容器（登录/注册/发帖/创建子站/编辑资料/修改密码）
  const MODALS_HTML = `
    <div class="modal-overlay" id="loginModal">
        <div class="modal" style="position:relative;">
            <button class="modal-close" onclick="closeModal('loginModal')">&times;</button>
            <div class="modal-title">✨ 登录</div>
            <form id="loginForm" onsubmit="return handleLogin(event)">
                <div class="form-group">
                    <label class="form-label">用户名 / 邮箱</label>
                    <input class="input" type="text" name="username" placeholder="请输入用户名或邮箱" required>
                </div>
                <div class="form-group">
                    <label class="form-label">密码</label>
                    <input class="input" type="password" name="password" placeholder="请输入密码" required>
                </div>
                <button type="submit" class="btn btn-primary btn-lg" style="width:100%;margin-top:8px;">登录</button>
            </form>
            <p style="text-align:center;margin-top:16px;font-size:0.85rem;color:var(--text-muted);">
                还没有账号？<a href="#" onclick="switchModal('loginModal','registerModal')">注册</a>
                · <a href="#" onclick="openForgotPassword()">忘记密码</a>
            </p>
        </div>
    </div>

    <div class="modal-overlay" id="registerModal">
        <div class="modal" style="position:relative;">
            <button class="modal-close" onclick="closeModal('registerModal')">&times;</button>
            <div class="modal-title">🌸 注册</div>
            <form id="registerForm" onsubmit="return handleRegister(event)">
                <div class="form-group">
                    <label class="form-label">用户名</label>
                    <input class="input" type="text" name="username" placeholder="至少2个字符" required minlength="2">
                </div>
                <div class="form-group">
                    <label class="form-label">邮箱</label>
                    <input class="input" type="email" name="email" placeholder="your@email.com" required>
                </div>
                <div class="form-group">
                    <label class="form-label">密码</label>
                    <input class="input" type="password" name="password" placeholder="至少6个字符" required minlength="6">
                </div>
                <button type="submit" class="btn btn-primary btn-lg" style="width:100%;margin-top:8px;">注册</button>
            </form>
            <p style="text-align:center;margin-top:16px;font-size:0.85rem;color:var(--text-muted);">
                已有账号？<a href="#" onclick="switchModal('registerModal','loginModal')">登录</a>
            </p>
        </div>
    </div>

    <div class="modal-overlay" id="forgotPasswordModal">
        <div class="modal" style="position:relative;">
            <button class="modal-close" onclick="closeModal('forgotPasswordModal')">&times;</button>
            <div class="modal-title">🔑 重置密码</div>
            <form id="forgotPasswordForm" onsubmit="return handleForgotPassword(event)">
                <div class="form-group">
                    <label class="form-label">注册邮箱</label>
                    <input class="input" type="email" name="email" placeholder="your@email.com" required>
                </div>
                <button type="submit" class="btn btn-primary btn-lg" style="width:100%;margin-top:8px;">发送重置链接</button>
            </form>
        </div>
    </div>

    <div class="modal-overlay" id="createPostModal">
        <div class="modal" style="position:relative;max-width:560px;">
            <button class="modal-close" onclick="closeModal('createPostModal')">&times;</button>
            <div class="modal-title">✍️ 发帖</div>
            <form id="createPostForm" onsubmit="return handleCreatePost(event)">
                <div class="form-group">
                    <label class="form-label">选择子站</label>
                    <select class="input" name="station_id" id="postStationSelect" required>
                        <option value="">请选择子站...</option>
                    </select>
                </div>
                <div class="form-group">
                    <label class="form-label">标题</label>
                    <input class="input" type="text" name="title" placeholder="给帖子起个标题" required maxlength="100">
                </div>
                <div class="form-group">
                    <label class="form-label">内容</label>
                    <textarea class="input" name="content" placeholder="说说你想分享的..." required rows="5" maxlength="10000"></textarea>
                </div>
                <div class="form-group">
                    <label class="form-label" style="cursor:pointer;">📎 上传图片（可选）
                        <input type="file" accept="image/*" onchange="uploadPostImage(this)" style="display:none;">
                    </label>
                </div>
                <div class="form-group">
                    <label style="display:flex;align-items:center;gap:8px;font-size:0.85rem;color:var(--text-secondary);cursor:pointer;">
                        <input type="checkbox" name="is_anonymous" value="1"> 🎭 匿名发布
                    </label>
                </div>
                <button type="submit" class="btn btn-primary btn-lg" style="width:100%;margin-top:8px;">发布</button>
            </form>
        </div>
    </div>

    <div class="modal-overlay" id="editPostModal">
        <div class="modal" style="position:relative;max-width:560px;">
            <button class="modal-close" onclick="closeModal('editPostModal')">&times;</button>
            <div class="modal-title">✏️ 编辑帖子</div>
            <form id="editPostForm" onsubmit="return handleEditPost(event)">
                <input type="hidden" name="post_id">
                <div class="form-group">
                    <label class="form-label">标题</label>
                    <input class="input" type="text" name="title" placeholder="标题" required maxlength="100">
                </div>
                <div class="form-group">
                    <label class="form-label">内容</label>
                    <textarea class="input" name="content" placeholder="内容" required rows="6" maxlength="10000"></textarea>
                </div>
                <button type="submit" class="btn btn-primary btn-lg" style="width:100%;margin-top:8px;">保存修改</button>
            </form>
        </div>
    </div>

    <div class="modal-overlay" id="createStationModal">
        <div class="modal" style="position:relative;max-width:520px;">
            <button class="modal-close" onclick="closeModal('createStationModal')">&times;</button>
            <div class="modal-title">🏗️ 创建子站</div>
            <form id="createStationForm" onsubmit="return handleCreateStation(event)">
                <div class="form-group">
                    <label class="form-label">子站名称</label>
                    <input class="input" type="text" name="name" placeholder="子站名称" required minlength="2">
                </div>
                <div class="form-group">
                    <label class="form-label">简介</label>
                    <textarea class="input" name="description" placeholder="介绍你的子站" rows="3"></textarea>
                </div>
                <div class="form-group">
                    <label class="form-label">图标（emoji）</label>
                    <input class="input" type="text" name="icon" placeholder="🏫" value="🏫">
                </div>
                <div class="form-group">
                    <label class="form-label">标签（逗号分隔）</label>
                    <input class="input" type="text" name="tags" placeholder="学习, 校园, 交流">
                </div>
                <button type="submit" class="btn btn-primary btn-lg" style="width:100%;margin-top:8px;">创建</button>
            </form>
        </div>
    </div>

    <div class="modal-overlay" id="editProfileModal">
        <div class="modal" style="position:relative;max-width:520px;">
            <button class="modal-close" onclick="closeModal('editProfileModal')">&times;</button>
            <div class="modal-title">✏️ 编辑资料</div>
            <form id="editProfileForm" onsubmit="return handleEditProfile(event)">
                <div class="form-group">
                    <label class="form-label">头像</label>
                    <img id="editAvatarPreview" src="/static/images/default-avatar.svg" style="width:60px;height:60px;border-radius:50%;display:block;margin-bottom:8px;">
                    <input class="input" type="file" name="avatar" accept="image/*">
                </div>
                <div class="form-group">
                    <label class="form-label">用户名</label>
                    <input class="input" type="text" name="username" required minlength="2" maxlength="30">
                </div>
                <div class="form-group">
                    <label class="form-label">简介</label>
                    <textarea class="input" name="bio" placeholder="介绍一下自己" rows="3"></textarea>
                </div>
                <button type="submit" class="btn btn-primary btn-lg" style="width:100%;margin-top:8px;">保存</button>
            </form>
        </div>
    </div>

    <div class="modal-overlay" id="changePasswordModal">
        <div class="modal" style="position:relative;max-width:480px;">
            <button class="modal-close" onclick="closeModal('changePasswordModal')">&times;</button>
            <div class="modal-title">🔑 修改密码</div>
            <form id="changePasswordForm" onsubmit="return handleChangePassword(event)">
                <div class="form-group">
                    <label class="form-label">原密码</label>
                    <input class="input" type="password" name="old_password" required>
                </div>
                <div class="form-group">
                    <label class="form-label">新密码</label>
                    <input class="input" type="password" name="new_password" required minlength="6">
                </div>
                <div class="form-group">
                    <label class="form-label">确认新密码</label>
                    <input class="input" type="password" name="confirm_password" required minlength="6">
                </div>
                <button type="submit" class="btn btn-primary btn-lg" style="width:100%;margin-top:8px;">确认修改</button>
            </form>
        </div>
    </div>
  `;

  function injectCommon() {
    const root = document.getElementById('app-root');
    if (!root) return;
    root.insertAdjacentHTML('afterbegin', NAV_HTML);
    document.getElementById('modalContainer').innerHTML = MODALS_HTML;
  }

  function initCommon() {
    injectCommon();
    // 高亮当前导航
    const path = window.location.pathname;
    const navMap = {
      '/': 'navHome', '/waterfall': 'navWaterfall', '/trade': 'navTrade',
      '/romance': 'navRomance', '/gossip': 'navGossip', '/shop': 'navShop', '/search': 'navSearch'
    };
    const activeId = navMap[path];
    if (activeId) {
      const el = document.getElementById(activeId);
      if (el) el.classList.add('active');
    }
  }

  // 在 DOMContentLoaded 前同步注入，确保 app.js 能拿到元素
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initCommon);
  } else {
    initCommon();
  }
})();
