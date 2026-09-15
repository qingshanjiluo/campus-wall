"""敏感词检测（R4-M3 审核流核心）。

分级：
- BLOCK   高危（违法/诈骗/导流赌博等）→ 直接拒绝提交
- REVIEW  低俗/辱骂/争议 → 进管理员审核队列（status=pending），不阻断用户操作
匹配：词表为小粒度子串（2+ 字），content.lower() in 判定；量级百条内，成本可忽略。
扩展：设 SENSITIVE_WORDS_FILE 指向 utf-8 词表文件（每行一词，`#` 注释；
前缀 ! 表示 BLOCK 级，缺省 REVIEW 级），与内置词表合并。
"""
import os

_BLOCK = (
    '赌博', '博彩', '开盘', '六合彩', '时时彩', '彩票中奖', '洗钱', '跑分',
    '毒品', '冰毒', '大麻购买', '枪支', '仿真枪购买', '弹药', '管制刀具购买',
    '办证', '刻章', '代开发票', '刷单兼职', '兼职日结佣金', '色情直播',
    '裸贷', '包养', '援交', '一夜情约炮', 'h图', '裸图出售', '翻墙软件出售',
)

_REVIEW = (
    '傻逼', '煞笔', '沙雕玩意', '滚蛋', '去死', '废物', '垃圾东西',
    '小三', '出轨', '偷拍', '尾随', '骚扰', '霸凌', '孤立同学',
    '传销', '微商代理', '加微信返现', '私下交易', '转账',
    '代写论文', '替考', '作弊', '答案出售', '透视',
    '举报你', '曝光你', '人肉', '开盒',
)

_EXTRA_REVIEW = []
_EXTRA_BLOCK = []
_FILE = os.environ.get('SENSITIVE_WORDS_FILE')
if _FILE and os.path.isfile(_FILE):
    try:
        with open(_FILE, encoding='utf-8') as f:
            for line in f:
                w = line.strip()
                if not w or w.startswith('#'):
                    continue
                if w.startswith('!'):
                    _EXTRA_BLOCK.append(w[1:].strip().lower())
                else:
                    _EXTRA_REVIEW.append(w.lower())
    except OSError:
        pass


def scan_text(text):
    """返回 'block' | 'review' | None 三级判定。"""
    if not text:
        return None
    t = str(text).lower()
    for w in list(_BLOCK) + _EXTRA_BLOCK:
        if w and w in t:
            return 'block'
    for w in list(_REVIEW) + _EXTRA_REVIEW:
        if w and w in t:
            return 'review'
    return None
