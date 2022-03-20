#### 设计
- 本次缓存设计参考LRU，使用的是双链表+头尾指针+哈希表实现
- 使用NewLRUCache时，自动开启后台线程，定时触发gc，当同时满足以下条件时，执行回收
	1. 当前gcState==0
	2. 离上一次gc过了gcPeriod秒
	3. cache内存使用率>3/4
- 本版本中Keys()直接返回elemSize，可能包含过期失效的
- 目前Get，Set，Del，Exists，Keys都是O(1)
- 内存实际占用，约为原本数据的两倍
- 当调用Set，优先触发gc，如果内存使用率依然是1，再rpop弹出元素


#### 目前发现的问题
- 内存大小无法准确控制，因为从Set接收到的参数都是interface{}，使用unsafe.Sizeof(),会返回固定的结果16byte
- ~~必须调用Get,Exists,Keys才会触发检查过期时间并删除的操作，意味着缓存中，可能存在大量过期，但是未删除的数据~~
- ~~Get,Set,Del,Exists,Keys都是O(n)，因为都需要遍历链表~~
- ~~目前Get，Set，Del，Exists都是O(1),Keys是O(n)，但是内存占用比上一版高出一倍~~
- ~~Set中，当超过了maxMemory时，只使用rpop这种方式，可能会弹出有效的元素，但是数据库中还保留无效过期的垃圾元素~~
- 由于目前存储的全是interface引用类型,如果用户get以后不释放，那么go的垃圾回收器也不会释放，可能导致cache内存使用率不高，但是程序的实际内存占用非常高
- val应该是被分配在堆上，垃圾回收器没有及时回收

#### 基准测试
基准测试主要用于横向对比
```
goos: darwin
goarch: amd64
pkg: cache/v3
cpu: Intel(R) Core(TM) i7-9750H CPU @ 2.60GHz
BenchmarkSetKB-12             	 3881370	       321.3 ns/op	      98 B/op	       3 allocs/op
BenchmarkSetMB-12             	 3118718	       415.4 ns/op	      97 B/op	       3 allocs/op
BenchmarkSetGB-12             	 1984275	       678.4 ns/op	     220 B/op	       3 allocs/op
BenchmarkSetOutofMemory-12    	 3397266	       380.4 ns/op	      99 B/op	       3 allocs/op
BenchmarkGet-12               	 4739529	       260.4 ns/op	       7 B/op	       0 allocs/op
BenchmarkDel-12               	29588482	        45.69 ns/op	       7 B/op	       0 allocs/op
BenchmarkExists-12            	20917980	        52.65 ns/op	       7 B/op	       0 allocs/op
BenchmarkKeys-12              	90007804	        13.74 ns/op	       0 B/op	       0 allocs/op
```
