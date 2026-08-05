// Reaction keys + glyphs. Keys are in lock-step with the backend allowlist
// (reactionKeys in apps/api/.../topic_write_service.go) and the .webp assets in
// public/emoji/. The glyphs are the accessible alt/fallback for each animation.

export interface KunReactionMeta {
  key: string
  emoji: string
  label: string
}

// The two effectful reactions, pinned to the top of the picker.
export const KUN_REACTION_LIKE = 'like'
export const KUN_REACTION_DISLIKE = 'dislike'

export const KUN_REACTION_LIKE_NOTE = '点赞会使被点赞的用户萌萌点 +1'
export const KUN_REACTION_DISLIKE_NOTE = '被点踩多的用户将会被智乃屠杀'

// The plain emoji reactions shown in the picker grid.
export const KUN_REACTION_EMOJIS: KunReactionMeta[] = [
  { key: 'heart', emoji: '❤️', label: '爱心' },
  { key: 'fire', emoji: '🔥', label: '火' },
  { key: 'party', emoji: '🎉', label: '庆祝' },
  { key: 'love', emoji: '🥰', label: '喜欢' },
  { key: 'clap', emoji: '👏', label: '鼓掌' },
  { key: 'thinking', emoji: '🤔', label: '思考' },
  { key: 'mindblown', emoji: '🤯', label: '震惊' },
  { key: 'scream', emoji: '😱', label: '尖叫' },
  { key: 'cry', emoji: '😢', label: '哭' },
  { key: 'pray', emoji: '🙏', label: '感谢' },
  { key: 'eyes', emoji: '👀', label: '关注' },
  { key: 'hundred', emoji: '💯', label: '满分' },
  { key: 'partyface', emoji: '🥳', label: '派对' },
  { key: 'starstruck', emoji: '🤩', label: '星星眼' },
  { key: 'angry', emoji: '😠', label: '生气' },
  { key: 'anxious', emoji: '😰', label: '紧张' },
  { key: 'banana', emoji: '🍌', label: '香蕉' },
  { key: 'eyebrow', emoji: '🤨', label: '挑眉' },
  { key: 'voltage', emoji: '⚡', label: '闪电' },
  { key: 'hotdog', emoji: '🌭', label: '热狗' },
  { key: 'hot', emoji: '🥵', label: '热' },
  { key: 'sob', emoji: '😭', label: '大哭' },
  { key: 'moai', emoji: '🗿', label: '摩艾' },
  { key: 'newmoon', emoji: '🌚', label: '黑月亮' },
  { key: 'police', emoji: '🚓', label: '警车' },
  { key: 'pouting', emoji: '😡', label: '怒' },
  { key: 'salute', emoji: '🫡', label: '敬礼' },
  { key: 'shrimp', emoji: '🦐', label: '虾' },
  { key: 'halo', emoji: '😇', label: '天使' },
  { key: 'sunglasses', emoji: '😎', label: '酷' },
  { key: 'whale', emoji: '🐳', label: '鲸鱼' }
]

// Animated .webp asset for a reaction key (compressed Telegram emoji in
// public/emoji/). Used by the picker + bar; the glyph above is the alt/fallback.
export const reactionAsset = (key: string) => `/emoji/${key}.webp`

// key → glyph, for the chip alt text in the bar (includes like/dislike).
export const KUN_REACTION_EMOJI: Record<string, string> = {
  like: '👍',
  dislike: '👎',
  heart: '❤️',
  fire: '🔥',
  party: '🎉',
  love: '🥰',
  clap: '👏',
  thinking: '🤔',
  mindblown: '🤯',
  scream: '😱',
  cry: '😢',
  pray: '🙏',
  eyes: '👀',
  hundred: '💯',
  partyface: '🥳',
  starstruck: '🤩',
  angry: '😠',
  anxious: '😰',
  banana: '🍌',
  eyebrow: '🤨',
  voltage: '⚡',
  hotdog: '🌭',
  hot: '🥵',
  sob: '😭',
  moai: '🗿',
  newmoon: '🌚',
  police: '🚓',
  pouting: '😡',
  salute: '🫡',
  shrimp: '🦐',
  halo: '😇',
  sunglasses: '😎',
  whale: '🐳'
}
