// The default collection stores an empty name: the SQL migration that seeds it
// has no username (identity is OAuth-owned — contract C6), so "<用户名>的收藏夹"
// is rendered dynamically here instead of being persisted. A user who renames
// their default collection gets a non-empty name, which we then show verbatim.
export const collectionDisplayName = (
  c: { is_default: boolean; name: string },
  ownerName?: string
): string => {
  if (c.is_default && !c.name) {
    return `${ownerName || '我'}的收藏夹`
  }
  return c.name
}

// The default collection also stores an empty description; render it dynamically
// for the same reason (no username in this DB). A user-set description wins.
export const collectionDisplayDescription = (
  c: { is_default: boolean; description: string },
  ownerName?: string
): string => {
  if (c.is_default && !c.description) {
    const who = ownerName || '我'
    return `${who}所有收藏的游戏已经被放置在 ${who}的收藏夹`
  }
  return c.description
}
