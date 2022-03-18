package v1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	assert := assert.New(t)
	cache := NewLRUCache()
	assert.Equal(cache.maxMemory, LRUDefaultMemory)
	assert.Equal(cache.elemCount, 0)
	assert.Equal(cache.elemSize, 0)
	assert.Nil(cache.head)
	assert.Nil(cache.tail)
}

func TestSetMaxMemory(t *testing.T) {
	assert := assert.New(t)
	cache := NewLRUCache()
	table := []struct {
		sizeStr string
		panic   bool
		size    int
	}{
		{"1KB", false, 1 * UnitKB},
		{"10KB", false, 10 * UnitKB},
		{"100KB", false, 100 * UnitKB},
		{"1MB", false, 1 * UnitMB},
		{"10MB", false, 10 * UnitMB},
		{"100MB", false, 100 * UnitMB},
		{"1GB", false, 1 * UnitGB},
		{"01GB", false, 1 * UnitGB},
		{"5GB", true, 0},
		{"1.1GB", true, 0},
		{"-1GB", true, 0},
		{"0.01GB", true, 0},
		{"GB", true, 0},
		{"ABB", true, 0},
		{"GB1", true, 0},
		{"啊啊", true, 0},
		{"123321", true, 0},
	}
	for _, v := range table {
		if v.panic {
			fc := func() {
				cache.SetMaxMemory(v.sizeStr)
			}
			assert.Panics(fc, v.sizeStr)
		} else {
			cache.SetMaxMemory(v.sizeStr)
			assert.Equal(cache.maxMemory, v.size)
		}
	}
}
