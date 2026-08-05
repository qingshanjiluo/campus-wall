/**
 * 动画层 — 涂鸦背景 + 加载动画 + 滚动淡入
 */

// ── 生成手绘涂鸦背景 ──
function generateDoodles() {
    const layer = document.getElementById('doodleLayer');
    if (!layer) return;
    const colors = ['#ffb3c6', '#FFD6A5', '#FDFFB6', '#E2D5F5', '#fb6f92', '#B5EAD7'];

    for (let i = 0; i < 25; i++) {
        const star = document.createElement('div');
        star.className = 'doodle-star';
        star.innerHTML = Math.random() > 0.5 ? '★' : '✦';
        star.style.left = Math.random() * 100 + '%';
        star.style.top = Math.random() * 100 + '%';
        star.style.fontSize = (Math.random() * 16 + 8) + 'px';
        star.style.color = colors[Math.floor(Math.random() * colors.length)];
        star.style.animationDelay = Math.random() * 3 + 's';
        layer.appendChild(star);
    }

    for (let i = 0; i < 6; i++) {
        const cloud = document.createElement('div');
        cloud.className = 'doodle-cloud';
        cloud.style.left = Math.random() * 100 + '%';
        cloud.style.top = Math.random() * 100 + '%';
        cloud.style.width = (Math.random() * 80 + 40) + 'px';
        cloud.style.height = (Math.random() * 40 + 20) + 'px';
        layer.appendChild(cloud);
    }

    for (let i = 0; i < 10; i++) {
        const line = document.createElement('div');
        line.className = 'doodle-line';
        line.style.left = Math.random() * 100 + '%';
        line.style.top = Math.random() * 100 + '%';
        line.style.width = (Math.random() * 120 + 40) + 'px';
        line.style.setProperty('--r', (Math.random() * 360) + 'deg');
        layer.appendChild(line);
    }
}

// ── 加载动画（打字机效果）──
function initLoadingAnimation() {
    const screen = document.getElementById('loadingScreen');
    const logo = document.getElementById('loadingLogo');
    const textEl = document.getElementById('loadingText');
    if (!screen || !logo || !textEl) return Promise.resolve();

    const texts = [
        "正在打开社团活动室的门...",
        "阳光透过百叶窗洒了进来...",
        "欢迎来到校园墙 ✨"
    ];

    return new Promise(resolve => {
        let resolved = false;
        function done() {
            if (resolved) return;
            resolved = true;
            screen.classList.add('hidden');
            resolve();
        }

        setTimeout(() => logo.classList.add('show'), 200);

        let textIdx = 0, charIdx = 0;
        function typeWriter() {
            if (textIdx < texts.length) {
                if (charIdx < texts[textIdx].length) {
                    textEl.innerHTML = texts[textIdx].substring(0, charIdx + 1) + '<span class="loading-cursor"></span>';
                    textEl.style.opacity = '1';
                    charIdx++;
                    setTimeout(typeWriter, 50 + Math.random() * 30);
                } else {
                    setTimeout(() => {
                        textIdx++;
                        charIdx = 0;
                        if (textIdx < texts.length) typeWriter();
                        else setTimeout(done, 500);
                    }, 400);
                }
            }
        }

        setTimeout(typeWriter, 600);

        // 安全超时：如果动画卡住，2秒后强制隐藏
        setTimeout(done, 2500);
    });
}

// ── 导航栏显示 ──
function showNavbar() {
    const nav = document.getElementById('navbar');
    if (nav) nav.classList.add('show');
}

// ── 滚动淡入（复用同一 observer）──
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
    document.querySelectorAll('.reveal:not(.visible)').forEach(el => revealObserver.observe(el));
}

// ── 启动所有动画 ──
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

// ── 标签页不可见时暂停背景动画 ──
document.addEventListener('visibilitychange', () => {
    const paused = document.hidden ? 'paused' : 'running';
    const layer = document.getElementById('doodleLayer');
    if (layer) {
        layer.style.animationPlayState = paused;
        layer.querySelectorAll('*').forEach(el => {
            el.style.animationPlayState = paused;
        });
    }
    document.querySelectorAll('.bg-glow').forEach(el => {
        el.style.animationPlayState = paused;
    });
});
