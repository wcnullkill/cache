package v4

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

const (
	UnitKB = 1 << 10
	UnitMB = 1 << 20
	UnitGB = 1 << 30

	MaxMemory           = 4 << 30   // 4GB
	MinMemory           = 1 << 10   //	1KB
	LRUDefaultMemory    = 100 << 20 // 100MB
	DefaultMapElem      = 100
	DefaultAutoGCPeriod = time.Second * 120
	MinGCPeriod         = time.Second * 1 // 最小gc间隔
)

var (
	LRUMaxTime, _ = time.Parse("2006-01-02 15:04:05", "2030-12-13 00:00:00")
)
var elemPool = sync.Pool{
	New: func() interface{} {
		return new(elem)
	},
}

type MemoryErr struct {
	size string
}

func (err *MemoryErr) Error() string {
	return fmt.Sprintf("错误的内存大小%s", err.size)
}

type ExpireErr struct {
	d time.Duration
}

func (err *ExpireErr) Error() string {
	return fmt.Sprintf("错误的有效期%d", err.d)
}

func getMemorySize(size string) (int, error) {
	bSize := []byte(size)
	if len(bSize) <= 2 {
		return 0, &MemoryErr{size: size}
	}
	sizeStr := string(bSize[:len(bSize)-2])
	unitStr := string(bSize[len(bSize)-2:])
	var mem int
	mem, err := strconv.Atoi(sizeStr)
	if err != nil || mem <= 0 {
		return 0, &MemoryErr{size: size}
	}
	// 单位
	switch strings.ToUpper(unitStr) {
	case "KB":
		mem = mem * UnitKB
	case "MB":
		mem = mem * UnitMB
	case "GB":
		mem = mem * UnitGB
	default:
		return 0, &MemoryErr{size: size}
	}
	if mem > MaxMemory || mem < MinMemory {
		return 0, &MemoryErr{size: size}
	}
	return mem, nil
}

// 由于类型都同意转换为interface{}，这里固定返回16byte
func sizeof(c interface{}) int {
	//
	// unsafe.Sizeof()返回的值介绍
	//
	// string类型，固定24byte
	// 数值类型，如int64等，与真实内存大小一致
	// struct值类型，与其内部元素有关
	// struct引用类型，固定8byte
	// map类型，实际上也是引用类型，固定8byte
	// 数组类型，与T具体类型和长度有关
	// slice类型，固定24byte

	return int(unsafe.Sizeof(c))
}

type Cache interface {
	SetMaxMemory(size string)
	Set(key string, val interface{}, expire time.Duration)
	Get(key string) (interface{}, bool)
	Del(key string) bool
	Exists(key string) bool
	Flush() bool
	Keys() int64
}

type lruCache struct {
	elemCount int //元素个数
	elemSize  int //元素占用内存大小
	maxMemory int //最大内存大小
	head      *elem
	tail      *elem
	l         sync.Mutex
	// 使用map存储key val，提高查询效率
	m        map[string]*elem
	gcState  int64 //是否处于gc状态
	gcTime   int64 //上次gc的unix时间
	gcPeriod int64 // 自动gc周期
}

func NewLRUCache() *lruCache {
	cache := &lruCache{
		maxMemory: LRUDefaultMemory,
		l:         sync.Mutex{},
		m:         make(map[string]*elem),
		gcTime:    time.Now().UnixNano(),
		gcPeriod:  int64(DefaultAutoGCPeriod),
	}
	go func() {
		for {
			cache.l.Lock()
			cache.gc()
			cache.l.Unlock()
			time.Sleep(time.Second * 1)
		}
	}()
	return cache
}

// 最大4GB，最小1KB
func (c *lruCache) SetMaxMemory(size string) {
	c.l.Lock()
	defer c.l.Unlock()
	siz, err := getMemorySize(size)
	if err != nil {
		panic(err)
	}
	c.maxMemory = siz
}
func (c *lruCache) SetGCPeriod(d time.Duration) {
	c.gcPeriod = int64(d)
}
func (c *lruCache) Set(key string, val interface{}, expire time.Duration) {
	if expire < 0 {
		panic(&ExpireErr{expire})
	}
	size := sizeof(key) + sizeof(val)
	c.l.Lock()
	defer c.l.Unlock()
	// 如果存在，直接修改
	v, ok := c.get(key)
	if ok {
		// 目前大小不变
		v.setVal(key, val)
		v.setExpire(expire)
		c.lpush(v)
		return
	}
	// 如果当前内存占用率大于1,则触发gc
	if c.elemSize+size > c.maxMemory {
		c.gc()
	}
	// 可能gc后，内存占用率仍高于1
	for c.elemSize+size > c.maxMemory {
		e := c.rpop()
		c.elemCount--
		c.elemSize -= e.size
		e.reset()
		elemPool.Put(e)
	}
	v1 := elemPool.Get().(*elem)
	v1.setVal(key, val)
	v1.setExpire(expire)
	c.lpush(v1)

	c.elemCount++
	c.elemSize += v1.size
}

func (c *lruCache) Get(key string) (interface{}, bool) {
	c.l.Lock()
	defer c.l.Unlock()
	val, ok := c.get(key)
	if !ok {
		return nil, false
	}
	if !val.alive() { // 过了有效期，删除并返回不存在
		c.elemCount--
		c.elemSize -= val.size
		c.del(val)
		val.reset()
		elemPool.Put(val)
		return nil, false
	}
	c.moveToHead(val)
	return val.val, true
}
func (c *lruCache) Del(key string) bool {
	c.l.Lock()
	defer c.l.Unlock()
	val, ok := c.get(key)
	if !ok {
		return false
	}
	c.del(val)
	c.elemCount--
	c.elemSize -= val.size
	val.reset()
	elemPool.Put(val)
	return true
}

func (c *lruCache) Exists(key string) bool {
	c.l.Lock()
	defer c.l.Unlock()
	val, ok := c.get(key)
	if ok && !val.alive() {
		c.elemCount--
		c.elemSize -= val.size
		c.del(val)
		val.reset()
		elemPool.Put(val)
		return false
	}
	return ok
}
func (c *lruCache) Flush() bool {
	c.l.Lock()
	defer c.l.Unlock()
	c.head = nil
	c.tail = nil
	c.elemCount = 0
	c.elemSize = 0
	// 重制哈希表
	oldm := c.m
	c.m = make(map[string]*elem)
	for _, v := range oldm {
		v.reset()
		elemPool.Put(v)
	}
	return true
}

func (c *lruCache) Keys() int64 {
	c.l.Lock()
	defer c.l.Unlock()
	return int64(c.elemCount)
}

func (c *lruCache) get(key string) (*elem, bool) {
	// 通过遍历哈希表，查询元素
	e, ok := c.m[key]
	return e, ok
}

// 将e移到第一个
func (c *lruCache) moveToHead(e *elem) {
	c.del(e)
	c.lpush(e)
}

// 删除e,e必定是链表中的一个元素
func (c *lruCache) del(e *elem) {
	if e.next == nil && e.prev == nil { // 只有e一个元素
		c.head = nil
		c.tail = nil
	} else if e.next == nil { // e是最后一个元素
		prev := e.prev
		prev.next = nil
		c.tail = prev
	} else if e.prev == nil { // e是第一个元素
		next := e.next
		next.prev = nil
		c.head = next
	} else {
		prev := e.prev
		prev.next = e.next
		e.next.prev = prev
	}
	delete(c.m, e.key)
	e.free()
}

// 把元素插入第一个
func (c *lruCache) lpush(e *elem) {
	if c.head == nil { // 没有元素
		c.tail = e
	} else {
		e.next = c.head
		e.prev = nil
		c.head.prev = e
	}
	c.m[e.key] = e
	c.head = e
}

// 弹出最后一个elem
func (c *lruCache) rpop() *elem {
	if c.head == nil { // 如果没有元素，返回nil
		return nil
	}
	//如果只有一个元素，需要清空tail和head
	if c.head.next == nil && c.tail.prev == nil {
		e := c.head
		c.tail = nil
		c.head = nil
		delete(c.m, e.key)
		return e
	}
	tail := c.tail
	newTail := tail.prev
	newTail.next = nil
	c.tail = newTail
	delete(c.m, tail.key)
	tail.free()
	return tail
}

// 回收过期元素
// 触发条件,必须同时满足
//		1. 当前gcState==0
//		2. gc间隔>最小gc间隔
//		3. gc间隔过了gcPeriod秒,或者cache内存使用率>3/4
// 触发时机
//		1. cache创建后，自启动一个goroutine定时调用
//		2. 调用Set时，如果内存使用率达到1
func (c *lruCache) gc() {
	if c.testgc() && atomic.CompareAndSwapInt64(&c.gcState, 0, 1) {
		c.gcTime = time.Now().UnixNano()
		for _, v := range c.m {
			if !v.alive() {
				c.elemCount--
				c.elemSize -= v.size
				c.del(v)
				v.reset()
				elemPool.Put(v)
			}
		}
		atomic.StoreInt64(&c.gcState, 0)
	}
}
func (c *lruCache) testgc() bool {
	state := atomic.LoadInt64(&c.gcState)
	interval := time.Now().UnixNano() - c.gcTime
	result := state == 0 && interval > int64(MinGCPeriod) && (interval > int64(c.gcPeriod) || c.elemSize > c.maxMemory*3/4)

	return result
}

type elem struct {
	key    string
	val    interface{} //存储内容
	size   int         //元素大小
	expire time.Time
	next   *elem
	prev   *elem
}

func (e *elem) setExpire(d time.Duration) {
	if d == 0 {
		e.expire = LRUMaxTime
	} else if d > 0 {
		e.expire = time.Now().Add(d)
	} else {
		panic(&ExpireErr{d: d})
	}
}
func (e *elem) setVal(key string, val interface{}) {
	e.key = key
	e.val = val
	e.size = sizeof(val) + sizeof(key)
}
func (e *elem) alive() bool {
	return e.expire.Unix() > time.Now().Unix()
}

// 清空前后关系
func (e *elem) free() {
	if e != nil {
		e.next = nil
		e.prev = nil
	}
}

// 全部清空
func (e *elem) reset() {
	if e != nil {
		e.next = nil
		e.prev = nil
		// e.expire = time.Now()
		e.key = ""
		e.size = 0
		e.val = nil

	}
}
