package throttle

import (
	"bytes"
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"sync"
	"testing"
	"time"
)

func Test_reader_single(t *testing.T) {
	totalSize := 1024 * 1024
	rateLimitBPS := 512 * 1024 // bps
	var eta time.Duration = time.Second * time.Duration(totalSize) / time.Duration(rateLimitBPS)

	blob := make([]byte, totalSize)
	tr := NewReader(bytes.NewReader(blob), rateLimitBPS, "")

	begin := time.Now()
	ioutil.ReadAll(tr)
	spend := time.Now().Sub(begin)

	fmt.Println(spend)
	now := time.Now()
	assert.WithinDuration(t, now.Add(eta), now.Add(spend), eta/time.Duration(10), "偏差不超过10%")
}

func Test_reader_shared_key(t *testing.T) {
	totalSize := 1024 * 1024
	rateLimitBPS := 512 * 1024
	readerCount := 3
	var eta time.Duration = time.Second * time.Duration(totalSize) / time.Duration(rateLimitBPS) * time.Duration(readerCount)

	var wg sync.WaitGroup
	wg.Add(readerCount)

	begin := time.Now()
	for i := 0; i < readerCount; i++ {
		go func() {
			tr := NewReader(bytes.NewReader(make([]byte, totalSize)), rateLimitBPS, "shared-key")
			ioutil.ReadAll(tr)
			wg.Done()
		}()
	}
	wg.Wait()

	spend := time.Now().Sub(begin)
	fmt.Println(spend)
	now := time.Now()
	assert.WithinDuration(t, now.Add(eta), now.Add(spend), eta/time.Duration(10), "偏差不超过10%")
}
