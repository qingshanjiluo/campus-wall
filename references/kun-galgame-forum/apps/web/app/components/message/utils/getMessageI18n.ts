const messageTemplates: Record<string, Record<string, string>> = {
  'en-us': {
    upvoted: ' upvoted you!',
    liked: ' liked you!',
    favorite: ' favorite you!',
    replied: ' replied you!',
    commented: ' commented you!',
    expired: ' report for your resource link has expired!',
    requested: ' has requested an update from you!',
    merged: ' merged your update request!',
    declined: ' declined your update request!',
    admin: 'System message',
    mentioned: ' mentioned you!',
    'quiz-answered': ' answered your quiz!',
    default: ' {{action}} you!'
  },
  'ja-jp': {
    upvoted: ' があなたを推しました！',
    liked: ' があなたに「いいね！」をしました！',
    favorite: ' があなたをお気に入りに追加しました！',
    replied: ' があなたに返信しました！',
    commented: ' があなたにコメントしました！',
    expired: ' があなたのリソースリンクの期限切れを報告しました！',
    requested: ' があなたに更新リクエストを送信しました！',
    merged: ' があなたの更新リクエストをマージしました！',
    declined: ' があなたの更新リクエストを拒否しました！',
    admin: 'システムメッセージ',
    mentioned: ' があなたをメンションしました！',
    'quiz-answered': ' があなたのクイズに回答しました！'
  },
  'zh-cn': {
    upvoted: ' 推了您!',
    liked: ' 点赞了您!',
    favorite: ' 收藏了您!',
    replied: ' 回复了您!',
    commented: ' 评论了您!',
    expired: ' 报告了您的资源链接已过期！',
    requested: ' 向您提出更新请求！',
    solution: '您的回复被标记为最佳答案!',
    merged: ' 合并了您的更新请求！',
    declined: ' 拒绝了您的更新请求！',
    admin: '系统消息',
    mentioned: ' 提到了您！',
    'quiz-answered': ' 回答了您的题目!'
  },
  'zh-tw': {
    upvoted: ' 推了您!',
    liked: ' 點贊了您!',
    favorite: ' 收藏了您!',
    replied: ' 回復了您!',
    commented: ' 評論了您!',
    expired: ' 報告了您的資源鏈接已過期！',
    requested: ' 嚮您提出更新請求！',
    merged: ' 合併了您的更新請求！',
    declined: ' 拒絕了您的更新請求！',
    admin: '繫統消息',
    mentioned: ' 提到了您！',
    'quiz-answered': ' 回答了您的題目!'
  }
}

const getMessageContent = (locale: Language, message: Message): string => {
  const template =
    messageTemplates[locale]![message.type]! ||
    messageTemplates[locale]!.default
  return template ?? ''
}

export const getMessageI18n = (message: Message) => {
  if (message.type === 'admin') {
    return messageTemplates['zh-cn']!.admin
  }

  // A @mention that carries reply text reads as a reply ("回复了您", with the
  // text shown below); a bare mention with no body stays "提到了您".
  if (message.type === 'mentioned' && message.content?.trim()) {
    return messageTemplates['zh-cn']!.replied
  }

  return getMessageContent('zh-cn', message)
}
