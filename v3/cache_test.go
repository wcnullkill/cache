package v3

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	assert := assert.New(t)
	cache := NewLRUCache()
	assert.Equal(cache.maxMemory, LRUDefaultMemory)
	assert.Equal(0, cache.elemCount)
	assert.Equal(0, cache.elemSize)
	assert.Nil(cache.head)
	assert.Nil(cache.tail)
	assert.Equal(int64(DefaultAutoGCPeriod), cache.gcPeriod)
}

// 测试设置最大内存
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
			assert.Equal(v.size, cache.maxMemory, v.sizeStr)
		}
	}
}

// 测试各种数据类型的Set与Get
func TestSetGet(t *testing.T) {
	type user struct {
		Name string
		Age  int
	}
	assert := assert.New(t)
	cache := NewLRUCache()
	table := []struct {
		key    string
		val    interface{}
		expire time.Duration
	}{
		{"a", 1, time.Second * 100},
		{"a1", 2, time.Second * 100},
		{"a2", 4, time.Second * 100},
		{"a3", -8, time.Second * 100},
		{"a4", "16", time.Second * 100},
		{"a5", "32", time.Second * 100},
		{"a6", 3.14, time.Second * 100},
		{"a7", -3.14, time.Second * 100},
		{"a8", true, time.Second * 100},
		{"a9", false, time.Second * 100},
		{"a10", 'a', time.Second * 100},
		{"a11", '\r', time.Second * 100},
		{"a12", "啊啊啊", time.Second * 100},
		{"a13", []int{1, 2, 3}, time.Second * 100},
		{"a14", []string{"1", "2", "3"}, time.Second * 100},
		{"a15", map[string]int{"ab": 1, "bed": 2}, time.Second * 100},
		{"a16", map[string]string{"ab": "aaaa", "bed": "asdff"}, time.Second * 100},
		{"a17", user{"wc", 88}, time.Second * 100},
		{"a18", &user{"wc", 88}, time.Second * 100},
	}
	for _, v := range table {
		cache.Set(v.key, v.val, v.expire)
	}
	for _, v := range table {
		val, ok := cache.Get(v.key)
		assert.Equal(v.val, val, v.key)
		assert.True(ok)
	}
}

// 测试Exists
func TestSetExists(t *testing.T) {
	type user struct {
		Name string
		Age  int
	}
	assert := assert.New(t)
	cache := NewLRUCache()
	table := []struct {
		key    string
		val    interface{}
		expire time.Duration
	}{
		{"a", 1, time.Second * 100},
		{"a1", 2, time.Second * 100},
		{"a2", 4, time.Second * 100},
		{"a3", -8, time.Second * 100},
		{"a4", "16", time.Second * 100},
		{"a5", "32", time.Second * 100},
		{"a6", 3.14, time.Second * 100},
		{"a7", -3.14, time.Second * 100},
		{"a8", true, time.Second * 100},
		{"a9", false, time.Second * 100},
		{"a10", 'a', time.Second * 100},
		{"a11", '\r', time.Second * 100},
		{"a12", "啊啊啊", time.Second * 100},
		{"a13", []int{1, 2, 3}, time.Second * 100},
		{"a14", []string{"1", "2", "3"}, time.Second * 100},
		{"a15", map[string]int{"ab": 1, "bed": 2}, time.Second * 100},
		{"a16", map[string]string{"ab": "aaaa", "bed": "asdff"}, time.Second * 100},
		{"a17", user{"wc", 88}, time.Second * 100},
		{"a18", &user{"wc", 88}, time.Second * 100},
	}
	for _, v := range table {
		cache.Set(v.key, v.val, v.expire)
	}
	for _, v := range table {
		ok := cache.Exists(v.key)
		assert.True(ok)
	}
}

// 测试重复Set，值将被覆盖
func TestSet(t *testing.T) {
	assert := assert.New(t)
	cache := NewLRUCache()
	table := []struct {
		key    string
		val    interface{}
		actual interface{}
	}{
		{"a", 1, 1},
		{"a", 1, 1},
		{"a", 2, 2},
		{"a", 3, 3},
	}
	for _, v := range table {
		cache.Set(v.key, v.val, 0)
		val, ok := cache.Get(v.key)
		assert.True(ok)
		assert.Equal(v.actual, val)
	}
}

// 测试超过最大内存时
func TestSetOutofMaxMemory(t *testing.T) {
	assert := assert.New(t)
	cache := NewLRUCache()
	cache.SetMaxMemory("32KB")
	count := 1 << 10
	// key算16byte
	// 目前val算16byte
	for i := 0; i < count; i++ {
		key := strconv.Itoa(i + 1)
		cache.Set(key, i+1, 0)
	}
	assert.Equal(32*count, cache.elemSize)
	assert.Equal(int64(count), cache.Keys())
	for i := 0; i < count; i++ {
		key := strconv.Itoa(i + 1 + count)
		cache.Set(key, i+1+count, 0)
		keys := cache.Keys()
		assert.Equal(int64(count), keys)
		key = strconv.Itoa(i + 1)
		ok := cache.Exists(key)
		assert.False(ok)
	}
}

// 测试Expire和Exists
func TestExpireExists(t *testing.T) {
	assert := assert.New(t)
	cache := NewLRUCache()
	table := []struct {
		key    string
		val    interface{}
		expire time.Duration
		exists bool
	}{
		{"a", 1, time.Second * 1, false},
		{"a1", 2, time.Second * 10, true},
		{"a3", -8, time.Second * 0, true},
	}
	for _, v := range table {
		cache.Set(v.key, v.val, v.expire)
	}
	time.Sleep(time.Second * 3)
	for _, v := range table {
		ok := cache.Exists(v.key)
		assert.Equal(v.exists, ok, v.key)
	}
}

// 测试Expire和Get
func TestExpireGet(t *testing.T) {
	assert := assert.New(t)
	cache := NewLRUCache()
	table := []struct {
		key    string
		val    interface{}
		expire time.Duration
		exists bool
		actual interface{}
	}{
		{"a", 1, time.Second * 1, false, 0},
		{"a1", 2, time.Second * 10, true, 2},
		{"a3", -8, time.Second * 0, true, -8},
	}
	for _, v := range table {
		cache.Set(v.key, v.val, v.expire)
	}
	time.Sleep(time.Second * 3)
	for _, v := range table {
		val, ok := cache.Get(v.key)
		assert.Equal(v.exists, ok, v.key)
		if ok {
			assert.Equal(v.actual, val, v.key)
		}
	}
}

// 测试Expire和Keys
func TestExpireKeys(t *testing.T) {
	assert := assert.New(t)
	cache := NewLRUCache()
	table := []struct {
		key    string
		val    interface{}
		expire time.Duration
		exists bool
	}{
		{"a", 1, time.Second * 1, false},
		{"a1", 2, time.Second * 3, true},
		{"a3", -8, time.Second * 0, true},
	}
	for _, v := range table {
		cache.Set(v.key, v.val, v.expire)
	}
	keys := cache.Keys()
	assert.Equal(int64(3), keys)
	time.Sleep(time.Second * 2)
	keys = cache.Keys()
	assert.Equal(int64(3), keys)
	time.Sleep(time.Second * 2)
	keys = cache.Keys()
	assert.Equal(int64(3), keys)
}

// 测试从链表尾删除
func TestDel1(t *testing.T) {
	type user struct {
		Name string
		Age  int
	}
	assert := assert.New(t)
	cache := NewLRUCache()
	table := []struct {
		key    string
		val    interface{}
		expire time.Duration
	}{
		{"a", 1, time.Second * 100},
		{"a1", 2, time.Second * 100},
		{"a2", 4, time.Second * 100},
		{"a3", -8, time.Second * 100},
		{"a4", "16", time.Second * 100},
		{"a5", "32", time.Second * 100},
		{"a6", 3.14, time.Second * 100},
		{"a7", -3.14, time.Second * 100},
		{"a8", true, time.Second * 100},
		{"a9", false, time.Second * 100},
		{"a10", 'a', time.Second * 100},
		{"a11", '\r', time.Second * 100},
		{"a12", "啊啊啊", time.Second * 100},
		{"a13", []int{1, 2, 3}, time.Second * 100},
		{"a14", []string{"1", "2", "3"}, time.Second * 100},
		{"a15", map[string]int{"ab": 1, "bed": 2}, time.Second * 100},
		{"a16", map[string]string{"ab": "aaaa", "bed": "asdff"}, time.Second * 100},
		{"a17", user{"wc", 88}, time.Second * 100},
		{"a18", &user{"wc", 88}, time.Second * 100},
	}
	for _, v := range table {
		cache.Set(v.key, v.val, v.expire)
	}
	count := int64(len(table))
	assert.Equal(cache.Keys(), count)
	for _, v := range table {
		ok := cache.Del(v.key)
		count--
		assert.True(ok, v.key)
		ok = cache.Exists(v.key)
		assert.False(ok, v.key)
		keys := cache.Keys()
		assert.Equal(count, keys)
	}
}

// 测试从链表头删除
func TestDel2(t *testing.T) {
	type user struct {
		Name string
		Age  int
	}
	assert := assert.New(t)
	cache := NewLRUCache()
	table := []struct {
		key    string
		val    interface{}
		expire time.Duration
	}{
		{"a", 1, time.Second * 100},
		{"a1", 2, time.Second * 100},
		{"a2", 4, time.Second * 100},
		{"a3", -8, time.Second * 100},
		{"a4", "16", time.Second * 100},
		{"a5", "32", time.Second * 100},
		{"a6", 3.14, time.Second * 100},
		{"a7", -3.14, time.Second * 100},
		{"a8", true, time.Second * 100},
		{"a9", false, time.Second * 100},
		{"a10", 'a', time.Second * 100},
		{"a11", '\r', time.Second * 100},
		{"a12", "啊啊啊", time.Second * 100},
		{"a13", []int{1, 2, 3}, time.Second * 100},
		{"a14", []string{"1", "2", "3"}, time.Second * 100},
		{"a15", map[string]int{"ab": 1, "bed": 2}, time.Second * 100},
		{"a16", map[string]string{"ab": "aaaa", "bed": "asdff"}, time.Second * 100},
		{"a17", user{"wc", 88}, time.Second * 100},
		{"a18", &user{"wc", 88}, time.Second * 100},
	}
	for _, v := range table {
		cache.Set(v.key, v.val, v.expire)
	}
	count := int64(len(table))
	assert.Equal(cache.Keys(), count)
	for i := range table {
		ok := cache.Del(table[len(table)-i-1].key)
		count--
		assert.True(ok, table[len(table)-i-1].key)
		ok = cache.Exists(table[len(table)-i-1].key)
		assert.False(ok, table[len(table)-i-1].key)
		keys := cache.Keys()
		assert.Equal(count, keys)
	}
	assert.Equal(int64(0), cache.Keys())
}

// 测试随机Del
func TestDel3(t *testing.T) {
	type user struct {
		Name string
		Age  int
	}
	assert := assert.New(t)
	cache := NewLRUCache()
	table := []struct {
		key    string
		val    interface{}
		expire time.Duration
	}{
		{"a", 1, time.Second * 100},
		{"a1", 2, time.Second * 100},
		{"a2", 4, time.Second * 100},
		{"a3", -8, time.Second * 100},
		{"a4", "16", time.Second * 100},
		{"a5", "32", time.Second * 100},
		{"a6", 3.14, time.Second * 100},
		{"a7", -3.14, time.Second * 100},
		{"a8", true, time.Second * 100},
		{"a9", false, time.Second * 100},
		{"a10", 'a', time.Second * 100},
		{"a11", '\r', time.Second * 100},
		{"a12", "啊啊啊", time.Second * 100},
		{"a13", []int{1, 2, 3}, time.Second * 100},
		{"a14", []string{"1", "2", "3"}, time.Second * 100},
		{"a15", map[string]int{"ab": 1, "bed": 2}, time.Second * 100},
		{"a16", map[string]string{"ab": "aaaa", "bed": "asdff"}, time.Second * 100},
		{"a17", user{"wc", 88}, time.Second * 100},
		{"a18", &user{"wc", 88}, time.Second * 100},
	}
	// 随机100次
	for i := 0; i < 100; i++ {
		m := make(map[string]struct{}, len(table))
		for _, v := range table {
			cache.Set(v.key, v.val, v.expire)
			m[v.key] = struct{}{}
		}
		count := int64(len(table))
		assert.Equal(cache.Keys(), count)
		for k := range m {
			ok := cache.Del(k)
			count--
			assert.True(ok, k)
			ok = cache.Exists(k)
			assert.False(ok, k)
			keys := cache.Keys()
			assert.Equal(count, keys)
		}
		assert.Equal(int64(0), cache.Keys())
	}
}

func TestFlush(t *testing.T) {
	assert := assert.New(t)
	cache := NewLRUCache()
	table := []struct {
		key string
		val interface{}
	}{
		{"a1", 1},
		{"a2", 2},
		{"a3", 3},
	}
	for _, v := range table {
		cache.Set(v.key, v.val, 0)
	}
	assert.Equal(int64(len(table)), cache.Keys())
	cache.Flush()
	assert.Equal(int64(0), cache.Keys())
	assert.Equal(0, cache.elemSize)
	assert.Equal(0, cache.elemCount)
	assert.Equal(0, len(cache.m))
	assert.Nil(cache.head)
	assert.Nil(cache.tail)
}

func TestSetGCPeriod(t *testing.T) {
	assert := assert.New(t)
	cache := NewLRUCache()
	cache.SetGCPeriod(time.Second * 120)
	assert.Equal(int64(time.Second*120), cache.gcPeriod)
}

// 测试gc，且gc后元素都有效
func TestGC(t *testing.T) {
	assert := assert.New(t)
	cache := NewLRUCache()
	t1 := cache.gcTime
	cache.SetMaxMemory("32KB")
	count := 1 << 10
	// key算16byte
	// 目前val算16byte
	for i := 0; i < count; i++ {
		key := strconv.Itoa(i + 1)
		cache.Set(key, i+1, 0)
	}
	assert.Equal(32*1<<10, cache.elemSize)
	// 此时内存使用率为1，且gcState=0，但是时间<自动gc时间。未达到gc条件
	// gctime不变
	assert.Equal(t1, cache.gcTime)
	cache.SetGCPeriod(time.Second * 2)
	time.Sleep(time.Second * 3)
	// 休眠时间，触发了gc，gctime变化
	assert.Greater(cache.gcTime, t1)
	assert.Equal(int64(1<<10), cache.Keys())
}

// 测试gc，触发gc时，元素都已失效
func TestGC1(t *testing.T) {
	assert := assert.New(t)
	cache := NewLRUCache()
	t1 := cache.gcTime
	cache.SetMaxMemory("32KB")
	count := 1 << 10
	// key算16byte
	// 目前val算16byte
	for i := 0; i < count; i++ {
		key := strconv.Itoa(i + 1)
		cache.Set(key, i+1, time.Second)
	}
	assert.Equal(32*1<<10, cache.elemSize)
	// 此时内存使用率为1，且gcState=0，但是时间<自动gc时间。未达到gc条件
	// gctime不变
	assert.Equal(t1, cache.gcTime)
	cache.SetGCPeriod(time.Second * 2)
	time.Sleep(time.Second * 3)
	// 休眠时间，触发了gc，gctime变化
	assert.Greater(cache.gcTime, t1)
	// 所有元素都失效，被删除
	assert.Equal(int64(0), cache.Keys())
	assert.Equal(0, cache.elemSize)
	assert.Equal(0, cache.elemCount)
	assert.Equal(0, len(cache.m))
	assert.Nil(cache.head)
	assert.Nil(cache.tail)
}

// 测试gc，触发gc时，一半元素都已失效
func TestGC2(t *testing.T) {
	assert := assert.New(t)
	cache := NewLRUCache()
	t1 := cache.gcTime
	cache.SetMaxMemory("32KB")
	count := 1 << 10
	// key算16byte
	// 目前val算16byte
	for i := 0; i < count/2; i++ {
		key := strconv.Itoa(i + 1)
		cache.Set(key, i+1, time.Second)
	}
	for i := count / 2; i < count; i++ {
		key := strconv.Itoa(i + 1)
		cache.Set(key, i+1, 0)
	}
	assert.Equal(32*1<<10, cache.elemSize)
	// 此时内存使用率为1，且gcState=0，但是时间<自动gc时间。未达到gc条件
	// gctime不变
	assert.Equal(t1, cache.gcTime)
	cache.SetGCPeriod(time.Second * 2)
	time.Sleep(time.Second * 3)
	// 休眠时间，触发了gc，gctime变化
	assert.Greater(cache.gcTime, t1)
	// 一半元素都失效，被删除
	assert.Equal(int64(count/2), cache.Keys())
	assert.Equal(count/2*32, cache.elemSize)
	assert.Equal(count/2, cache.elemCount)
	assert.Equal(count/2, len(cache.m))
	assert.NotNil(cache.head)
	assert.NotNil(cache.tail)
}
func BenchmarkSetKB(b *testing.B) {
	cache := NewLRUCache()
	cache.SetMaxMemory("1KB")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Set(strconv.Itoa(i), i, 0)
	}

}
func BenchmarkSetMB(b *testing.B) {
	cache := NewLRUCache()
	cache.SetMaxMemory("1MB")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Set(strconv.Itoa(i), i, 0)
	}

}
func BenchmarkSetGB(b *testing.B) {
	cache := NewLRUCache()
	cache.SetMaxMemory("1GB")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Set(strconv.Itoa(i), i, 0)
	}

}
func BenchmarkSetOutofMemory(b *testing.B) {
	cache := NewLRUCache()
	cache.SetMaxMemory("320KB")
	for i := 0; i < 10<<10; i++ {
		cache.Set(strconv.Itoa(i), i, 0)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Set(strconv.Itoa(i), i, 0)
	}
}
func BenchmarkGet(b *testing.B) {
	cache := NewLRUCache()
	cache.SetMaxMemory("10MB")
	for i := 0; i < b.N; i++ {
		cache.Set(strconv.Itoa(i), i, 0)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(strconv.Itoa(i))
	}
}
func BenchmarkDel(b *testing.B) {
	cache := NewLRUCache()
	cache.SetMaxMemory("10MB")
	for i := 0; i < 1<<10; i++ {
		cache.Set(strconv.Itoa(i), i, 0)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Del(strconv.Itoa(i))
	}
}
func BenchmarkExists(b *testing.B) {
	cache := NewLRUCache()
	cache.SetMaxMemory("10MB")
	for i := 0; i < 1<<10; i++ {
		cache.Set(strconv.Itoa(i), i, 0)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Exists(strconv.Itoa(i))
	}
}
func BenchmarkKeys(b *testing.B) {
	cache := NewLRUCache()
	cache.SetMaxMemory("10MB")
	for i := 0; i < 1<<10; i++ {
		cache.Set(strconv.Itoa(i), i, 0)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Keys()
	}
}
