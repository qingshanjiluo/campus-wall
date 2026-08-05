// Ambient (global) reaction types — mirrors shared/user.d.ts so topic / reply
// types and components can use them without an import.

interface KunReactorUser {
  id: number
  name: string
  avatar: string
}

// One reaction key on a topic/reply: total count, whether the viewer reacted,
// and — only when count < 5 — the reactors (for avatar display).
interface KunReaction {
  reaction: string
  count: number
  mine: boolean
  reactors?: KunReactorUser[]
}
