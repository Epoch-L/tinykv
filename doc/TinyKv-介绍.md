# 前言

&emsp;&emsp;开一个新坑，将tinykv的4个project全部实现。虽然今天我点进去看的时候就萌生退意。好在没有放弃之前，把project1完成了，这让我跃跃欲试挑战后面的project。。。。。。


&emsp;&emsp;打算后续将每个project的解决思路写出来，下面介绍一下tinykv这个项目。我的地址是[git tinykv master](https://github.com/gopherWxf/tinykv/tree/master)，如果遇到不会的希望可以给读者提供一些思路。



# tinykv

&emsp;&emsp;[TinyKV](https://github.com/talent-plan/tinykv) 使用 Raft 共识算法构建 key-value 存储系统。它的灵感来自MIT 6.824和TiKV 项目。

&emsp;&emsp;完成本项目后，您将具备实施具有分布式事务支持的水平可扩展、高可用性、键值存储服务的知识。此外，您将对 TiKV 架构和实现有更好的了解。


# 架构

&emsp;&emsp;整个项目一开始就是一个key-value server和一个scheduler server的骨架代码——需要一步步完成核心逻辑：



* project1：Standalone KV     
  * 实现一个独立的存储引擎。
  * 实现原始键值服务处理程序。
* project2：Raft KV        
  * 实现基本的 Raft 算法。
  * 在 Raft 之上构建一个容错的 KV 服务器。
  * 增加对 Raft 日志垃圾回收和快照的支持。
* project3：Multi-raft KV       
  * 对 Raft 算法实施成员变更和领导层变更。
  * 在 Raft 存储上实现 conf 更改和区域拆分。
  * 实现一个基本的调度器。
* project4：Transaction       
  * 实现多版本并发控制层。
  * 实现KvGet、KvPrewrite和KvCommit请求的处理程序。
  * 实现KvScan、KvCheckTxnStatus、KvBatchRollback和KvResolveLock 请求的处理程序。



# 代码结构

![在这里插入图片描述](https://img-blog.csdnimg.cn/2f11b14daac64ee4a12baaff7dce74f8.png)



&emsp;&emsp;类似于 TiDB + TiKV + PD 的存储和计算分离的架构，`TinyKV 只关注分布式数据库系统的存储层`。如果您也对 SQL 层感兴趣，请参阅TinySQL。除此之外，还有一个名为 TinyScheduler 的组件作为整个 TinyKV 集群的中心控制，从 TinyKV 的心跳中收集信息。之后，TinyScheduler 可以生成调度任务并将任务分发给 TinyKV 实例。`所有实例都通过 RPC 进行通信。`


整个项目被组织到以下目录中：


- kv包含键值存储的实现。
- raft包含 Raft 共识算法的实现。
- scheduler包含 TinyScheduler 的实现，负责管理 TinyKV 节点和生成时间戳。
- proto包含节点和进程之间的所有通信的实现，使用基于 gRPC 的协议缓冲区。这个包包含了 TinyKV 使用的协议定义，以及生成的你可以使用的 Go 代码。
- log包含根据级别输出日志的实用程序。





# 如何去写


&emsp;&emsp;tinykv提供了非常多的单元测试，通过MakeFile来验证我们写的代码


![在这里插入图片描述](https://img-blog.csdnimg.cn/9c14a7b6a5f24c2982cc3a7844bae687.png)

```bash
git clone https://github.com/tidb-incubator/tinykv.git
cd tinykv
make

# 例:我们project1写好后 通过单元测试验证是否正确实现
make project1
```


&emsp;&emsp;那么我们如何去写呢？其实tinykv早已经准备好了，已project1举例。可以看到下面都有提示，我们需要通过分析源码，进行填空即可。


```go
type StandAloneStorage struct {
	// Your Data Here (1).
}

func NewStandAloneStorage(conf *config.Config) *StandAloneStorage {
	// Your Code Here (1).
}

func (s *StandAloneStorage) Start() error {
	// Your Code Here (1).
}

func (s *StandAloneStorage) Stop() error {
	// Your Code Here (1).
}

func (s *StandAloneStorage) Reader(ctx *kvrpcpb.Context) (storage.StorageReader, error) {
	// Your Code Here (1).
}

func (s *StandAloneStorage) Write(ctx *kvrpcpb.Context, batch []storage.Modify) error {
	// Your Code Here (1).
}
```


&emsp;&emsp;对于每个project，都会有一篇markdown文档进行简单的介绍，和一些提示。所以仅有的这几篇文档，要仔细阅读才行！
