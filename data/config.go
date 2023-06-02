package data

import "errors"

type FeaturesConfig struct {
	WithReactions  bool
	WithFiles      bool
	WithGroupCalls bool
	WithVoice      bool
}

var ErrFeatureDisabled = errors.New("feature disabled")
var ErrAccessDenied = errors.New("access denied")
