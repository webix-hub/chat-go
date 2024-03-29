package data

import "errors"

type FeaturesConfig struct {
	WithReactions     bool
	WithFiles         bool
	WithBots          bool
	WithGroupCalls    bool
	WithVoiceMessages bool
}

var ErrFeatureDisabled = errors.New("feature disabled")
var ErrAccessDenied = errors.New("access denied")
