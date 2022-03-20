#### 设计
- 本次缓存设计参考LRU，使用的是双链表+头尾指针实现


#### 目前发现的问题
- 内存大小无法准确控制，因为从Set接收到的参数都是interface{}，使用unsafe.Sizeof(),会返回固定的结果16byte
- 必须调用Get,Exists,Keys才会触发检查过期时间并删除的操作，意味着缓存中，可能存在大量过期，但是未删除的数据
- Get,Set,Del,Exists,Keys都是O(n)，因为都需要遍历链表


#### 基准测试
基准测试主要用于横向对比
```
goos: darwin
goarch: amd64
pkg: cache/v1
cpu: Intel(R) Core(TM) i7-9750H CPU @ 2.60GHz
BenchmarkSetKB-12             	 4703823	       252.2 ns/op	      96 B/op	       2 allocs/op
BenchmarkSetMB-12             	   53576	     98611 ns/op	      95 B/op	       2 allocs/op
BenchmarkSetGB-12             	   56648	    117430 ns/op	      95 B/op	       2 allocs/op
BenchmarkSetOutofMemory-12    	   27975	     42062 ns/op	      95 B/op	       2 allocs/op
BenchmarkGet-12               	   27194	    110498 ns/op	       4 B/op	       0 allocs/op
BenchmarkDel-12               	29181471	        43.07 ns/op	       7 B/op	       0 allocs/op
BenchmarkExists-12            	  475306	      2549 ns/op	       7 B/op	       0 allocs/op
BenchmarkKeys-12              	   16089	     76908 ns/op	       0 B/op	       0 allocs/op
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