package utils

import "time"

type FixedDelay time.Duration

func (d FixedDelay) SampleDuration() time.Duration {
	return time.Duration(d)
}

func (d FixedDelay) Quantile(probability float64) time.Duration {
	return time.Duration(d)
}
