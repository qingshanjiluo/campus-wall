-- 017: friend_link — admin-managed 友情链接, replacing the static frontend
-- config apps/web/app/config/friend.json. Friends are grouped into 3 fixed
-- categories (official / galgame / others); drag-reorder in the admin persists
-- via sort_order (ascending within a category). banner is a full image URL
-- (image_service webp for new uploads; the seed keeps the existing
-- /friends/<name>.webp static path).
BEGIN;

CREATE TABLE IF NOT EXISTS friend_link (
  id          SERIAL PRIMARY KEY,
  category    TEXT        NOT NULL,
  name        TEXT        NOT NULL,
  link        TEXT        NOT NULL,
  description TEXT        NOT NULL DEFAULT '',
  banner      TEXT        NOT NULL DEFAULT '',
  status      TEXT        NOT NULL DEFAULT 'normal',
  sort_order  INTEGER     NOT NULL DEFAULT 0,
  created     TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_friend_link_category_order
  ON friend_link (category, sort_order);

-- Seed from friend.json (40 rows). Guarded by NOT EXISTS so a fresh env gets
-- the data while a re-run (or a DB that already has friend links) is a no-op.
INSERT INTO friend_link (category, name, link, description, banner, status, sort_order)
SELECT v.category, v.name, v.link, v.description, v.banner, v.status, v.sort_order
FROM (VALUES
  ('official', '鲲 Galgame 论坛（备用网址）', 'https://www.kungal.org', '世界上最萌的 Galgame 论坛! 现阶段世界上最先进的 Galgame 资源发布网站! 永远免费! Galgame 下载, Galgame 资源网站', '/friends/kunForum.webp', 'normal', 0),
  ('official', '鲲 Galgame 补丁', 'https://www.moyu.moe', '开源, 免费, 零门槛, 纯手写, 最先进的 Galgame 补丁资源下载站, 提供 Windows, 安卓, KRKR, Tyranor 等各类平台的 Galgame 补丁资源下载。永远免费！', '/friends/kunPatch.webp', 'normal', 1),
  ('official', '鲲 Galgame 表情包', 'https://sticker.kungal.com/', 'Galgame 表情包网站, 鲲 Galgame, Galgame 表情包下载, Galgame, 表情包, 下载', '/friends/kunSticker.webp', 'normal', 2),
  ('official', '鲲 Galgame 导航', 'https://nav.kungal.org/', '鲲 Galgame 导航页, 指明鲲 Galgame 网站集群的所有网站', '/friends/kunNav.webp', 'normal', 3),
  ('galgame', 'ACGNGAME', 'https://acgn.games', 'ACGNGAME, Gal World, Galgame 游戏爱好者之家', '/friends/acgngame.webp', 'normal', 0),
  ('galgame', '失落的小站', 'https://shinnku.com', 'Upset Gal, 失落的小站 失落小站 - galgame资源站', '/friends/shinnku.webp', 'normal', 1),
  ('galgame', '月幕 galgame', 'https://www.ymgal.games', 'YM Galgame, 月幕 Galgame -最戳你XP的美少女游戏综合交流平台 | 来感受这绝妙的艺术体裁', '/friends/ymgal.webp', 'normal', 2),
  ('galgame', 'Galgamer', 'https://galgamer.moe', '因爲你是一個一個一個 <Galgamer/美少女> 啊啊啊啊阿', '/friends/galgamer.webp', 'normal', 3),
  ('galgame', 'Hikarinagi', 'https://www.hikarinagi.org/', 'Hikarinagi致力于为所有ACG爱好者提供一个交流和分享的平台! 你不仅可以找到汉化Galgame、小说、漫画等等超多资源, 还能和谐愉快地与大家互动!', '/friends/hikarinagi.webp', 'normal', 4),
  ('galgame', '青桔网', 'https://www.qingju.org/', '青桔网是由青桔移植组建立的非盈利型galgame免费分享平台', '/friends/qingju.webp', 'normal', 5),
  ('galgame', 'TouchGal', 'https://www.touchgal.us/', 'TouchGAL是立足于分享快乐的一站式Galgame文化社区, 为Gal爱好者提供一片净土!', '/friends/touchgal.webp', 'normal', 6),
  ('galgame', '维咔 ACG', 'https://www.vikacg.com/', '维咔VikACG[V站] - 肥宅们的欢乐家园', '/friends/vikacg.webp', 'normal', 7),
  ('galgame', '紫缘社', 'https://galzy.eu.org/', 'Galgame 资源站, 这里收录了大部分电脑端与手机端的汉化 Galgame', '/friends/galzy.webp', 'normal', 8),
  ('galgame', 'Singureo', 'https://www.singureo.com/', '本站自2025年1月25日正式上线以来,始终以简洁美观人性化的理念运营,以用户的体验为主,希望能够给广大用户带来优质的体验', '/friends/singureo.webp', 'normal', 9),
  ('galgame', '梓澪の妙妙屋', 'https://zi0.cc/', '梓澪のGalgame仓库', '/friends/zi0.webp', 'normal', 10),
  ('galgame', '绮梦 ACG', 'https://acgs.one/', '专注分享次元世界 - Galgame, 游戏, 免费, 下载', '/friends/acgs.webp', 'normal', 11),
  ('galgame', '喵源领域', 'https://www.nekotaku.me/', '喵源领域是一个专注于高质量的Galgame文化分享网站, 提供最新最全的Galgame相关文件下载服务, 助力将Galgame游戏文化推向世界!', '/friends/nekotaku.webp', 'normal', 12),
  ('galgame', 'Galgame 月谣', 'https://www.sayafx.space/', 'Galgame, 月谣, 月谣分享, Galgame 月谣, Galgame 下载, 免费 Galgame, Galgame 资讯', '/friends/sayafx.webp', 'normal', 13),
  ('galgame', '量子 ACG', 'https://www.lzacg.top/', '量子ACG是一个以游戏为主, 进而推动日语学习的网站', '/friends/lzacg.webp', 'normal', 14),
  ('galgame', '御爱同萌', 'https://www.ai2.moe/', '禦愛同萌！一個以非營利為目的的交流社區', '/friends/ai2moe.webp', 'normal', 15),
  ('galgame', 'Galgamex', 'https://www.galgamex.net/', 'Galgamex是一个集多种功能于一体的综合性社区。在这里, 你能畅聊二次元话题, 结交志同道合的朋友, 还能获取海量二次元资源, 全部免费下载。', '/friends/galgamex.webp', 'normal', 16),
  ('galgame', 'Nysoure', 'https://res.nyne.dev/', 'Nysoure 是一个galgame分享网站', '/friends/nysoure.webp', 'normal', 17),
  ('galgame', 'LyCorisGal', 'https://www.lycorisgal.com/', 'LyCorisGal 是一个Gal引导资源站。帮助用户学习并获取资源', '/friends/lycorisgal.webp', 'normal', 18),
  ('galgame', 'NekoGal', 'https://www.nekogal.com/', 'NekoGAL- Galgame传递者, 免费·高质量 Galgame 资源站', '/friends/nekogal.webp', 'normal', 19),
  ('galgame', 'KDays', 'https://bbs2.kdays.net/', 'GalGame 动漫综合向论坛', '/friends/kdays.webp', 'normal', 20),
  ('galgame', '0721_Galgame', 'https://nn0721.icu/', 'Galgame,Galgame网站,Galgame下载,Galgame资源,Galgame汉化Galgame制作,Galgame讨论', '/friends/0721_galgame.webp', 'normal', 21),
  ('galgame', '次元宅社', 'https://www.acgsekai.xyz/', '次元宅社, 提供ACGN资源, 是公益性的全免费资源分享平台。打造二次元网站一片净土, 我们的理念是开放、共创, 希望与您共同CHANGE SEKAI!', '/friends/acgsekai.webp', 'normal', 22),
  ('galgame', '05的资源小站', 'https://05fx.022016.xyz/', '05号机 一手维护建设的资源站', '/friends/05.webp', 'normal', 23),
  ('galgame', '玖黎 ACG', 'https://feiyuwan.top/', '一个免费分享 Gal, Cos, Asmr 的网站', '/friends/feiyuwan.webp', 'normal', 24),
  ('galgame', '书音的图书馆', 'https://shionlib.com/zh/', '书音的图书馆 - 免费、开源、零门槛的 视觉小说 / Galgame 档案库', '/friends/shionlib.webp', 'normal', 25),
  ('galgame', '初音游戏库', 'https://mikugame.icu/', 'MikuGame初音游戏库是专业的游戏资源分享平台, 提供海量免费3A大作、Galgame视觉小说游戏、绅士游戏等各类游戏资源下载。', '/friends/mikugame.webp', 'normal', 26),
  ('galgame', '米洛次元', 'https://www.mikiacg.com/', '优质-免费-干净-简洁, 免费,优质精品游戏', '/friends/mikiacg.webp', 'normal', 27),
  ('galgame', 'KisuGal', 'https://kisugal.icu/', '免费的galgame资源下载平台', '/friends/kisugal.webp', 'normal', 28),
  ('others', 'OhMyGPT', 'https://ohmygpt.com', 'OhMyGPT.COM 可以让你便捷地无限量访问GPT-3.5-turbo、GPT-3.5-turbo-16k、GPT-4、GPT-4-32k、DALL-E、whisper、MidJourney等先进的AI模型。', '/friends/ohmygpt.webp', 'normal', 0),
  ('others', 'Koibumi URL Shorten', 'https://s.koid.cc/', 'A simple url shorten app', '/friends/koibumi.webp', 'normal', 1),
  ('others', 'TGNAV', 'https://tgnav.github.io/', 'TGNAV - Telegram频道群组导航。收录Telegram上的优质频道和群组, 打造一个高质量Telegram导航', '/friends/tgnav.webp', 'normal', 2),
  ('others', '梦璃', 'https://moeli-desu.com/', '梦璃, 一个 ACG 综合资源分享平台, 一处为阁下提供分享 COSAV 的净土', '/friends/moeli.webp', 'normal', 3),
  ('others', 'ACG 盒子', 'https://www.acgbox.link/', 'ACG 盒子, ACG 二次元导航网站, 收录 ACG 二次元相关的网站, 打造一个 ACG 二次元专属的网站。', '/friends/acgbox.webp', 'normal', 4),
  ('others', '初音导航', 'https://www.chooiin.com/', '最初的声音，无限的未来。', '/friends/chooiin.webp', 'normal', 5),
  ('others', '萌哩 - 萌萌的二次元美图', 'https://www.moely.link/', '全网精选二次元美图，提供高质量原图下载', '/friends/moely.webp', 'normal', 6)
) AS v(category, name, link, description, banner, status, sort_order)
WHERE NOT EXISTS (SELECT 1 FROM friend_link);

COMMIT;
