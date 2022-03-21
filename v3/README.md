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
BenchmarkSetKB-12             	 3834207	       305.9 ns/op	      98 B/op	       3 allocs/op
BenchmarkSetMB-12             	 3360806	       378.8 ns/op	      98 B/op	       3 allocs/op
BenchmarkSetGB-12             	 2175087	       574.3 ns/op	     209 B/op	       3 allocs/op
BenchmarkSetOutofMemory-12    	 3364515	       356.2 ns/op	      99 B/op	       3 allocs/op
BenchmarkGet-12               	 5133748	       250.8 ns/op	       7 B/op	       0 allocs/op
BenchmarkDel-12               	25219101	        44.18 ns/op	       7 B/op	       0 allocs/op
BenchmarkExists-12            	23611804	        53.70 ns/op	       7 B/op	       0 allocs/op
BenchmarkKeys-12              	87736422	        13.77 ns/op	       0 B/op	       0 allocs/op
```
