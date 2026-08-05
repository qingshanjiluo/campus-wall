export interface EditStorePersist {
  mode: 'preview' | 'code'

  title: string
  content: string
  category: string
  section: string[]
  isNSFW: boolean
  // Optional 1..9 cover images as /image/<hash> tokens (see useTopicSubmitter).
  coverImages: string[]
}

export interface EditStoreTemp {
  id: number
  title: string
  content: string
  category: string
  section: string[]
  isNSFW: boolean
  coverImages: string[]

  isTopicRewriting: boolean
}
