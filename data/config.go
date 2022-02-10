package data

import "errors"

type FeaturesConfig struct {
	WithReactions bool
	WithFiles     bool
}

var ErrFeatureDisabled = errors.New("feature disabled")
