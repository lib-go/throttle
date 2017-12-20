package throttle

import (
	"testing"
	"time"
	"github.com/stretchr/testify/assert"
	"fmt"
)

func TestTokenBucket_Take(t *testing.T) {
	var e error
	var now = time.Now()
	var wt time.Duration

	b := &TokenBucket{
		rate:     100,
		capacity: 200,
	}

	e, _ = b.Take(10, now)
	assert.Nil(t, e)
	assert.Equal(t, uint(b.capacity-10), b.tokens)

	e, wt = b.Take(200, now)
	assert.Equal(t, ErrNotEnoughTokens, e)
	assert.Equal(t, float64(10)/float64(100), wt.Seconds())
	fmt.Println(wt)

}
