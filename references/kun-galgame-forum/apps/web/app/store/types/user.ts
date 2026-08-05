export interface UserStore {
  id: number
  // OAuth subject UUID — the stable account identity used as the account-switch
  // login_hint (the local integer id is not what OAuth keys on).
  sub: string
  name: string
  avatar: string
  avatarMin: string
  moemoepoint: number
  // Raw OAuth role list (e.g. ["admin"]) — an unordered SET of capability
  // claims. `user` is implicit and NEVER appears; a plain logged-in user has
  // roles = []. Every capability (canModerate / canAdminister / isCreator,
  // including the avatar-menu "创作者申请" gate) is derived from this set via
  // useRole(); there is no numeric tier and no separate creator flag.
  roles: string[]
  isCheckIn: boolean
  dailyToolsetUploadBytes: number
}
