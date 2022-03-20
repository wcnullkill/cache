#### 设计
- 本次缓存设计参考LRU，使用的是双链表+头尾指针+哈希表实现


#### 目前发现的问题
- 内存大小无法准确控制，因为从Set接收到的参数都是interface{}，使用unsafe.Sizeof(),会返回固定的结果16byte
- 必须调用Get,Exists,Keys才会触发检查过期时间并删除的操作，意味着缓存中，可能存在大量过期，但是未删除的数据
- ~~Get,Set,Del,Exists,Keys都是O(n)，因为都需要遍历链表~~
- 目前Get，Set，Del，Exists都是O(1),Keys是O(n)，但是内存占用比上一版高出一倍
- Set中，当超过了maxMemory时，只使用rpop这种方式，可能会弹出有效的元素，但是数据库中还保留无效过期的垃圾元素
#### 基准测试
基准测试主要用于横向对比
```
goos: darwin
goarch: amd64
pkg: cache/v2
cpu: Intel(R) Core(TM) i7-9750H CPU @ 2.60GHz
BenchmarkSetKB-12             	 4989520	       228.4 ns/op	      98 B/op	       3 allocs/op
BenchmarkSetMB-12             	 4245046	       284.6 ns/op	      98 B/op	       3 allocs/op
BenchmarkSetGB-12             	 2115027	       585.6 ns/op	     213 B/op	       3 allocs/op
BenchmarkSetOutofMemory-12    	 4237939	       293.9 ns/op	      99 B/op	       3 allocs/op
BenchmarkGet-12               	 4916308	       253.0 ns/op	       7 B/op	       0 allocs/op
BenchmarkDel-12               	24701490	        42.81 ns/op	       7 B/op	       0 allocs/op
BenchmarkExists-12            	23714551	        53.66 ns/op	       7 B/op	       0 allocs/op
BenchmarkKeys-12              	   13317	     90040 ns/op	       0 B/op	       0 allocs/op
```
