package rand

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNextNearestPow2uint64(t *testing.T) {
	assert.Equal(t, uint64(2), nextNearestPow2uint64(2), "should be 2")
	assert.Equal(t, uint64(4), nextNearestPow2uint64(3), "should be 4")
	assert.Equal(t, uint64(8), nextNearestPow2uint64(5), "should be 8")
	assert.Equal(t, uint64(16), nextNearestPow2uint64(10), "should be 16")
}

func TestPow2FastModulus(t *testing.T) {
	scenarios := []struct {
		number   int
		divisor  int
		expected int
	}{
		{
			number:   13,
			divisor:  4,
			expected: 1,
		},
		{
			number:   13,
			divisor:  8,
			expected: 5,
		},
		{
			number:   16,
			divisor:  8,
			expected: 0,
		},
	}
	for _, sc := range scenarios {
		ans := sc.number & (sc.divisor - 1)
		assert.Equal(t, sc.expected, ans, "should be equal")
	}
}

func TestNewPool(t *testing.T) {
	pool := NewPool(1, 3)
	assert.Equal(t, 4, len(pool.sources), "rand pool should have size of 4")
}
