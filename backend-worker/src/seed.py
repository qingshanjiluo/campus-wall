"""种子数据：初始化演示内容（D1 版）。

用法：
  uv run pywrangler python_shell src/seed.py
或直接部署后在 Worker 中调用 init_db + seed（见 init_db 说明）。
"""
import asyncio
import json
import random
from datetime import datetime, timedelta

from context import env
import db
from auth import hash_password
from models import create_user, create_station, create_post

edb = db.execute
qdb = db.query


USERS = [
    {'username': 'xiaohua', 'email': 'xiaohua@test.com', 'password': '123456',
     'bio': '大二中文系 / 喜欢拍照和写随笔', 'avatar': '/static/images/avatars/1.svg'},
    {'username': 'bob', 'email': 'bob@test.com', 'password': '123456',
     'bio': '计算机科学与技术 / 全栈开发中', 'avatar': '/static/images/avatars/2.svg'},
    {'username': 'xiaolv', 'email': 'xiaolv@test.com', 'password': '123456',
     'bio': '美术学院 / 画画是第一生产力', 'avatar': '/static/images/avatars/3.svg'},
    {'username': 'dapeng', 'email': 'dapeng@test.com', 'password': '123456',
     'bio': '体育生 / 每天五公里起步', 'avatar': '/static/images/avatars/4.svg'},
    {'username': 'tangtang', 'email': 'tangtang@test.com', 'password': '123456',
     'bio': '音乐表演 / 吉他+ukulele+钢琴', 'avatar': '/static/images/avatars/5.svg'},
    {'username': 'xingye', 'email': 'xingye@test.com', 'password': '123456',
     'bio': '物理系 / 今晚有流星雨！', 'avatar': '/static/images/avatars/6.svg'},
    {'username': 'lanmeijiang', 'email': 'lanmeijiang@test.com', 'password': '123456',
     'bio': '经济学 / 闲置交易小达人', 'avatar': '/static/images/avatars/7.svg'},
    {'username': 'admin', 'email': 'admin@test.com', 'password': 'admin123',
     'bio': '系统管理员', 'role': 'admin', 'avatar': '/static/images/avatars/8.svg'},
]

STATIONS = [
    {'name': '表白墙', 'icon': 'heart', 'description': '那些说不出口的喜欢，就写在这里吧', 'category': '情感/表白',
     'tags': ['情感', '表白', '匿名']},
    {'name': '失物招领', 'icon': 'search', 'description': '丢了东西？捡到东西？来这里互助', 'category': '校园生活/互助',
     'tags': ['互助', '校园']},
    {'name': '二手交易', 'icon': 'shopping-cart', 'description': '闲置好物，低价出给有缘人', 'category': '校园生活/交易',
     'tags': ['交易', '闲置']},
    {'name': '学习互助', 'icon': 'book-open', 'description': '一起学习，一起进步，资料共享', 'category': '学习/互助',
     'tags': ['学习', '互助', '考研']},
    {'name': '活动招募', 'icon': 'party-popper', 'description': '校园活动、社团招新、比赛组队', 'category': '校园生活/活动',
     'tags': ['活动', '社团', '组队']},
    {'name': '树洞', 'icon': 'tree-pine', 'description': '说说那些不敢对别人说的话', 'category': '情感/倾诉',
     'tags': ['匿名', '倾诉', '情感']},
    {'name': '食堂测评', 'icon': 'utensils', 'description': '干饭人的日常，今天吃什么好呢？', 'category': '生活/美食',
     'tags': ['美食', '测评', '食堂']},
    {'name': '随手拍', 'icon': 'camera', 'description': '校园里的每一帧风景，都值得被记录', 'category': '兴趣/摄影',
     'tags': ['摄影', '风景', '记录']},
    {'name': '自习室打卡', 'icon': 'pen-line', 'description': '每天进步一点点，和小伙伴一起监督学习', 'category': '学习/自律',
     'tags': ['学习', '打卡', '自律']},
    {'name': '深夜树洞', 'icon': 'moon', 'description': '夜晚专属的情绪出口，天亮就好了', 'category': '情感/倾诉',
     'tags': ['匿名', '情感', '夜晚']},
    {'name': '晨跑打卡', 'icon': 'running', 'description': '早起的人先享受世界', 'category': '运动/健身',
     'tags': ['运动', '打卡', '健康']},
    {'name': '电影放映室', 'icon': 'film', 'description': '周末一起看电影，分享触动心灵的瞬间', 'category': '兴趣/影视',
     'tags': ['影视', '讨论', '推荐']},
    {'name': '校园猫咪图鉴', 'icon': 'cat', 'description': '记录校园里每一只毛茸茸的小可爱', 'category': '兴趣/萌宠',
     'tags': ['萌宠', '摄影', '猫咪']},
    {'name': '考研互助站', 'icon': 'pen-line', 'description': '资料共享、经验交流，我们一起上岸', 'category': '学习/考研',
     'tags': ['学习', '考研', '互助']},
    {'name': '吉他社', 'icon': 'music', 'description': '用音乐连接彼此，弹唱我们的青春', 'category': '兴趣/音乐',
     'tags': ['音乐', '社团', '乐器']},
]

POSTS = [
    {'title': '图书馆三楼靠窗的那个男生', 'content': '今天在图书馆三楼靠窗的位置看到了一个超级好看的男生！穿着白色卫衣，戴着银色项链，专注看书的样子太迷人了...有人认识他吗？', 'station': 1, 'author': 1},
    {'title': '二食堂的收银小姐姐', 'content': '每次去二食堂打饭，那个收银小姐姐都会多给我打一点菜，还对我笑。是不是对我有意思？还是我想多了...', 'station': 1, 'author': 4},
    {'title': '捡到一只粉色猫耳耳机', 'content': '在体育馆女更衣室捡到一个粉色猫耳头戴式耳机，品牌是Sony的，看起来很新。请失主联系我认领！', 'station': 2, 'author': 6},
    {'title': '丢了一把透明雨伞', 'content': '昨天下午在教学楼C栋3楼走廊丢了一把透明的雨伞，伞柄上有一个小熊挂件。有看到的同学请联系我，谢谢！', 'station': 2, 'author': 3},
    {'title': '出iPad Air 5 64G 星光色', 'content': '九成新iPad Air 5，64G，星光色，带原装Apple Pencil。因为换了Pro所以出掉，价格可议，面交优先~ 有保护壳和钢化膜', 'station': 3, 'author': 7},
    {'title': '出考研数学全套资料', 'content': '张宇全套+李永乐复习全书+660题+真题，几乎全新只做了几页。打包价80元，有需要的同学私聊', 'station': 3, 'author': 2},
    {'title': '高等数学下册笔记求交换', 'content': '急！有没有人有高等数学下册的笔记？下周就要考试了，我可以请喝奶茶交换！最好是字迹工整的那种~', 'station': 4, 'author': 1},
    {'title': '有没有一起学Python的小伙伴', 'content': '想组一个Python学习小组，每周线上讨论一次，互相监督进度。零基础也欢迎，主要是为了坚持下去！', 'station': 4, 'author': 2},
    {'title': '校园歌手大赛求组队', 'content': '有人想一起组队参加下周的校园歌手大赛吗？我是吉他手，想找一位主唱和鼓手。风格偏民谣和流行，有兴趣的私聊我！', 'station': 5, 'author': 5},
    {'title': '志愿者招募 | 校园环保日', 'content': '下周六校园环保日活动需要志愿者！工作内容：捡拾垃圾+环保宣传。提供午餐和志愿者时长证明，名额有限先到先得~', 'station': 5, 'author': 4},
    {'title': '大三了还不知道自己想做什么', 'content': '看着身边同学都在准备考研或者找实习，我却连自己想做什么都不知道。每天都很焦虑但又不知道该怎么办...有没有同感的同学？', 'station': 6, 'author': 3},
    {'title': '其实我偷偷喜欢室友两年了', 'content': '从来没跟任何人说过...每天看着ta笑我就开心，ta难过我也跟着难过。但是不敢说，怕连朋友都做不了。唉...', 'station': 6, 'author': 1},
    {'title': '二食堂新开的麻辣烫绝了！', 'content': '食堂二楼新开的麻辣烫真的绝了！汤底是骨汤熬的，辣度可以自选，而且蔬菜特别新鲜。推荐大家一定要试试那个手工丸子！排队大概15分钟', 'station': 7, 'author': 1},
    {'title': '三食堂早餐煎饼测评', 'content': '三食堂的杂粮煎饼，加蛋加生菜加脆饼，6块钱一个。味道中规中矩，胜在量大实惠。比外面的干净，适合赶早课的同学', 'station': 7, 'author': 4},
    {'title': '图书馆窗外的夕阳', 'content': '下午在图书馆自习，一抬头就被窗外的景色惊艳到了。金色的阳光洒在银杏树上，整个画面像是一幅油画。忍不住放下笔，静静地看了好一会儿...', 'station': 8, 'author': 1},
    {'title': '雨后的操场 彩虹！', 'content': '傍晚下了一场暴雨，雨停后操场上出现了一道超完整的彩虹！赶紧用手机拍下来了，可惜照片拍不出肉眼看到的那种震撼', 'station': 8, 'author': 6},
    {'title': 'Day 30 | 坚持一个月啦！', 'content': '从开学到现在，整整30天没有间断过自习打卡。虽然有时候真的很想躺平，但看到打卡记录里满满的一页，就觉得一切都值得。明天继续加油！', 'station': 9, 'author': 2},
    {'title': '今天效率超高！', 'content': '从早上8点学到了晚上10点，中间只休息了2小时。高数、英语、专业课都推进了不少。给自己点个赞！希望明天也能保持', 'station': 9, 'author': 4},
    {'title': '失眠了，感觉好孤独', 'content': '又是一个睡不着的夜晚。室友都睡了，只有我还盯着天花板。不知道为什么，明明身边很多人，却总觉得很孤独。也许这就是大学吧...', 'station': 10, 'author': 3},
    {'title': '给明年的自己写一封信', 'content': '亲爱的未来的我：希望你已经找到了自己喜欢的事情，不再迷茫。希望你变得更勇敢，更自信。不管怎样，谢谢你一直在努力。晚安', 'station': 10, 'author': 1},
    {'title': '5km PB 23分钟！', 'content': '今天终于突破了个人最好成绩！5公里跑了23分12秒，比上次快了将近一分钟。果然坚持训练是有用的，继续冲！', 'station': 11, 'author': 4},
    {'title': '本周推荐：《你的名字》', 'content': '本周六晚7点，教学楼A101放映《你的名字》。新海诚的画面配上大荧幕，体验完全不一样。免费入场，座位先到先得！', 'station': 12, 'author': 6},
    {'title': '图书馆门口的橘猫又胖了', 'content': '今天路过图书馆，发现那只常驻的橘猫又圆了一圈。看来不止我一个人在投喂它...附照片一张，这眼神仿佛在说"还有吗？"', 'station': 13, 'author': 3},
    {'title': '考研政治怎么复习？', 'content': '26考研er，政治完全不知道从哪里开始。徐涛和腿姐选哪个？什么时候开始背？求学长学姐指点！', 'station': 14, 'author': 2},
    {'title': '招新啦！零基础也欢迎', 'content': '吉他社春季招新！不管你会不会弹，只要你喜欢音乐，我们都欢迎。社团提供免费教学，还有机会参加校园音乐节。报名链接在评论区~', 'station': 15, 'author': 5},
]

COMMENTS = [
    {'post': 1, 'author': 2, 'content': '我好像认识！是不是戴黑框眼镜那个？'},
    {'post': 1, 'author': 5, 'content': '哈哈我也看到了，在看《百年孤独》对不对'},
    {'post': 1, 'author': 3, 'content': '帮顶！祝你找到他'},
    {'post': 3, 'author': 7, 'content': '是我的！太感谢了！私信你'},
    {'post': 5, 'author': 1, 'content': '多少钱出？有磕碰吗'},
    {'post': 5, 'author': 4, 'content': '3000行不行？可以面交验货'},
    {'post': 7, 'author': 5, 'content': '我有！加微信发你'},
    {'post': 9, 'author': 1, 'content': '我唱歌还行！可以试试'},
    {'post': 13, 'author': 2, 'content': '确实好吃！我连吃三天了'},
    {'post': 13, 'author': 6, 'content': '排队太久了...建议错峰去'},
    {'post': 15, 'author': 2, 'content': '太美了！能发原图吗'},
    {'post': 17, 'author': 1, 'content': '加油！我也在坚持打卡'},
    {'post': 20, 'author': 2, 'content': '写得好温暖，希望你一切都好'},
    {'post': 23, 'author': 1, 'content': '好可爱！它叫什么名字'},
    {'post': 23, 'author': 4, 'content': '大家都叫它"大橘"'},
    {'post': 25, 'author': 5, 'content': '报名！请问在哪里填表'},
]

GROUPS = [
    ('萌新', 'sprout', '#B5EAD7', '刚入学的萌新', 1, 1),
    ('学霸', 'book-open', '#C9E4F5', 'GPA 3.5+', 5, 0),
    ('社交达人', 'party-popper', '#FFB5BA', '粉丝数 > 50', 3, 0),
    ('恋爱达人', 'heart', '#fb6f92', '恋爱专区活跃用户', 3, 0),
    ('交易达人', 'coins', '#FFD6A5', '交易区活跃用户', 3, 0),
    ('摄影师', 'camera', '#E2D5F5', '随手拍区活跃用户', 3, 0),
    ('元老', 'crown', '#ccff00', '注册超过一年', 10, 0),
]

SHOP_ITEMS = [
    ('初学者徽章', '新手专属徽章', 'medal', 50, 0, 'badge'),
    ('学霸徽章', '学习区专属徽章', 'book-open', 100, 0, 'badge'),
    ('恋心徽章', '恋爱区专属徽章', 'heart', 150, 0, 'badge'),
    ('VIP 头像框', '专属头像装饰', 'sparkles', 200, 0, 'avatar_frame'),
    ('改名卡', '修改一次用户名', 'pencil', 300, 0, 'rename'),
    ('置顶卡', '帖子置顶24小时', 'pin', 500, 0, 'pin'),
    ('匿名发帖', '匿名发布一次帖子', 'mask', 80, 0, 'anonymous'),
    ('彩虹昵称', '7天彩虹色昵称', 'palette', 250, 0, 'rainbow_name'),
]

ROMANCE_DATA = [
    (1, '小花', 'female', 20, '中文系', '["读书","摄影"]', '找一个一起自习的人'),
    (4, '大鹏', 'male', 21, '体育系', '["跑步","篮球"]', '找一个一起晨跑的'),
    (5, '糖糖', 'female', 19, '音乐系', '["吉他","唱歌"]', '找一个一起弹琴的'),
    (3, '小绿', 'female', 20, '美术系', '["画画","手工"]', '找一个有艺术细胞的'),
]

GOSSIPS = [
    '听说图书馆三楼有人在偷偷准备求婚...',
    '食堂阿姨今天心情好，打了满满一勺肉！',
    '有人知道操场那只橘猫去哪了吗？两天没看到了',
    '二教的WiFi密码改了，谁知道新密码？',
]

KANBAN_MESSAGES = [
    ('从第一次擦肩到一万次回眸，爱意需要被看见。', 'greeting'),
    ('今天的你也超可爱！记得按时吃饭哦~', 'greeting'),
    ('别换队了，恋爱要双排才好玩。', 'greeting'),
    ('数学不会可以问学姐，恋爱不会可以问本站。', 'greeting'),
]


async def seed(env_override=None, skip_if_seeded=True):
    """执行种子填充。env_override 供本地测试，运行在 Worker 时用绑定 env。"""
    import context
    if env_override:
        context.env = env_override
    if context.env is None:
        raise RuntimeError('缺少 env 绑定（需在 Worker 请求上下文或部署环境中运行）')

    count = await qdb('SELECT COUNT(*) AS c FROM users')
    if count and count[0]['c'] > 0:
        if skip_if_seeded:
            print('Database already seeded. Skipping.')
            return
        await edb('DELETE FROM likes')
        await edb('DELETE FROM comments')
        await edb('DELETE FROM posts')
        await edb('DELETE FROM stations')
        await edb('DELETE FROM users')

    user_ids = []
    for u in USERS:
        uid = await create_user(u['username'], u['email'], u['password'])
        await edb('UPDATE users SET avatar = ?, bio = ?, role = ? WHERE id = ?',
                  (u.get('avatar', ''), u.get('bio', ''), u.get('role', 'user'), uid))
        user_ids.append(uid)

    station_ids = []
    for s in STATIONS:
        sid = await create_station(s['name'], s['description'], s['icon'], s['tags'],
                                   user_ids[0], s.get('category', ''))
        station_ids.append(sid)

    post_ids = []
    for p in POSTS:
        pid = await create_post(p['title'], p['content'],
                                user_ids[p['author'] - 1],
                                station_ids[p['station'] - 1])
        await edb('UPDATE posts SET views = ?, likes_count = ? WHERE id = ?',
                  (random.randint(10, 500), random.randint(5, 200), pid))
        await edb('UPDATE stations SET post_count = post_count + 1 WHERE id = ?',
                  (station_ids[p['station'] - 1],))
        await edb("INSERT OR IGNORE INTO station_members (user_id, station_id) VALUES (?, ?)",
                  (user_ids[p['author'] - 1], station_ids[p['station'] - 1]))
        await edb('UPDATE stations SET user_count = (SELECT COUNT(*) FROM station_members WHERE station_id = ?) WHERE id = ?',
                  (station_ids[p['station'] - 1], station_ids[p['station'] - 1]))
        post_ids.append(pid)

    for cm in COMMENTS:
        cid = await edb('INSERT INTO comments (content, author_id, post_id) VALUES (?, ?, ?)',
                        (cm['content'], user_ids[cm['author'] - 1], post_ids[cm['post'] - 1]))
        await edb('UPDATE posts SET comments_count = comments_count + 1 WHERE id = ?',
                  (post_ids[cm['post'] - 1],))

    follows = [(0, 1), (0, 2), (1, 0), (1, 3), (2, 0), (3, 0), (4, 0), (5, 1), (6, 0)]
    for f, t in follows:
        await edb('INSERT OR IGNORE INTO follows (follower_id, following_id) VALUES (?, ?)',
                  (user_ids[f], user_ids[t]))

    for i in range(len(post_ids)):
        likers = random.sample(range(len(user_ids)), min(3, len(user_ids)))
        for l in likers:
            await edb('INSERT OR IGNORE INTO likes (user_id, target_type, target_id) VALUES (?, ?, ?)',
                      (user_ids[l], 'post', post_ids[i]))
        await edb('UPDATE posts SET likes_count = (SELECT COUNT(*) FROM likes WHERE target_type=\'post\' AND target_id=?) WHERE id = ?',
                  (post_ids[i], post_ids[i]))

    for name, icon, color, desc, min_lv, is_def in GROUPS:
        await edb('INSERT OR IGNORE INTO identity_groups (name, icon, color, description, min_level, is_default) VALUES (?,?,?,?,?,?)',
                  (name, icon, color, desc, min_lv, is_def))

    for i, uid in enumerate(user_ids):
        gname = GROUPS[i % len(GROUPS)][0] if i < len(GROUPS) else ''
        await edb('UPDATE users SET coins = ?, points = ?, level = ?, identity_group = ? WHERE id = ?',
                  (random.randint(50, 500), random.randint(20, 200), random.randint(1, 8), gname, uid))

    for name, desc, icon, coins, points, itype in SHOP_ITEMS:
        await edb('INSERT OR IGNORE INTO shop_items (name, description, icon, price_coins, price_points, item_type) VALUES (?,?,?,?,?,?)',
                  (name, desc, icon, coins, points, itype))

    for uid, nick, gender, age, dept, hobbies, looking in ROMANCE_DATA:
        await edb('''INSERT OR IGNORE INTO romance_profiles (user_id, nickname, gender, age, department, hobbies, looking_for)
                     VALUES (?,?,?,?,?,?,?)''', (uid, nick, gender, age, dept, hobbies, looking))

    for uid in user_ids[:4]:
        for days_ago in range(5):
            d = (datetime.now() - timedelta(days=days_ago)).strftime('%Y-%m-%d')
            await edb('INSERT OR IGNORE INTO checkins (user_id, checkin_date, streak, coins_earned, points_earned) VALUES (?,?,?,?,?)',
                      (uid, d, 5 - days_ago, 10 + (5 - days_ago) * 5, (5 - days_ago) * 2))
        await edb("UPDATE users SET checkin_streak = 5, last_checkin = ? WHERE id = ?",
                  (datetime.now().strftime('%Y-%m-%d'), uid))

    for content in GOSSIPS:
        await edb('INSERT INTO gossip (content, is_anonymous, likes_count) VALUES (?,1,?)',
                  (content, random.randint(5, 100)))

    for msg, mtype in KANBAN_MESSAGES:
        await edb('INSERT OR IGNORE INTO kanban_messages (message, message_type) VALUES (?, ?)',
                  (msg, mtype))

    await edb("INSERT OR IGNORE INTO site_announcements (title, content, created_by) VALUES (?, ?, ?)",
              ('欢迎来到校园墙', '匿名表白、失物招领、二手交易、学习互助……更多玩法等你解锁！', user_ids[7]))

    print(f'Seeded: {len(USERS)} users, {len(STATIONS)} stations, {len(POSTS)} posts, {len(COMMENTS)} comments + extended data')


if __name__ == '__main__':
    import sys
    sys.path.insert(0, '.')
    asyncio.run(seed())