export type GalgameRevisionAction =
  | 'created'
  | 'updated'
  | 'merged'
  | 'reverted'
  | 'declined'

export interface GalgameRevision {
  id: number
  revision: number
  action: GalgameRevisionAction
  note: string
  user: KunUser
  is_minor: boolean
  created: Date | string
}
