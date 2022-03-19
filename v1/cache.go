package v1

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"
)

const (
	UnitKB = 1 << 10
	UnitMB = 1 << 20
	UnitGB = 1 << 30

	MaxMemory        = 4 << 30   // 4GB
	MinMemory        = 1 << 10   //	1KB
	LRUDefaultMemory = 100 << 20 // 100MB
)

var (
	LRUMaxTime, _ = time.Parse("2006-01-02 15:04:05", "2030-12-13 00:00:00")
)

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

type LRUCache struct {
	elemCount int //元素个数
	elemSize  int //元素占用内存大小
	maxMemory int //最大内存大小
	head      *elem
	tail      *elem
	l         sync.Mutex
}

func NewLRUCache() *LRUCache {
	return &LRUCache{
		maxMemory: LRUDefaultMemory,
		l:         sync.Mutex{},
	}
}

// 最大4GB，最小1KB
func (c *LRUCache) SetMaxMemory(size string) {
	c.l.Lock()
	defer c.l.Unlock()
	siz, err := getMemorySize(size)
	if err != nil {
		panic(err)
	}
	c.maxMemory = siz
}
func (c *LRUCache) Set(key string, val interface{}, expire time.Duration) {
	if expire < 0 {
		panic(&ExpireErr{expire})
	}
	size := sizeof(key) + sizeof(val)
	c.l.Lock()
	defer c.l.Unlock()
	// 如果存在，先删除，再新增
	v, ok := c.get(key)
	if ok {
		c.del(v)
		c.elemCount--
		c.elemSize -= v.size
	}

	for c.elemSize+size > c.maxMemory {
		e := c.rpop()
		c.elemCount--
		c.elemSize -= e.size
		e.free()
	}

	e := new(elem)
	e.setVal(key, val)
	e.setExpire(expire)
	c.lpush(e)

	c.elemCount++
	c.elemSize += e.size
}

func (c *LRUCache) Get(key string) (interface{}, bool) {
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
		return nil, false
	}
	c.moveToHead(val)
	return val.val, true
}
func (c *LRUCache) Del(key string) bool {
	c.l.Lock()
	defer c.l.Unlock()
	val, ok := c.get(key)
	if !ok {
		return false
	}
	c.del(val)
	c.elemCount--
	c.elemSize -= val.size
	return true
}

func (c *LRUCache) Exists(key string) bool {
	c.l.Lock()
	defer c.l.Unlock()
	val, ok := c.get(key)
	if ok && !val.alive() {
		c.elemCount--
		c.elemSize -= val.size
		c.del(val)
		return false
	}
	return ok
}
func (c *LRUCache) Flush() bool {
	c.l.Lock()
	defer c.l.Unlock()
	c.head = nil
	c.tail = nil
	c.elemCount = 0
	c.elemSize = 0
	return true
}

func (c *LRUCache) Keys() int64 {
	c.l.Lock()
	defer c.l.Unlock()
	// 可能有过期元素，需要遍历一次
	var next *elem
	node := c.head
	for node != nil {
		next = node.next
		if !node.alive() {
			c.elemCount--
			c.elemSize -= node.size
			c.del(node)
		}
		node = next
	}
	return int64(c.elemCount)
}

func (c *LRUCache) get(key string) (*elem, bool) {
	// 遍历链表，查找元素
	node := c.head
	for node != nil {
		if node.key == key {
			return node, true
		}
		node = node.next
	}
	return nil, false
}

// 将e移到第一个
func (c *LRUCache) moveToHead(e *elem) {
	c.del(e)
	c.lpush(e)
}

// 删除e,e必定是链表中的一个元素
func (c *LRUCache) del(e *elem) {
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

	e.free()
}

// 把元素插入第一个
func (c *LRUCache) lpush(e *elem) {
	if c.head == nil { // 没有元素
		c.tail = e
	} else {
		e.next = c.head
		e.prev = nil
		c.head.prev = e
	}
	c.head = e
}

// 弹出最后一个elem
func (c *LRUCache) rpop() *elem {
	if c.head == nil { // 如果没有元素，返回nil
		return nil
	}
	//如果只有一个元素，需要清空tail和head
	if c.head.next == nil && c.tail.prev == nil {
		e := c.head
		c.tail = nil
		c.head = nil
		return e
	}
	tail := c.tail
	newTail := tail.prev
	newTail.next = nil
	c.tail = newTail
	return tail
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

func (e *elem) free() {
	if e != nil {
		e.next = nil
		e.prev = nil
	}
}
