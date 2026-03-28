package bigcache

import "time"

type clock interface {
	Epoch() int64
}

type systemClock struct {
}

func (c systemClock) Epoch() int64 {
	return time.Now().Unix()
}
