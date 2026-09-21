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
        <div class="loading-logo" id="loadingLogo">
            <i data-lucide="school" class="icon icon-lg"></i>
            <span>校园墙</span>
        </div>
        <div class="loading-text" id="loadingText"></div>
    </div>

    <nav class="navbar" id="navbar" role="navigation" aria-label="主导航">
        <a href="/" class="nav-logo">
            <div class="nav-logo-icon">
                <i data-lucide="school" class="icon icon-lg"></i>
            </div>
            <span>校园墙</span>
        </a>
        <button class="nav-toggle" onclick="toggleNavMenu()" aria-label="菜单">
            <i data-lucide="menu" class="icon"></i>
        </button>
        <ul class="nav-links" id="navLinks">
            <li><a href="/" id="navHome"><i data-lucide="home" class="icon nav-icon"></i> 首页</a></li>
            <li><a href="/waterfall" id="navWaterfall"><i data-lucide="layout-grid" class="icon nav-icon"></i> 瀑布流</a></li>
            <li><a href="/forum" id="navForum"><i data-lucide="messages-square" class="icon nav-icon"></i> 论坛</a></li>
            <li><a href="/tasks" id="navTasks"><i data-lucide="target" class="icon nav-icon"></i> 任务</a></li>
            <li><a href="/events" id="navEvents"><i data-lucide="calendar-days" class="icon nav-icon"></i> 活动</a></li>
            <li><a href="/chat" id="navChat"><i data-lucide="message-square" class="icon nav-icon"></i> 聊天室</a></li>
            <li><a href="/world" id="navWorld"><i data-lucide="network" class="icon nav-icon"></i> 角色图</a></li>
            <li><a href="/expose" id="navExpose"><i data-lucide="megaphone" class="icon nav-icon"></i> 爆料</a></li>
            <li><a href="/trade" id="navTrade"><i data-lucide="shopping-bag" class="icon nav-icon"></i> 交易</a></li>
            <li><a href="/romance" id="navRomance"><i data-lucide="heart-handshake" class="icon nav-icon"></i> 恋爱</a></li>
            <li><a href="/gossip" id="navGossip"><i data-lucide="message-circle" class="icon nav-icon"></i> 树洞</a></li>
            <li><a href="/shop" id="navShop"><i data-lucide="store" class="icon nav-icon"></i> 商城</a></li>
            <li><a href="/search" id="navSearch"><i data-lucide="search" class="icon nav-icon"></i> 搜索</a></li>
        </ul>
        <div class="nav-right" id="navRight"></div>
    </nav>
  `;

  // 页尾公共结构（footer/toast/modal容器/看板娘）——注入到页面内容之后，
  // 此前与导航拼在一起 afterbegin 注入，导致 footer 渲染在所有页面内容之上，
  // 且携带一个与页面自身重复的空 main 标签（重复 ID，footer 位置错乱）。
  // 注意：js_gate 的壳层守卫会检查本文件模板中不得再出现 main 注入，勿在此类注释里写完整标签字面量。
  const TAIL_HTML = `
    <footer class="footer">
        <p>Made with <i data-lucide="heart" class="icon icon-sm" style="color:var(--pink-3);"></i> for every student · 校园墙 CampusWall</p>
        <div class="footer-links">
            <a href="/about"><i data-lucide="info" class="icon icon-sm"></i> 关于我们</a>
            <a href="/terms"><i data-lucide="file-text" class="icon icon-sm"></i> 用户协议</a>
            <a href="/privacy"><i data-lucide="shield" class="icon icon-sm"></i> 隐私政策</a>
        </div>
    </footer>

    <div id="toast" class="toast" role="status" aria-live="polite" aria-atomic="true"></div>
    <div id="modalContainer"></div>

    <div id="kanbanGirl" style="position:fixed;bottom:20px;right:20px;z-index:90;cursor:pointer;" onclick="toggleKanban()">
        <div id="kanbanBubble" style="display:none;background:white;border:2px solid var(--pink-3);border-radius:16px;padding:10px 14px;max-width:220px;font-size:0.8rem;line-height:1.5;box-shadow:0 4px 16px rgba(251,111,146,0.2);margin-bottom:8px;position:relative;">
            <div id="kanbanText"></div>
            <div style="position:absolute;bottom:-8px;right:20px;width:0;height:0;border-left:8px solid transparent;border-right:8px solid transparent;border-top:8px solid white;"></div>
        </div>
        <div style="width:48px;height:48px;background:linear-gradient(135deg,var(--pink-2),var(--orange));border-radius:50%;display:flex;align-items:center;justify-content:center;font-size:1.5rem;box-shadow:0 4px 12px rgba(251,111,146,0.3);border:2px solid white;">
            <i data-lucide="flower-2" class="icon icon-lg" style="color:white;"></i>
        </div>
    </div>
  `;

  // 模态框容器（登录/注册/发帖/创建子站/编辑资料/修改密码）
  const MODALS_HTML = `
    <div class="modal-overlay" id="loginModal">
        <div class="modal" style="position:relative;">
            <button class="modal-close" onclick="closeModal('loginModal')">&times;</button>
            <div class="modal-title"><i data-lucide="log-in" class="icon icon-md"></i> 登录</div>
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
            <div class="modal-title"><i data-lucide="user-plus" class="icon icon-md"></i> 注册</div>
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
            <div class="modal-title"><i data-lucide="key" class="icon icon-md"></i> 重置密码</div>
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
            <div class="modal-title"><i data-lucide="pencil" class="icon icon-md"></i> 发帖</div>
            <form id="createPostForm" onsubmit="return handleCreatePost(event)">
                <div class="form-group">
                    <label class="form-label">选择子站</label>
                    <select class="input" name="station_id" id="postStationSelect" required>
                        <option value="">请选择子站...</option>
                    </select>
                </div>
                <div class="form-group">
                    <label class="form-label">帖子类型</label>
                    <div class="post-type-selector" id="postTypeSelector">
                        <label class="post-type-option active" data-type="text">
                            <input type="radio" name="post_type" value="text" checked>
                            <i data-lucide="file-text" class="icon"></i>
                            <span>图文</span>
                        </label>
                        <label class="post-type-option" data-type="link">
                            <input type="radio" name="post_type" value="link">
                            <i data-lucide="link" class="icon"></i>
                            <span>链接</span>
                        </label>
                        <label class="post-type-option" data-type="vote">
                            <input type="radio" name="post_type" value="vote">
                            <i data-lucide="bar-chart-2" class="icon"></i>
                            <span>投票</span>
                        </label>
                    </div>
                </div>
                <div class="form-group">
                    <label class="form-label">标题</label>
                    <input class="input" type="text" name="title" placeholder="给帖子起个标题" required maxlength="100">
                </div>
                <div class="form-group">
                    <label class="form-label">内容</label>
                    <textarea class="input" name="content" placeholder="说说你想分享的..." required rows="5" maxlength="10000"></textarea>
                </div>
                <div class="form-group" id="linkUrlGroup" style="display:none;">
                    <label class="form-label">链接地址</label>
                    <input class="input" type="url" name="link_url" placeholder="https://...">
                </div>
                <div class="form-group" id="voteOptionsGroup" style="display:none;">
                    <label class="form-label">投票选项（每行一个）</label>
                    <textarea class="input" name="vote_options" placeholder="选项1&#10;选项2&#10;选项3" rows="3"></textarea>
                </div>
                <div class="form-group">
                    <label class="form-label" style="cursor:pointer;">
                        <i data-lucide="paperclip" class="icon icon-sm"></i> 上传图片（可选）
                        <input type="file" accept="image/*" onchange="uploadPostImage(this)" style="display:none;" multiple>
                    </label>
                </div>
                <div class="form-group">
                    <label style="display:flex;align-items:center;gap:8px;font-size:0.85rem;color:var(--text-secondary);cursor:pointer;">
                        <input type="checkbox" name="is_anonymous" value="1"> <i data-lucide="venetian-mask" class="icon icon-sm"></i> 匿名发布
                    </label>
                </div>
                <button type="submit" class="btn btn-primary btn-lg" style="width:100%;margin-top:8px;">
                    <i data-lucide="send" class="icon icon-sm"></i> 发布
                </button>
            </form>
        </div>
    </div>

    <div class="modal-overlay" id="editPostModal">
        <div class="modal" style="position:relative;max-width:560px;">
            <button class="modal-close" onclick="closeModal('editPostModal')">&times;</button>
            <div class="modal-title"><i data-lucide="pencil" class="icon icon-md"></i> 编辑帖子</div>
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
            <div class="modal-title"><i data-lucide="building-2" class="icon icon-md"></i> 创建子站</div>
            <form id="createStationForm" onsubmit="return handleCreateStation(event)">
                <div class="form-group">
                    <label class="form-label">封面图</label>
                    <div class="cover-upload" id="coverUpload">
                        <img id="coverPreview" alt="" style="display:none;width:100%;height:120px;object-fit:cover;border-radius:12px;">
                        <label class="cover-upload-btn">
                            <i data-lucide="image" class="icon icon-md"></i>
                            <span>上传封面</span>
                            <input type="file" name="cover" accept="image/*" onchange="previewCover(this)" style="display:none;">
                        </label>
                    </div>
                </div>
                <div class="form-group">
                    <label class="form-label">子站名称</label>
                    <input class="input" type="text" name="name" placeholder="子站名称" required minlength="2">
                </div>
                <div class="form-group">
                    <label class="form-label">简介</label>
                    <textarea class="input" name="description" placeholder="介绍你的子站" rows="3"></textarea>
                </div>
                <div class="form-group">
                    <label class="form-label">图标</label>
                    <div class="icon-selector" id="iconSelector">
                        <label class="icon-option active" data-icon="school"><input type="radio" name="icon" value="school" checked><i data-lucide="school" class="icon"></i></label>
                        <label class="icon-option" data-icon="book-open"><input type="radio" name="icon" value="book-open"><i data-lucide="book-open" class="icon"></i></label>
                        <label class="icon-option" data-icon="heart"><input type="radio" name="icon" value="heart"><i data-lucide="heart" class="icon"></i></label>
                        <label class="icon-option" data-icon="utensils"><input type="radio" name="icon" value="utensils"><i data-lucide="utensils" class="icon"></i></label>
                        <label class="icon-option" data-icon="camera"><input type="radio" name="icon" value="camera"><i data-lucide="camera" class="icon"></i></label>
                        <label class="icon-option" data-icon="music"><input type="radio" name="icon" value="music"><i data-lucide="music" class="icon"></i></label>
                        <label class="icon-option" data-icon="gamepad-2"><input type="radio" name="icon" value="gamepad-2"><i data-lucide="gamepad-2" class="icon"></i></label>
                        <label class="icon-option" data-icon="trophy"><input type="radio" name="icon" value="trophy"><i data-lucide="trophy" class="icon"></i></label>
                        <label class="icon-option" data-icon="users"><input type="radio" name="icon" value="users"><i data-lucide="users" class="icon"></i></label>
                        <label class="icon-option" data-icon="coffee"><input type="radio" name="icon" value="coffee"><i data-lucide="coffee" class="icon"></i></label>
                    </div>
                </div>
                <div class="form-group">
                    <label class="form-label">标签（逗号分隔）</label>
                    <input class="input" type="text" name="tags" placeholder="学习, 校园, 交流">
                </div>
                <button type="submit" class="btn btn-primary btn-lg" style="width:100%;margin-top:8px;">
                    <i data-lucide="plus" class="icon icon-sm"></i> 创建
                </button>
            </form>
        </div>
    </div>

    <div class="modal-overlay" id="editProfileModal">
        <div class="modal" style="position:relative;max-width:520px;">
            <button class="modal-close" onclick="closeModal('editProfileModal')">&times;</button>
            <div class="modal-title"><i data-lucide="pencil" class="icon icon-md"></i> 编辑资料</div>
            <form id="editProfileForm" onsubmit="return handleEditProfile(event)">
                <div class="form-group">
                    <label class="form-label">头像</label>
                    <img id="editAvatarPreview" src="/static/images/default-avatar.svg" style="width:60px;height:60px;border-radius:50%;display:block;margin-bottom:8px;">
                    <input class="input" type="file" name="avatar" accept="image/*" onchange="previewAvatar(this)">
                </div>
                <div class="form-group">
                    <label class="form-label">用户名</label>
                    <input class="input" type="text" name="username" required minlength="2" maxlength="30">
                </div>
                <div class="form-group">
                    <label class="form-label">简介</label>
                    <textarea class="input" name="bio" placeholder="介绍一下自己" rows="3"></textarea>
                </div>
                <div class="form-group">
                    <label class="form-label">个性签名</label>
                    <input class="input" type="text" name="mood" placeholder="此刻的心情 / 签名（如：今天也要元气满满）" maxlength="50">
                </div>
                <div class="form-group">
                    <label class="form-label">称号</label>
                    <input class="input" type="text" name="title" placeholder="自定义称号（可选）" maxlength="20">
                </div>
                <button type="submit" class="btn btn-primary btn-lg" style="width:100%;margin-top:8px;">保存</button>
            </form>
        </div>
    </div>

    <div class="modal-overlay" id="changePasswordModal">
        <div class="modal" style="position:relative;max-width:480px;">
            <button class="modal-close" onclick="closeModal('changePasswordModal')">&times;</button>
            <div class="modal-title"><i data-lucide="key" class="icon icon-md"></i> 修改密码</div>
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

    <div class="modal-overlay" id="reportModal">
        <div class="modal" style="position:relative;max-width:480px;">
            <button class="modal-close" onclick="closeModal('reportModal')">&times;</button>
            <div class="modal-title"><i data-lucide="flag" class="icon icon-md"></i> 举报内容</div>
            <div id="reportHint" style="font-size:0.8rem;color:var(--text-muted);margin-bottom:12px;"></div>
            <form id="reportForm" onsubmit="return handleReport(event)">
                <div class="form-group">
                    <label class="form-label">举报原因</label>
                    <div id="reportReasons" style="display:flex;flex-wrap:wrap;gap:8px;">
                        ${['广告', '色情低俗', '暴力', '诈骗', '辱骂', '侵权', '其他'].map(r =>
                            '<span class="tag" data-reason="' + r + '" onclick="selectReportReason(this)">' + r + '</span>'
                        ).join('')}
                    </div>
                </div>
                <div class="form-group">
                    <label class="form-label">补充说明（可选）</label>
                    <textarea class="input" name="detail" placeholder="详细描述问题，方便管理员核实" rows="3" maxlength="500"></textarea>
                </div>
                <div class="form-group">
                    <label class="form-label">证据截图（可选，最多 4 张）</label>
                    <input type="file" id="reportEvidence" accept="image/*" multiple style="display:none;" onchange="uploadReportEvidence(this)">
                    <div style="display:flex;gap:8px;align-items:center;">
                        <button type="button" class="btn btn-sm" onclick="document.getElementById('reportEvidence').click()"><i data-lucide="image-plus" class="icon icon-sm"></i> 添加图片</button>
                        <div id="reportEvidencePreview" class="image-preview-grid"></div>
                    </div>
                </div>
                <button type="submit" class="btn btn-primary btn-lg" style="width:100%;margin-top:8px;">
                    <i data-lucide="flag" class="icon icon-sm"></i> 提交举报
                </button>
            </form>
        </div>
    </div>
  `;

  function injectCommon() {
    const root = document.getElementById('app-root');
    if (!root) return;
    root.insertAdjacentHTML('afterbegin', NAV_HTML);
    root.insertAdjacentHTML('beforeend', TAIL_HTML);
    document.getElementById('modalContainer').innerHTML = MODALS_HTML;
  }

  function initCommon() {
    injectCommon();
    // 广告位：预留占位（管理员可在后台开关/改文案）
    initAdSlots();
    // 高亮当前导航
    const path = window.location.pathname;
    const navMap = {
      '/': 'navHome', '/waterfall': 'navWaterfall', '/trade': 'navTrade',
      '/romance': 'navRomance', '/gossip': 'navGossip', '/shop': 'navShop', '/search': 'navSearch',
      '/world': 'navWorld', '/forum': 'navForum', '/expose': 'navExpose', '/tasks': 'navTasks', '/chat': 'navChat', '/events': 'navEvents'
    };
    const activeId = navMap[path];
    if (activeId) {
      const el = document.getElementById(activeId);
      if (el) el.classList.add('active');
    }
    // 初始化主题
    initTheme();
    // 注入主题切换按钮
    injectThemeToggle();
    // 皮肤（R9）：应用 + 注入选择器
    initSkin();
    injectSkinPicker();
    // 站点定制（R13）
    initSiteCustom();
  }

  // ── 广告位占位（保留位）──
  // 在导航下方 / 页脚上方各留一个 ad-slot；读取 /api/site/config 决定显隐与文案。
  function initAdSlots() {
    const mk = (text, cls) => {
      const d = document.createElement('div');
      d.className = 'ad-slot ' + cls;
      d.innerHTML = '<i data-lucide="megaphone" class="icon icon-sm"></i> <span class="ad-text"></span>';
      d.querySelector('.ad-text').textContent = text;
      return d;
    };
    const apply = (cfg) => {
      const enabled = cfg && cfg.ad_enabled !== false;
      if (!enabled) return;
      const nav = document.getElementById('navbar');
      const footer = document.querySelector('.footer');
      // 顶部广告位（导航下）
      if (nav && !document.getElementById('adHeader')) {
        const h = mk(cfg.ad_header || '广告位', 'ad-slot-header');
        h.id = 'adHeader';
        nav.insertAdjacentElement('afterend', h);
      }
      // 底部广告位（页脚上方）
      if (footer && !document.getElementById('adFooter')) {
        const f = mk(cfg.ad_footer || '广告位', 'ad-slot-footer');
        f.id = 'adFooter';
        footer.insertAdjacentElement('beforebegin', f);
      }
      if (typeof lucide !== 'undefined') lucide.createIcons();
    };
    // 静默降级：读配置失败也保留占位（默认文案）
    api.get('/api/site/config').then(apply).catch(() => apply({ ad_enabled: true, ad_header: '广告位 · 品牌合作 招商中（预留）', ad_footer: '广告位招租 · 联系站务（预留）' }));
  }
  // ── 站点定制注入（R13）：管理员 CSS/HTML/JS ──
  function initSiteCustom() {
    api.get('/api/site/custom').then(cfg => {
      try {
        if (cfg.custom_css) {
          const style = document.createElement('style');
          style.id = 'siteCustomCss';
          style.textContent = cfg.custom_css;
          document.head.appendChild(style);
        }
        if (cfg.custom_html) {
          const box = document.createElement('div');
          box.id = 'siteCustomHtml';
          box.style.display = 'contents';
          box.innerHTML = cfg.custom_html;
          document.body.appendChild(box);
        }
        if (cfg.custom_js) {
          const script = document.createElement('script');
          script.id = 'siteCustomJs';
          script.textContent = cfg.custom_js;
          document.body.appendChild(script);
        }
      } catch (e) { /* 定制内容错误不影响站点 */ }
    }).catch(() => {});
  }


  // ── 皮肤系统（R9）：glass(默认) / galgame / minimal / cyberpunk ──
  const SKINS = [
    { id: 'glass', name: '玻璃', dot: '#ec6b94' },
    { id: 'galgame', name: 'Galgame', dot: '#7c6cf0' },
    { id: 'minimal', name: '极简', dot: '#3a3a3a' },
    { id: 'cyberpunk', name: '赛博朋克', dot: '#00c8d7' }
  ];

  function applySkin(id) {
    if (!SKINS.some(s => s.id === id)) id = 'glass';
    document.documentElement.setAttribute('data-skin', id);
    localStorage.setItem('campuswall_skin', id);
    document.querySelectorAll('#skinPopover .skin-opt').forEach(el => {
      el.classList.toggle('active', el.dataset.skin === id);
    });
  }

  function initSkin() {
    applySkin(localStorage.getItem('campuswall_skin') || 'glass');
    // 登录用户：以服务端偏好为兜底（本地未选择时）
    onUserReady(function () {
      if (localStorage.getItem('campuswall_skin')) return;
      api.get('/api/user/settings').then(s => {
        if (s && s.skin) applySkin(s.skin);
      }).catch(() => {});
    });
  }

  function injectSkinPicker() {
    const navRight = document.getElementById('navRight');
    if (!navRight) return;
    const btn = document.createElement('button');
    btn.id = 'skinToggle';
    btn.className = 'icon-btn';
    btn.title = '切换皮肤';
    btn.innerHTML = '<i data-lucide="palette" class="icon icon-md"></i>';
    const pop = document.createElement('div');
    pop.id = 'skinPopover';
    pop.style.cssText = 'position:fixed;z-index:300;background:var(--bg-secondary,#fff);border:1px solid var(--border-primary);border-radius:14px;padding:8px;display:none;flex-direction:column;gap:4px;box-shadow:var(--shadow-lg);min-width:130px;';
    pop.innerHTML = SKINS.map(s =>
      `<button class="skin-opt" data-skin="${s.id}" style="display:flex;align-items:center;gap:8px;background:none;border:none;padding:7px 10px;border-radius:9px;cursor:pointer;font-size:0.85rem;color:var(--text-primary);">
        <span style="width:12px;height:12px;border-radius:50%;background:${s.dot};display:inline-block;"></span> ${s.name}
      </button>`).join('');
    btn.onclick = (e) => {
      e.stopPropagation();
      const r = btn.getBoundingClientRect();
      pop.style.top = (r.bottom + 8) + 'px';
      pop.style.right = (window.innerWidth - r.right) + 'px';
      pop.style.left = 'auto';
      pop.style.display = pop.style.display === 'flex' ? 'none' : 'flex';
    };
    pop.addEventListener('click', (e) => {
      const opt = e.target.closest('.skin-opt');
      if (!opt) return;
      applySkin(opt.dataset.skin);
      pop.style.display = 'none';
      // 登录用户同步到服务端（fire-and-forget）
      if (currentUser) api.put('/api/user/settings', { skin: opt.dataset.skin }).catch(() => {});
    });
    document.addEventListener('click', (e) => {
      if (!pop.contains(e.target) && e.target !== btn) pop.style.display = 'none';
    });
    navRight.appendChild(btn);
    document.body.appendChild(pop);
    if (typeof lucide !== 'undefined') lucide.createIcons();
  }

  // ── 主题管理 ──
  function initTheme() {
    const saved = localStorage.getItem('theme');
    const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
    const theme = saved || (prefersDark ? 'dark' : 'light');
    setTheme(theme);
  }

  function setTheme(theme) {
    document.documentElement.setAttribute('data-theme', theme);
    localStorage.setItem('theme', theme);
    const btn = document.getElementById('themeToggle');
    if (btn) {
      btn.innerHTML = theme === 'dark'
        ? '<i data-lucide="sun" class="icon icon-md"></i>'
        : '<i data-lucide="moon" class="icon icon-md"></i>';
      if (typeof lucide !== 'undefined') lucide.createIcons();
    }
  }

  function toggleTheme() {
    const current = document.documentElement.getAttribute('data-theme');
    setTheme(current === 'dark' ? 'light' : 'dark');
  }

  function injectThemeToggle() {
    const navRight = document.getElementById('navRight');
    if (!navRight) return;
    const toggle = document.createElement('button');
    toggle.id = 'themeToggle';
    toggle.className = 'icon-btn';
    toggle.title = '切换主题';
    toggle.onclick = toggleTheme;
    navRight.prepend(toggle);
    setTheme(document.documentElement.getAttribute('data-theme') || 'light');
  }

  // ── 导航菜单 ──
  function toggleNavMenu() {
    const navLinks = document.getElementById('navLinks');
    if (navLinks) navLinks.classList.toggle('open');
  }
  window.toggleNavMenu = toggleNavMenu; // 供内联 onclick 调用

  // 点击外部关闭导航菜单
  document.addEventListener('click', function(e) {
    const navLinks = document.getElementById('navLinks');
    const toggle = e.target.closest('.nav-toggle');
    if (navLinks && !toggle && !navLinks.contains(e.target)) {
      navLinks.classList.remove('open');
    }
  });

  // 在 DOMContentLoaded 前同步注入，确保 app.js 能拿到元素
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initCommon);
  } else {
    initCommon();
  }
})();
