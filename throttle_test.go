package throttle

import (
	"bytes"
	"fmt"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
	"time"
	"io/ioutil"
	"io"
	"log"
	"sync/atomic"
)

type testReader struct {
	Read ReadWriteFunc
}

type testWriter struct {
	Write ReadWriteFunc
}

type mockReader struct {
	read ReadWriteFunc
}

func (r *mockReader) Read(b []byte) (n int, e error) {
	return r.read(b)
}

func newMockReader(read ReadWriteFunc) (io.Reader) {
	return &mockReader{read: read}
}

func TestSingleSleepLimit(t *testing.T) {
	// 一个Reader
	totalSize := 1024 * 1024 * 2
	rateLimitBPS := 512 * 1024

	bin := make([]byte, totalSize)
	buf := bytes.NewBuffer(bin)

	rd := newMockReader(SleepLimit(buf.Read, rateLimitBPS, ""))

	now := time.Now()
	bout, _ := ioutil.ReadAll(rd)
	lapsed := time.Now().Sub(now)
	eta := time.Second * time.Duration(totalSize) / time.Duration(rateLimitBPS)

	log.Printf("input/output=%d/%d, lapsed=%v, eta=%v, delta=%v(%.0f%%)",
		totalSize, len(bout), lapsed, eta, eta-lapsed, float64(lapsed-eta)/float64(eta)*100)

	assert.WithinDuration(t, now.Add(eta), now.Add(lapsed), eta/time.Duration(10), "偏差不超过10%")
}

func TestSharedSleepLimit(t *testing.T) {
	// 多个同样key的reader，并行读取，共同限速
	totalSize := 1024 * 1024
	rateLimitBPS := 512 * 1024
	concurrent := 2

	var wg sync.WaitGroup
	wg.Add(concurrent)

	var totalSizeOut uint32 = 0
	for i := 0; i < concurrent; i++ {
		go func() {
			bin := make([]byte, totalSize)
			buf := bytes.NewBuffer(bin)
			rd := newMockReader(SleepLimit(buf.Read, rateLimitBPS, "TestSharedSleepLimit"))
			bout, _ := ioutil.ReadAll(rd)
			atomic.AddUint32(&totalSizeOut, uint32(len(bout)))
			wg.Done()
		}()
	}

	now := time.Now()
	wg.Wait()
	lapsed := time.Now().Sub(now)
	eta := time.Second * time.Duration(totalSize) / time.Duration(rateLimitBPS) * time.Duration(concurrent)

	log.Printf("input/output=%d/%d, lapsed=%v, eta=%v, delta=%v(%.0f%%)",
		totalSize*concurrent, totalSizeOut, lapsed, eta, eta-lapsed, float64(lapsed-eta)/float64(eta)*100)

	assert.WithinDuration(t, now.Add(eta), now.Add(lapsed), eta/time.Duration(10), "偏差不超过10%")
}

func TestSingleDiscardLimit(t *testing.T) {
	// 一个Reader
	totalSize := 1024 * 1024 * 20
	rateLimitBPS := 1024 * 1024 * 10

	bin := make([]byte, totalSize)
	buf := bytes.NewBuffer(bin)
	readDelay := time.Millisecond * 60
	rd := newMockReader(DiscardLimit(func(b []byte) (int, error) {
		n, e := buf.Read(b)
		time.Sleep(readDelay)
		return n, e
	}, rateLimitBPS, ""))

	now := time.Now()
	bout, _ := ioutil.ReadAll(rd)
	lapsed := time.Now().Sub(now)
	eta := time.Second * time.Duration(len(bout)) / time.Duration(rateLimitBPS)

	log.Printf("input/output=%d/%d, lapsed=%v, eta=%v, delta=%v(%.0f%%)",
		totalSize, len(bout), lapsed, eta, eta-lapsed, float64(lapsed-eta)/float64(eta)*100)

	assert.WithinDuration(t, now.Add(eta), now.Add(lapsed), eta/time.Duration(10), "偏差不超过10%")
}

func Test_discard_reader_shared_key(t *testing.T) {
	var totalN int
	var spend time.Duration
	var eta time.Duration
	var now time.Time
	var speed int
	var readMaker func() *testReader

	totalSize := 1024 * 1024 * 2
	rateLimitBPS := 512 * 1024 // bps

	blob := make([]byte, totalSize)
	for i := 0; i < len(blob); i++ {
		blob[i] = byte(i)
	}
	count := 3

	// 低速
	readMaker = func() *testReader {
		return &testReader{
			Read: DiscardLimit(bytes.NewReader(blob).Read, rateLimitBPS, "Test_discard_reader_shared_key_1"),
		}
	}

	speed = (rateLimitBPS - 128*1024) / count
	totalN, spend = multiRead(count, readMaker, speed, totalSize/count)

	now = time.Now()
	eta = time.Second * time.Duration(totalN) / time.Duration(speed*count) // 期望以低速为准

	fmt.Println("total", totalN, "spend", spend, "eta", eta, "eta-spend:", eta-spend)
	assert.WithinDuration(t, now.Add(eta), now.Add(spend), eta/time.Duration(10), "偏差不超过10%")

	// 高速
	readMaker = func() *testReader {
		return &testReader{
			Read: DiscardLimit(bytes.NewReader(blob).Read, rateLimitBPS, "Test_discard_reader_shared_key_2"),
		}
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
	var writeMaker func() *testWriter
	var speed int

	totalSize := 1024 * 1024 * 2
	rateLimitBPS := 512 * 1024 // bps
	count := 3

	//低速通过
	writeMaker = func() *testWriter {
		return &testWriter{
			Write: SleepLimit(bytes.NewBuffer(nil).Write, rateLimitBPS, "Test_discard_writer_single_shared_key_1"),
		}
	}

	speed = (rateLimitBPS - 128*1024) / count
	totalN, spend = multiWrite(count, writeMaker, speed, totalSize/count)

	now = time.Now()
	eta = time.Second * time.Duration(totalN) / time.Duration(speed*count) // 期望以低速为准

	fmt.Println("total", totalN, "spend", spend, "eta", eta, "eta-spend:", eta-spend)
	assert.WithinDuration(t, now.Add(eta), now.Add(spend), eta/time.Duration(10), "偏差不超过10%")

	// 高速丢弃
	writeMaker = func() *testWriter {
		return &testWriter{
			Write: SleepLimit(bytes.NewBuffer(nil).Write, rateLimitBPS, "Test_discard_writer_single_shared_key_2"),
		}
	}

	speed = (rateLimitBPS + 128*1024) / count
	totalN, spend = multiWrite(count, writeMaker, speed, totalSize)

	now = time.Now()
	eta = time.Second * time.Duration(totalN) / time.Duration(rateLimitBPS)

	fmt.Println("total", totalN, "spend", spend, "eta", eta, "eta-spend:", eta-spend)
	assert.WithinDuration(t, now.Add(eta), now.Add(spend), eta/time.Duration(10), "偏差不超过10%")
}

func read(rd *testReader, speed int, totalSize int) (int, time.Duration) {
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

func multiRead(count int, readMaker func() *testReader, speed int, totalSize int) (int, time.Duration) {
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

func write(w *testWriter, speed int, blob []byte) (int, time.Duration) {
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

func multiWrite(count int, writeMaker func() *testWriter, speed int, totalSize int) (int, time.Duration) {
	wg := sync.WaitGroup{}

	wg.Add(count)
	blob := make([]byte, totalSize)

	totalN := 0
	begin := time.Now()
	for i := 0; i < count; i++ {
		go func() {
			w := writeMaker()
			// 确保原子操作
			n, _ := write(w, speed, blob)
			totalN += n
			wg.Done()
		}()
	}
	wg.Wait()

	spend := time.Now().Sub(begin)
	return totalN, spend
}
