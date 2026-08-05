/**
 * The topic id of the DLsite partnership announcement, linked as 「合作详情」 from
 * the 补票 prompt.
 *
 * Published 2026-07-26: https://www.kungal.com/topic/3867
 *
 * 0 disables the link rather than pointing somewhere plausible — a wrong id would
 * send users to an unrelated topic.
 *
 * Deliberately a frontend constant rather than server config: it is a stable
 * editorial link, not a deployment knob, and keeping it here means the copy and the
 * link it references live in the same place.
 */
export const KUN_DLSITE_ANNOUNCE_TOPIC_ID = 3867
