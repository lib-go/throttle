package throttle

import (
	"fmt"
	"io"
	"sync"
	"time"
)

const defaultNowCacheInterval = 100 * time.Millisecond

var (
	defaultRateControl = &rateControl{
		raters: make(map[string]*rater),
	}
	defaultNowCache    *nowCache
	mutex              = &sync.Mutex{}
)

type nowCache struct {
	Now      time.Time
}

func newNowCache() {
	mutex.Lock()
	if defaultNowCache == nil {
		defaultNowCache = &nowCache{Now: time.Now()}
		go defaultNowCache.foreverTick()
	}
	mutex.Unlock()
}

func (c *nowCache) foreverTick() {
	for {
		time.Sleep(defaultNowCacheInterval)
		c.Now = time.Now()
	}
}

// rateControl
type rateControl struct {
	sync.Mutex
	raters map[string]*rater
}

func (c *rateControl) NewRater(rateLimitBPS int, key string) *rater {
	// 作为reader的下次读取时间，同一个key的reader的recordTime指向同一个*time.Time
	var raterRef *rater
	makeRetRef := func() *rater {
		r := &rater{
			rateBPS:      rateLimitBPS,
			estimateTime: new(time.Time),
		}
		*(r.estimateTime) = defaultNowCache.Now
		return r
	}

	if key == "" {
		raterRef = makeRetRef()
	} else {
		c.Lock()

		if raterRef = c.raters[key]; raterRef == nil {
			raterRef = makeRetRef()
			c.raters[key] = raterRef
		}
		c.Unlock()
	}

	return raterRef
}

// rater
type rater struct {
	sync.Mutex

	rateBPS      int
	estimateTime *time.Time
}

func (r *rater) UpdateEstimateTime(n int) {
	if n > 0 && r.rateBPS > 0 {
		//fmt.Println(n, "est:", time.Microsecond*time.Duration(float32(n)/float32(r.rateBPS)*1000*1000))
		estimate := time.Microsecond * time.Duration(float32(n)/float32(r.rateBPS)*1000*1000)
		*r.estimateTime = r.estimateTime.Add(estimate)
	}
}

func (r *rater) WaitingDuration() time.Duration {
	duration := r.estimateTime.Sub(defaultNowCache.Now)

	if duration > 0 {
		return duration
	} else {
		return time.Duration(0)
	}
}

// rw
var ErrorDiscard = fmt.Errorf("[]byte discard")

type readWrite func(chunk []byte) (int, error)
type raterRW func(chunk []byte, ope readWrite, rt *rater) (n int, e error)

func raterSleep(chunk []byte, ope readWrite, rt *rater) (n int, e error) {
	n, e = ope(chunk)
	rt.UpdateEstimateTime(n)

	if duration := rt.WaitingDuration(); duration > 0 {
		time.Sleep(duration)
	}
	return n, e
}

func raterDiscard(chunk []byte, ope readWrite, rt *rater) (n int, e error) {
	// 先等再允许写入, 避免某个包写入时间过长, 后面包一直丢弃
	if duration := rt.WaitingDuration(); duration > 0 {
		n, e = 0, ErrorDiscard
	} else {
		n, e = ope(chunk)
		rt.UpdateEstimateTime(n)
	}
	return n, e
}

type readWriter struct {
	io.ReadWriter
	ope readWrite
	rrw raterRW
	rt  *rater
}

func (r *readWriter) Read(chunk []byte) (int, error) {
	return r.rrw(chunk, r.ope, r.rt)
}

func (r *readWriter) Write(chunk []byte) (int, error) {
	return r.rrw(chunk, r.ope, r.rt)
}

func NewReaderWriter(ope readWrite, rateLimitBPS int, key string, rrw raterRW) *readWriter {
	newNowCache()

	rt := defaultRateControl.NewRater(rateLimitBPS, key)
	return &readWriter{ope: ope, rrw: rrw, rt: rt}
}

// builder
func NewSleepReader(rd io.Reader, rateLimitBPS int, key string) *readWriter {
	return NewReaderWriter(rd.Read, rateLimitBPS, key, raterSleep)
}

func NewDiscardReader(rd io.Reader, rateLimitBPS int, key string) *readWriter {
	return NewReaderWriter(rd.Read, rateLimitBPS, key, raterDiscard)
}

func NewSleepWriter(w io.Writer, rateLimitBPS int, key string) *readWriter {
	return NewReaderWriter(w.Write, rateLimitBPS, key, raterSleep)
}

func NewDiscardWriter(w io.Writer, rateLimitBPS int, key string) *readWriter {
	return NewReaderWriter(w.Write, rateLimitBPS, key, raterDiscard)
}
