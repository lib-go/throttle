package throttle

import (
	"errors"
	"sync"
	"time"
)

var (
	DefaultRateController = NewRateController()

	ErrorDiscard = errors.New("[]byte discard")
)

type ReadWriteFunc func(b []byte) (int, error)

func (f ReadWriteFunc) Read(b []byte) (n int, e error) {
	return f(b)
}

func (f ReadWriteFunc) Write(b []byte) (n int, e error) {
	return f(b)
}

type rater struct {
	sync.Mutex

	rateLimitBPS int
	availableAt  *time.Time // 在此时间之前不可用
}

func (r *rater) RecordTransfer(n int) {
	if n > 0 && r.rateLimitBPS > 0 {
		eta := time.Microsecond * time.Duration(float32(n)/float32(r.rateLimitBPS)*1000*1000)

		*(r.availableAt) = r.availableAt.Add(eta) // todo：如果长时间低速传输会导致后面可以长速传输
		// todo: 能不能改在成为 Take 接口
	}
}

func (r *rater) WaitTime() time.Duration {
	duration := r.availableAt.Sub(cachedNow())

	if duration > 0 {
		return duration
	} else {
		return time.Duration(0)
	}
}

type rateController struct {
	sync.Mutex

	raters map[string]*rater
}

func NewRateController() (m *rateController) {
	m = &rateController{
		raters: make(map[string]*rater),
	}
	return m
}

func (m *rateController) makeRater(rateLimitBPS int) (r *rater) {
	maybeInitNowCache()

	r = &rater{
		rateLimitBPS: rateLimitBPS,
		availableAt:  new(time.Time),
	}
	*(r.availableAt) = cachedNow()

	return r
}

func (m *rateController) getOrCreateRater(rateLimitBPS int, key string) (r *rater) {
	if key == "" {
		r = m.makeRater(rateLimitBPS)
	} else {
		m.Lock()
		if r = m.raters[key]; r == nil || r.rateLimitBPS != rateLimitBPS {
			r = m.makeRater(rateLimitBPS)
			m.raters[key] = r
		}
		m.Unlock()
	}
	return
}

func (m *rateController) SleepLimit(fi ReadWriteFunc, rateLimitBPS int, key string) (fo ReadWriteFunc) {
	r := m.getOrCreateRater(rateLimitBPS, key)

	return func(b []byte) (n int, e error) {
		n, e = fi(b)
		r.RecordTransfer(n)

		if duration := r.WaitTime(); duration > 0 {
			time.Sleep(duration)
		}
		return n, e
	}
}

func (m *rateController) DiscardLimit(fi ReadWriteFunc, rateLimitBPS int, key string) (fo ReadWriteFunc) {
	r := m.getOrCreateRater(rateLimitBPS, key)

	return func(b []byte) (n int, e error) {
		if duration := r.WaitTime(); duration > 0 {
			n, e = 0, ErrorDiscard
		} else {
			n, e = fi(b)
			r.RecordTransfer(n)
		}
		return
	}
}

func SleepLimit(fi ReadWriteFunc, rateLimitBPS int, key string) (fo ReadWriteFunc) {
	return DefaultRateController.SleepLimit(fi, rateLimitBPS, key)
}

func DiscardLimit(fi ReadWriteFunc, rateLimitBPS int, key string) (fo ReadWriteFunc) {
	return DefaultRateController.DiscardLimit(fi, rateLimitBPS, key)
}
