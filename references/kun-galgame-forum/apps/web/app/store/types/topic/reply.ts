export interface ReplyRewriteData {
  id: number
  mainContent: string
}

export interface SuccessfulReplyEvent {
  data: TopicReply
  type: 'created' | 'updated' | 'deleted'
}

// A floor being 「引用」-ed. Carried as a one-shot signal (tempReply.pendingQuote)
// to the open reply editor, which inserts an inline @mention + #quote header into
// its (focused, live) document via a ProseMirror transaction — the same path the
// @-mention dropdown uses, so the caret behaves natively (incl. IME). NOT a
// persisted/aside field: once inserted it lives in the editor content itself.
export interface ReplyReference {
  userId: number
  userName: string
  replyId: number
  floor: number
}

export interface ReplyStoreTemp {
  isEdit: boolean
  isScrollToTop: boolean
  scrollToReplyId: number
  isReplyRewriting: boolean
  replyRewrite: ReplyRewriteData | null
  lastSuccessfulReply: SuccessfulReplyEvent | null
  // Set by a 「引用」 click; consumed (and cleared) by the reply editor once it
  // has inserted the inline header.
  pendingQuote: ReplyReference | null
}

export interface ReplyStorePersist {
  mode: 'preview' | 'source'
  // A reply is now a single body (inline @mention / #quote tokens replaced the
  // old per-target editors). mainContent is markdown.
  replyDraft: {
    mainContent: string
  }
}
