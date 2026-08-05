import type { KunTabItem, KunSelectOption } from '@kungal/ui-vue'

export type KunGalgameResourceTypeOptions =
  | 'all'
  | 'game'
  | 'patch'
  | 'collection'
  | 'voice'
  | 'image'
  | 'ai'
  | 'video'
  | 'others'

export type KunGalgameResourceLanguageOptions =
  | 'all'
  | 'ja-jp'
  | 'en-us'
  | 'zh-cn'
  | 'zh-tw'
  | 'others'

export type KunGalgameResourcePlatformOptions =
  | 'all'
  | 'windows'
  | 'mac'
  | 'linux'
  | 'emulator'
  | 'app'
  | 'others'

export const KUN_GALGAME_RESOURCE_TYPE_MAP: Record<string, string> = {
  name: '资源链接的类型',
  all: '全部类型',
  game: '游戏本体',
  patch: '补丁',
  collection: '合集',
  voice: '音声相关',
  image: '图片相关',
  ai: 'AI 相关',
  video: '视频相关',
  others: '其它'
}
export const kunGalgameResourceTypeOptions = [
  { value: 'all', label: '全部类型' },
  { value: 'game', label: '游戏本体' },
  { value: 'patch', label: '补丁' },
  { value: 'collection', label: '合集' },
  { value: 'voice', label: '音声相关' },
  { value: 'image', label: '图片相关' },
  { value: 'ai', label: 'AI 相关' },
  { value: 'video', label: '视频相关' },
  { value: 'others', label: '其它' }
] as const
export const KUN_RESOURCE_TYPE_CONST = [
  'game',
  'patch',
  'collection',
  'voice',
  'image',
  'ai',
  'video',
  'others'
] as const

export const KUN_GALGAME_RESOURCE_LANGUAGE_MAP: Record<string, string> = {
  all: '全部语言',
  'ja-jp': '日语',
  'en-us': '英语',
  'zh-cn': '简体中文',
  'zh-tw': '繁体中文',
  others: '其它'
}
export const kunGalgameResourceLanguageOptions = [
  { value: 'all', label: '全部语言' },
  { value: 'ja-jp', label: '日语' },
  { value: 'en-us', label: '英语' },
  { value: 'zh-cn', label: '简体中文' },
  { value: 'zh-tw', label: '繁体中文' },
  { value: 'others', label: '其它' }
] as const
export const KUN_RESOURCE_LANGUAGE_CONST = [
  'ja-jp',
  'en-us',
  'zh-cn',
  'zh-tw',
  'others'
] as const

// A galgame's ORIGINAL language (wiki field `original_language`), distinct from
// the resource-language filter above. The wiki owns this set and stores 30+
// codes (ja-jp / en-us / ru / zh-cn / ko-kr / es / uk / fr / … — both `xx` and
// `xx-yy` forms). The forum used to know only 4, so any Korean/Russian/French/…
// title showed its raw code and — worse — failed the edit form's strict enum on
// re-edit. This is the display set (common languages get a 中文 name; the long
// tail falls back to the raw code). Validation is kept LENIENT separately (see
// validations/galgame.ts) so no wiki code is ever rejected.
export const KUN_GALGAME_ORIGINAL_LANGUAGE_MAP: Record<string, string> = {
  'ja-jp': '日语',
  'en-us': '英语',
  'zh-cn': '简体中文',
  'zh-tw': '繁体中文',
  'ko-kr': '韩语',
  ru: '俄语',
  es: '西班牙语',
  uk: '乌克兰语',
  fr: '法语',
  de: '德语',
  it: '意大利语',
  'pt-br': '葡萄牙语 (巴西)',
  'pt-pt': '葡萄牙语',
  pl: '波兰语',
  id: '印尼语',
  th: '泰语',
  vi: '越南语',
  tr: '土耳其语',
  cs: '捷克语',
  ar: '阿拉伯语',
  nl: '荷兰语',
  sv: '瑞典语',
  others: '其它'
}

// Edit-form dropdown options, derived from the display map (insertion order =
// roughly by prevalence). KunSelect needs the current value to be present or it
// renders blank, which is what forced the manual re-pick before.
export const kunGalgameOriginalLanguageOptions = Object.entries(
  KUN_GALGAME_ORIGINAL_LANGUAGE_MAP
).map(([value, label]) => ({ value, label }))

// Render a galgame original-language code as a name, falling back to the raw
// code for the rare long-tail (so it shows e.g. `ca` rather than breaking).
export const getGalgameOriginalLanguageName = (langCode: string): string =>
  KUN_GALGAME_ORIGINAL_LANGUAGE_MAP[langCode?.toLowerCase()] || langCode

export const KUN_GALGAME_RESOURCE_PLATFORM_MAP: Record<string, string> = {
  name: '资源链接的平台',
  all: '全部平台',
  windows: 'Windows',
  mac: 'macOS',
  linux: 'Linux',
  emulator: '模拟器',
  app: '应用直装',
  others: '其它'
}
export const kunGalgameResourcePlatformOptions = [
  { value: 'all', label: '全部平台' },
  { value: 'windows', label: 'Windows' },
  { value: 'mac', label: 'macOS' },
  { value: 'linux', label: 'Linux' },
  { value: 'emulator', label: '模拟器' },
  { value: 'app', label: '应用直装' },
  { value: 'others', label: '其它' }
] as const
export const KUN_RESOURCE_PLATFORM_CONST = [
  'windows',
  'mac',
  'linux',
  'emulator',
  'app',
  'others'
] as const

export const KUN_GALGAME_RESOURCE_SORT_FIELD_MAP: Record<string, string> = {
  views: '总浏览数',
  time: '更新顺序',
  created: '创建顺序',
  view_1d: '日浏览数',
  view_7d: '周浏览数',
  view_30d: '月浏览数',
  release_date: '发售日期',
  rating: '评分'
}

export const kunGalgameSortFieldOptions: KunSelectOption[] = [
  { value: 'view', label: '浏览顺序' },
  { value: 'time', label: '更新顺序' },
  { value: 'created', label: '创建顺序' }
]

export const galgameIntroductionLanguageTabs: KunTabItem[] = [
  {
    textValue: '英语介绍',
    value: 'en-us'
  },
  {
    textValue: '日语介绍',
    value: 'ja-jp'
  },
  {
    textValue: '简体中文',
    value: 'zh-cn'
  },
  {
    textValue: '繁体中文',
    value: 'zh-tw'
  }
]

export const KUN_GALGAME_AGE_LIMIT_MAP: Record<string, string> = {
  all: '本游戏不含有成人内容',
  r18: '本游戏可能含有成人内容'
}

export const KUN_GALGAME_CONTENT_LIMIT_MAP: Record<string, string> = {
  sfw: '页面内容不含有 R18 内容',
  nsfw: '页面内容可能含有 R18 内容'
}
