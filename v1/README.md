#### 设计
- 本次缓存设计参考LRU，使用的是双链表+头尾指针实现


#### 目前发现的问题
- 内存大小无法准确控制，因为从Set接收到的参数都是interface{}，使用unsafe.Sizeof(),会返回固定的结果16byte
- 必须调用Get,Exists,Keys才会触发检查过期时间并删除的操作，意味着缓存中，可能存在大量过期，但是未删除的数据
- Get,Set,Del,Exists,Keys都是O(n)，因为都需要遍历链表


#### 基准测试
```
goos: darwin
goarch: amd64
pkg: cache/v1
cpu: Intel(R) Core(TM) i7-9750H CPU @ 2.60GHz
BenchmarkSet-12          	   56389	    127561 ns/op	      95 B/op	       2 allocs/op
BenchmarkSetWhenKB-12    	   47462	    110733 ns/op	      95 B/op	       2 allocs/op
BenchmarkGet-12          	   26947	    111419 ns/op	       4 B/op	       0 allocs/op
BenchmarkDel-12          	   54756	    124129 ns/op	       5 B/op	       0 allocs/op
BenchmarkExists-12       	   44218	     90732 ns/op	       5 B/op	       0 allocs/op
BenchmarkKeys-12         	   10000	    740076 ns/op	       0 B/op	       0 allocs/op
```
#### 笔记
1. interface{}，可能会引起内存逃逸，因为其结构如下，固定占用16byte
```golang
    type eface struct{
        _type *_type    
        data unsafe.Pointer
    }
```
2. 上述interface中,_type结构如下
```golang
    type _type struct{
        size    uintptr     // 占用空间大小
        hash    uint32      // 快速确定类型是否相等
        equal   func(unsafe.Pointer,unsafe.Pointer)bool // 当前类型的多个对象是否相等
    }
```
3. unsafe.Sizeof返回结果，可能也跟环境有关
    - string类型，固定24byte
    - 数值类型，如int64等，与真实内存大小一致
    - struct值类型，与其内部元素有关
    - struct引用类型，固定8byte
    - map类型，实际上也是引用类型，固定8byte
    - 数组类型，与T具体类型和长度有关
    - slice类型，固定24byte