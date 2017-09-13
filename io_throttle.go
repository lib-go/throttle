package throttle

import (
	"io"
	"sync"
	"time"
)

var (
	defaultRateControl *rateControl
)

type rateControl struct {
	sync.Mutex

	now     time.Time
	times   map[string]*time.Time
	ticking bool
}

func NewRateControl() *rateControl {
	return &rateControl{
		now:   time.Now(),
		times: make(map[string]*time.Time),
	}
}

func (c *rateControl) foreverTick() {
	for {
		c.now = time.Now()
		time.Sleep(time.Millisecond * 100)
	}
}

func (c *rateControl) NewReader(rd io.Reader, rateLimitBPS int, key string) *reader {
	if !c.ticking {
		c.ticking = true
		go c.foreverTick()
	}

	// 作为reader的下次读取时间，同一个key的reader的nextReadTime指向同一个*time.Time
	var timeRef *time.Time
	if key == "" {
		timeRef = new(time.Time)
	} else {
		c.Lock()
		if timeRef = c.times[key]; timeRef == nil {
			timeRef = new(time.Time)
			c.times[key] = timeRef
		}
		c.Unlock()
	}
	*timeRef = c.now

	return &reader{
		rc:           c,
		rd:           rd,
		rateBPS:      rateLimitBPS,
		nextReadTime: timeRef,
	}
}

type reader struct {
	rc *rateControl

	rd           io.Reader
	rateBPS      int // kbps
	nextReadTime *time.Time
}

func NewReader(reader io.Reader, rateLimitBPS int, key string) *reader {
	if defaultRateControl == nil {
		defaultRateControl = NewRateControl()
	}

	return defaultRateControl.NewReader(reader, rateLimitBPS, key)
}

func (r *reader) Read(chunk []byte) (n int, e error) {
	n, e = r.rd.Read(chunk)

	if r.rateBPS > 0 {
		estimateReadDuration := time.Second * time.Duration(n) / time.Duration(r.rateBPS)
		*(r.nextReadTime) = r.nextReadTime.Add(estimateReadDuration)
		if r.nextReadTime.After(r.rc.now) {
			time.Sleep(r.nextReadTime.Sub(r.rc.now))
		}
	}
	return
}
