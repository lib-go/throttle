package throttle

import (
	"errors"
	"time"
	"fmt"
)
// 仅实现了一下，还没正式用

var (
	ErrNotEnoughTokens = errors.New("not enough tokens")
)

type TokenBucket struct {
	rate     uint      // per second
	tokens   uint      // 当前有多少token
	capacity uint      // token最大总量
	filledAt time.Time // 上次fill时间
}

func (b *TokenBucket) fill(now time.Time) {

	if b.filledAt.IsZero() {
		// 初始化
		b.tokens = b.capacity
	} else if now.After(b.filledAt) {
		// 根据距离上次注入的时间，计算应该增加的token
		newTokens := uint(float64(b.rate) * (now.Sub(b.filledAt).Seconds()))
		b.tokens += newTokens
		if b.tokens > b.capacity {
			b.tokens = b.capacity
		}
		fmt.Println("token + ", newTokens, "=", b.tokens)
	} else {
		// cachedNow >= filledAt，无效fill，跳出
		return
	}

	b.filledAt = now
}

func (b *TokenBucket) Take(n int, now time.Time) (e error, timeToWait time.Duration) {
	b.fill(now)
	un := uint(n)

	if b.tokens < un {
		e = ErrNotEnoughTokens
		timeToWait = time.Duration(float64(time.Second) * float64(un-b.tokens) / float64(b.rate))
		return
	}

	b.tokens -= un
	fmt.Println("token - ", un, "=", b.tokens)

	return
}
