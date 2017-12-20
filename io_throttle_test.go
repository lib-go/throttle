package throttle

import (
	"bytes"
	"fmt"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
	"time"
	"io"
)

func Test_sleep_reader_shared_key(t *testing.T) {
	var totalN int
	var spend time.Duration
	var eta time.Duration
	var now time.Time
	var readMaker func() io.Reader

	totalSize := 1024 * 1024 * 2
	rateLimitBPS := 512 * 1024
	count := 3

	blob := make([]byte, totalSize)

	readMaker = func() io.Reader {
		return NewSleepReader(bytes.NewReader(blob), rateLimitBPS, "Test_sleep_reader_shared_key")
	}

	totalN, spend = multiRead(count, readMaker, rateLimitBPS, totalSize/count)

	now = time.Now()
	eta = time.Second * time.Duration(totalN) / time.Duration(rateLimitBPS)

	fmt.Println("total", totalN, "spend", spend, "eta", eta, "eta-spend:", eta-spend)
	assert.WithinDuration(t, now.Add(eta), now.Add(spend), eta/time.Duration(10), "偏差不超过10%")
}

func Test_sleep_writer_shared_key(t *testing.T) {
	var totalN int
	var spend time.Duration
	var eta time.Duration
	var now time.Time
	var writeMaker func() io.Writer

	totalSize := 1024 * 1024 * 2
	rateLimitBPS := 512 * 1024
	count := 3

	writeMaker = func() io.Writer {
		return NewSleepWriter(&bytes.Buffer{}, rateLimitBPS, "Test_sleep_writer_shared_key")
	}

	totalN, spend = multiSend(count, writeMaker, rateLimitBPS, totalSize/count)

	now = time.Now()
	eta = time.Second * time.Duration(totalN) / time.Duration(rateLimitBPS)

	fmt.Println("total", totalN, "spend", spend, "eta", eta, "eta-spend:", eta-spend)
	assert.WithinDuration(t, now.Add(eta), now.Add(spend), eta/time.Duration(10), "偏差不超过10%")
}

func Test_discard_reader_shared_key(t *testing.T) {
	var totalN int
	var spend time.Duration
	var eta time.Duration
	var now time.Time
	var speed int
	var readMaker func() io.Reader

	totalSize := 1024 * 1024 * 2
	rateLimitBPS := 512 * 1024 // bps

	blob := make([]byte, totalSize)
	for i := 0; i < len(blob); i++ {
		blob[i] = byte(i)
	}
	count := 3

	// 低速
	readMaker = func() io.Reader {
		return NewDiscardReader(bytes.NewReader(blob), rateLimitBPS, "Test_discard_reader_shared_key_1")
	}

	speed = (rateLimitBPS - 128*1024) / count
	totalN, spend = multiRead(count, readMaker, speed, totalSize/count)

	now = time.Now()
	eta = time.Second * time.Duration(totalN) / time.Duration(speed*count) // 期望以低速为准

	fmt.Println("total", totalN, "spend", spend, "eta", eta, "eta-spend:", eta-spend)
	assert.WithinDuration(t, now.Add(eta), now.Add(spend), eta/time.Duration(10), "偏差不超过10%")

	// 高速
	readMaker = func() io.Reader {
		return NewDiscardReader(bytes.NewReader(blob), rateLimitBPS, "Test_discard_reader_shared_key_2")
	}

	speed = (rateLimitBPS + 128*1024) / count
	totalN, spend = multiRead(count, readMaker, speed, totalSize/count)

	now = time.Now()
	eta = time.Second * time.Duration(totalN) / time.Duration(rateLimitBPS) // 期望以低速为准

	fmt.Println("total", totalN, "spend", spend, "eta", eta, "eta-spend:", eta-spend)
	assert.WithinDuration(t, now.Add(eta), now.Add(spend), eta/time.Duration(10), "偏差不超过10%")
}

func Test_discard_writer_shared_key(t *testing.T) {
	var totalN int
	var spend time.Duration
	var eta time.Duration
	var now time.Time
	var writeMaker func() io.Writer
	var speed int

	totalSize := 1024 * 1024 * 2
	rateLimitBPS := 512 * 1024 // bps
	count := 3

	//低速通过
	writeMaker = func() io.Writer {
		return NewDiscardWriter(&bytes.Buffer{}, rateLimitBPS, "Test_discard_writer_single_shared_key_1")
	}

	speed = (rateLimitBPS - 128*1024) / count
	totalN, spend = multiSend(count, writeMaker, speed, totalSize/count)

	now = time.Now()
	eta = time.Second * time.Duration(totalN) / time.Duration(speed*count) // 期望以低速为准

	fmt.Println("total", totalN, "spend", spend, "eta", eta, "eta-spend:", eta-spend)
	assert.WithinDuration(t, now.Add(eta), now.Add(spend), eta/time.Duration(10), "偏差不超过10%")

	// 高速丢弃
	writeMaker = func() io.Writer {
		return NewDiscardWriter(&bytes.Buffer{}, rateLimitBPS, "Test_discard_writer_single_shared_key_2")
	}

	speed = (rateLimitBPS + 128*1024) / count
	totalN, spend = multiSend(count, writeMaker, speed, totalSize)

	now = time.Now()
	eta = time.Second * time.Duration(totalN) / time.Duration(rateLimitBPS)

	fmt.Println("total", totalN, "spend", spend, "eta", eta, "eta-spend:", eta-spend)
	assert.WithinDuration(t, now.Add(eta), now.Add(spend), eta/time.Duration(10), "偏差不超过10%")
}

func read(rd io.Reader, speed int, totalSize int) (int, time.Duration) {
	sum := 0
	readDuration := 100 * time.Millisecond
	readSize := speed / int(time.Second/readDuration)
	b := make([]byte, readSize)
	begin := time.Now()
	for sendN := 0; sendN < totalSize; sendN += readSize {
		n, _ := rd.Read(b)
		est := time.Millisecond * time.Duration(float64(sendN+readSize)/float64(speed)*1000)
		duration := est - time.Now().Sub(begin)
		//fmt.Println(sendN+readSize, speed, est, time.Now().Sub(begin), duration)
		if duration > 0 {
			time.Sleep(duration)
		}
		sum += n
	}
	spend := time.Now().Sub(begin)
	//fmt.Println(sum, spend, float64(sum)/float64(speed))
	return sum, spend
}

func multiRead(count int, readMaker func() io.Reader, speed int, totalSize int) (int, time.Duration) {
	wg := sync.WaitGroup{}

	wg.Add(count)

	totalN := 0
	begin := time.Now()
	for i := 0; i < count; i++ {
		go func() {
			rd := readMaker()
			// 确保原子操作
			n, _ := read(rd, speed, totalSize)
			totalN += n
			wg.Done()
		}()
	}
	wg.Wait()

	spend := time.Now().Sub(begin)
	return totalN, spend
}

func send(w io.Writer, speed int, blob []byte) (int, time.Duration) {
	sum := 0
	totalSize := len(blob)
	sendDuration := 100 * time.Millisecond
	sendSize := speed / int(time.Second/sendDuration)
	begin := time.Now()
	for sendN := 0; sendN < totalSize; sendN += sendSize {
		n, _ := w.Write(blob[:sendSize])

		est := time.Millisecond * time.Duration(float64(sendN+sendSize)/float64(speed)*1000)
		duration := est - time.Now().Sub(begin)
		//fmt.Println(sendN+sendSize, speed, est, time.Now().Sub(begin), duration)
		if duration > 0 {
			time.Sleep(duration)
		}
		sum += n
	}
	spend := time.Now().Sub(begin)
	//fmt.Println(sum, spend, float64(sum)/float64(speed))
	return sum, spend
}

func multiSend(count int, writeMaker func() io.Writer, speed int, totalSize int) (int, time.Duration) {
	wg := sync.WaitGroup{}

	wg.Add(count)
	blob := make([]byte, totalSize)

	totalN := 0
	begin := time.Now()
	for i := 0; i < count; i++ {
		go func() {
			w := writeMaker()
			// 确保原子操作
			n, _ := send(w, speed, blob)
			totalN += n
			wg.Done()
		}()
	}
	wg.Wait()

	spend := time.Now().Sub(begin)
	return totalN, spend
}
