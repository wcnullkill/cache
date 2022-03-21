#### 设计
- 本次缓存设计参考LRU，使用的是双链表+头尾指针+哈希表实现
- 使用NewLRUCache时，自动开启后台线程，定时触发gc，当同时满足以下条件时
	1. 当前gcState==0
	2. gc间隔>最小gc间隔
	3. gc间隔过了gcPeriod秒,或者cache内存使用率>3/4
- 本版本中Keys()直接返回elemSize，可能包含过期失效的
- 目前Get，Set，Del，Exists，Keys都是O(1)
- 内存实际占用，约为原本数据的两倍
- 当调用Set，优先触发gc，如果内存使用率依然是1，再rpop弹出元素
- 定时刷新gc的线程，上锁，避免gc过程中，由Set再次触发gc
- 引入sync.Pool，复用elem对象，降低对象分配与垃圾回收，效果比较显著


#### 目前发现的问题
- 内存大小无法准确控制，因为从Set接收到的参数都是interface{}，使用unsafe.Sizeof(),会返回固定的结果16byte
- ~~必须调用Get,Exists,Keys才会触发检查过期时间并删除的操作，意味着缓存中，可能存在大量过期，但是未删除的数据~~
- ~~Get,Set,Del,Exists,Keys都是O(n)，因为都需要遍历链表~~
- ~~目前Get，Set，Del，Exists都是O(1),Keys是O(n)，但是内存占用比上一版高出一倍~~
- ~~Set中，当超过了maxMemory时，只使用rpop这种方式，可能会弹出有效的元素，但是数据库中还保留无效过期的垃圾元素~~
- ~~由于目前存储的全是interface引用类型,如果用户get以后不释放，那么go的垃圾回收器也不会释放，可能导致cache内存使用率不高，但是程序的实际内存占用非常高~~
- ~~val应该是被分配在堆上，垃圾回收器没有及时回收~~

#### 基准测试
基准测试主要用于横向对比
```
goos: darwin
goarch: amd64
pkg: cache/v4
cpu: Intel(R) Core(TM) i7-9750H CPU @ 2.60GHz
BenchmarkSetKB-12             	 4252358	       286.2 ns/op	      18 B/op	       2 allocs/op
BenchmarkSetMB-12             	 3714596	       365.1 ns/op	      18 B/op	       2 allocs/op
BenchmarkSetGB-12             	 1496504	       694.4 ns/op	     157 B/op	       2 allocs/op
BenchmarkSetOutofMemory-12    	 3719701	       313.4 ns/op	      19 B/op	       2 allocs/op
BenchmarkGet-12               	 4638534	       277.5 ns/op	       9 B/op	       0 allocs/op
BenchmarkDel-12               	27660736	        43.21 ns/op	       7 B/op	       0 allocs/op
BenchmarkExists-12            	24033320	        59.31 ns/op	       7 B/op	       0 allocs/op
BenchmarkKeys-12              	85832187	        13.67 ns/op	       0 B/op	       0 allocs/op
```
