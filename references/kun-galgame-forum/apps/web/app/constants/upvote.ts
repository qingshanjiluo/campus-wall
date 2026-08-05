// Playful "why I pushed this topic" blurbs for the 推话题 feed card. When a user
// upvotes a topic WITHOUT writing their own one-liner, the card shows one of
// these (picked deterministically by the activity id, so it stays stable per
// item but varies across the feed). Kept ≤30 chars to match the input limit.
export const KUN_TOPIC_UPVOTE_DESCRIPTIONS = [
  '他觉得这个萝莉写的太好了，足足给了她十根棒棒糖！',
  '感觉吾之暗黑的半身要觉醒了，必须推！',
  '好家伙敢这么写，我必须让全部人看到你',
  '这帖子的含金量，连我家的猫都看呆了',
  '不推不是绅士，推了才是真の同志！',
  '月色真美，这篇帖子也真美，所以我推了',
  '看完直接钉在了我的精神时光屋墙上',
  '这是什么神仙楼主，给我狠狠地顶上去！',
  '吾辈楷模，此贴当浮一大白！',
  '我的钱包空了，但我的心满了，值！',
  '熬夜也要看完，所以决定拉更多人一起熬夜',
  '这波操作我直接跪着看完，膝盖已奉上',
  '别问，问就是太香了，推！',
  '此贴一出，谁与争锋？必须顶进我心里',
  '为了这碟醋，我能包一整顿饺子',
  '笑死，这都能写出来，天才吧？强烈安利！',
  '我愿称之为本周最佳，没有之一',
  '看完之后我悟了，你们也该悟一悟',
  '萌即是正义，此贴萌度爆表，推上天！',
  '这文笔建议直接出道，先收下我十萌萌点',
  '楼主是懂我的，这种好东西不能我一个人看',
  '警告：本贴含有大量上头成分，已替你测试',
  '路见好贴，拔刀相助，这一推义不容辞',
  '我的 DNA 动了，必须让大家的一起动'
] as const

// Deterministic pick by id — same item always shows the same blurb, but
// different upvotes spread across the pool.
export const randomUpvoteDescription = (seed: number): string =>
  KUN_TOPIC_UPVOTE_DESCRIPTIONS[
    Math.abs(seed) % KUN_TOPIC_UPVOTE_DESCRIPTIONS.length
  ]!
