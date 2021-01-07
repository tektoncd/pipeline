package zapdriver

import (
	"go.uber.org/zap"
)

// NewProduction builds a sensible production Logger that writes InfoLevel and
// above logs to standard error as JSON.
//
// It's a shortcut for NewProductionConfig().Build(...Option).
func NewProduction(options ...zap.Option) (*zap.Logger, error) {
	options = append(options, WrapCore())

	return NewProductionConfig().Build(options...)
}

// NewProductionWithCore is same as NewProduction but accepts a custom configured core
func NewProductionWithCore(core zap.Option, options ...zap.Option) (*zap.Logger, error) {
	options = append(options, core)

	return NewProductionConfig().Build(options...)
}

// NewDevelopment builds a development Logger that writes DebugLevel and above
// logs to standard error in a human-friendly format.
//
// It's a shortcut for NewDevelopmentConfig().Build(...Option).
func NewDevelopment(options ...zap.Option) (*zap.Logger, error) {
	options = append(options, WrapCore())

	return NewDevelopmentConfig().Build(options...)
}

// NewDevelopmentWithCore is same as NewDevelopment but accepts a custom configured core
func NewDevelopmentWithCore(core zap.Option, options ...zap.Option) (*zap.Logger, error) {
	options = append(options, core)

	return NewDevelopmentConfig().Build(options...)
}
