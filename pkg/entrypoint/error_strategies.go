package entrypoint

type ErrorStrategy string

const (
	SkipOnPriorStepErrors ErrorStrategy = "SkipOnPriorStepErrors"
	IgnorePriorStepErrors ErrorStrategy = "IgnorePriorStepErrors"
)

var allErrorStrategies = []ErrorStrategy{SkipOnPriorStepErrors, IgnorePriorStepErrors}

func IsValidErrorStrategy(e ErrorStrategy) bool {
	for _, strat := range allErrorStrategies {
		if e == strat {
			return true
		}
	}
	return false
}

func GetErrorStrategyNames() []string {
	ret := []string{}
	for _, s := range allErrorStrategies {
		ret = append(ret, string(s))
	}
	return ret
}
