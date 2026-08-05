// MUST match the backend's constants.TopicSectionConsume (g-seeking / g-other /
// t-help). Missing one here means the FE won't warn that posting there costs
// moemoepoint, but the server charges anyway.
export const TOPIC_SECTION_CONSUME_MOEMOEPOINTS = [
  'g-seeking',
  'g-other',
  't-help'
] as const

export const MOEMOEPOINT_COST_FOR_CONSUME_SECTION = 10
