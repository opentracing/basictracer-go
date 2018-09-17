package basictracer

import (
	gorand "math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/opentracing/basictracer-go/rand"
)

func TestRandomID(t *testing.T) {
	randompool = rand.NewPool(time.Now().UnixNano(), 10)
	uniques := 1000000
	ids := map[uint64]bool{}
	for i := 0; i < uniques; i++ {
		id := randomID()
		ids[id] = true
	}
	assert.Equal(t, uniques, len(ids), "should have no duplicates")
}

func TestRandomID2(t *testing.T) {
	randompool = rand.NewPool(time.Now().UnixNano(), 10)
	uniques := 1000000
	ids := map[uint64]bool{}
	for i := 0; i < uniques; i++ {
		id1, id2 := randomID2()
		ids[id1] = true
		ids[id2] = true
	}
	assert.Equal(t, uniques*2, len(ids), "should have no duplicates")
}

var (
	randomIDGen     *gorand.Rand
	randomIDGenOnce sync.Once
	randomIDGenLock sync.Mutex
)

// implementation using single random generator
func singleSourceRandomID() uint64 {
	// Golang does not seed the rng for us. Make sure it happens.
	randomIDGenOnce.Do(func() {
		randomIDGen = gorand.New(gorand.NewSource(time.Now().UnixNano()))
	})

	// The golang random generators are *not* intrinsically thread-safe.
	randomIDGenLock.Lock()
	defer randomIDGenLock.Unlock()
	return uint64(randomIDGen.Int63())
}

func BenchmarkSingleSourceGenSeededGUID(b *testing.B) {
	// run with 100000 goroutines
	b.SetParallelism(100000)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			singleSourceRandomID()
		}
	})
}

func BenchmarkGenSeededRandomID(b *testing.B) {
	// run with 100000 goroutines
	b.SetParallelism(100000)
	randompool = rand.NewPool(time.Now().UnixNano(), 16) // 16 random generators
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			randomID()
		}
	})
}
