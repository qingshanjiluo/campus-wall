package constants

// Moemoepoint rewards and costs for various operations.
// Like operations give moemoepoint to the CONTENT OWNER (被赞者), not the liker.
const (
	RewardCreateTopic    = 3
	RewardCreateGalgame  = 3
	RewardCreateResource = 3
	RewardCreateToolset  = 3
	RewardReply          = 1
	RewardPRMerge        = 1

	CostConsumeSection = 10
	CostChangeUsername = 17
	CostUpvoteSender   = 10
	RewardUpvoteOwner  = 5

	// Rating reward tiers based on short_summary length
	RatingRewardHigh         = 10
	RatingRewardMedium       = 5
	RatingRewardLow          = 3
	RatingLenThresholdHigh   = 666
	RatingLenThresholdMedium = 233

	// Quiz: authoring grants a FLAT reward regardless of difficulty (granted at
	// create time, refunded on delete). A correct answer currently grants
	// nothing — QuizCorrectReward is kept as a knob to re-enable a
	// difficulty-scaled answer reward later.
	QuizCreateReward  = 2
	QuizCorrectReward = 0

	TextPreviewLength = 233
)
