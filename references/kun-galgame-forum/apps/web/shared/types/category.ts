export interface CategoryLatestTopicInfo {
  id: number
  title: string
  created: Date | string
}

export interface CategorySectionStats {
  id: number
  name: string
  topic_count: number
  view_count: number
  latest_topic: CategoryLatestTopicInfo | null
}
