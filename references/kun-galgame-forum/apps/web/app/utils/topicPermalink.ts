// Deep-link helpers for the home feed cards. `topicLink` is a card's plain
// `/topic/:id`; append the query the topic-detail page reads (Detail.vue locates
// the reply-stream page from ?reply=<floor> / ?comment=<id>, then scrolls there).
// Both fall back to the bare topic link when the floor / id is missing (0), so a
// card whose target was deleted or never enriched still links to the topic, never
// to a broken anchor.
export const replyPermalink = (topicLink: string, floor?: number) =>
  floor && floor > 0 ? `${topicLink}?reply=${floor}` : topicLink

export const commentPermalink = (topicLink: string, commentId?: number) =>
  commentId && commentId > 0 ? `${topicLink}?comment=${commentId}` : topicLink
