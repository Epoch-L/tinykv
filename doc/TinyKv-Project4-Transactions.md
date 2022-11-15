# 前言

&emsp;&emsp;project4就是根据Percolator 2PC实现事务的快照隔离（SI）



# Project4 Transactions 文档翻译

 ## Project 4: Transactions

&emsp;&emsp;在之前的项目中，你已经建立了一个 key/value 数据库，通过使用 Raft，该数据库在多个节点上是一致的。要做到真正的可扩展，数据库必须能够处理多个客户端。有了多个客户端，就有了一个问题：如果两个客户端试图 "同时 "写同一个 key，会发生什么？如果一个客户写完后又立即读取该 key，他们是否应该期望读取的值与写入的值相同？在 Project 4 中，你将通过在我们的数据库中建立一个事务系统来解决这些问题。




&emsp;&emsp;该事务系统将是客户端（TinySQL）和服务器（TinyKV）之间的协作协议。为了确保事务属性，这两个部分必须正确实现。我们将有一个完整的事务请求的API，独立于你在 Project 1 中实现的原始请求（事实上，如果一个客户同时使用原始和事务API，我们不能保证事务属性）。



&emsp;&emsp;事务使用快照隔离（SI）。这意味着在一个事务中，客户端将从数据库中读取数据，就像它在事务开始时被冻结一样（事务看到的是数据库的一致视图）。一个事务要么全部写入数据库，要么不写入（如果它与另一个事务冲突）。



&emsp;&emsp;为了提供SI，你需要改变数据在后备存储中的存储方式。你不是为每个 key 存储一个值，而是为一个 key 和一个时间（用一个时间戳表示）存储一个值。这被称为多版本并发控制（MVCC），因为一个值的多个不同版本被存储在每个 key 上。



&emsp;&emsp;你将在 Part A 实现 MVCC，在 Part B & Part C你将实现事务性API。


## TinyKV中的事务


&emsp;&emsp;TinyKV的事务设计遵循 Percolator；它是一个两阶段提交协议（2PC）。


&emsp;&emsp;一个事务是一个读和写的组合。一个事务有一个开始时间戳，当一个事务被提交时，它有一个提交时间戳（必须大于开始时间戳）。整个事务从开始时间戳有效的 key 的版本中读取。在提交之后，所有的写入似乎都是在提交时间戳处写入的。任何要写入的 key 在开始时间戳和提交时间戳之间不得被其他事务写入，否则，整个事务被取消（这被称为写冲突）。



&emsp;&emsp;该协议开始时，客户端从 TinyScheduler 获得一个开始时间戳。然后，它在本地建立事务，从数据库中读取（使用包括开始时间戳的 KvGet 或 KvScan 请求，与 RawGet 或 RawScan 请求相反），但只在本地内存中记录写入。一旦事务被建立，客户端将选择一个 key 作为主键（注意，这与SQL主键无关）。客户端会向 TinyKV 发送KvPrewrite 消息。一个 KvPrewrite 消息包含了事务中的所有写内容。TinyKV 服务器将尝试锁定事务所需的所有key。如果锁定任何一个 key 失败，那么 TinyKV 就会向客户端回应该事务已经失败。客户端可以稍后重试该事务（即用不同的开始时间戳）。如果所有的 key 都被锁定，那么预写就会成功。每个锁都存储了事务的主键和一个生存时间（TTL）。


&emsp;&emsp;事实上，由于一个事务中的 key 可能在多个 region ，因此存储在不同的 Raft 组中，客户端将发送多个 KvPrewrite 请求，一个给每个 region 的领导者。每个预写只包含该 region 的修改内容。如果所有的预写都成功，那么客户端将为包含主键的 region 发送一个提交请求。提交请求将包含一个提交时间戳（客户端也从 TinyScheduler 获得），这是事务的写入被提交的时间，从而对其他事务可见。

&emsp;&emsp;如果任何预写失败，那么客户端就会通过向所有 region 发送 KvBatchRollback 请求来回滚该事务（以解锁该事务中的所有 key 并删除任何预写的值）。


&emsp;&emsp;在 TinyKV 中，TTL 检查不是自发进行的。为了启动超时检查，客户端在一个 KvCheckTxnStatus 请求中向 TinyKV 发送当前时间。该请求通过其主键和开始时间戳来识别事务。该锁可能丢失或已经提交；如果不是，TinyKV 将该锁的 TTL 与 KvCheckTxnStatus 请求中的时间戳相比较。如果锁已经超时，那么 TinyKV 就会回滚该锁。在任何情况下，TinyKV 都会响应锁的状态，这样客户端就可以通过发送KvResolveLock 请求来采取行动。当客户端由于另一个事务的锁而无法预写一个事务时，通常会检查事务状态。


&emsp;&emsp;如果主键提交成功，那么客户端将提交其他 region 的所有其他 key。这些请求应该总是成功的，因为通过对预写请求的积极响应，服务器承诺，如果它收到该事务的提交请求，那么它将会成功。一旦客户端得到了所有的预写响应，事务失败的唯一途径就是超时，在这种情况下，提交主键就会失败。一旦主键被提交，那么其他键就不能再超时了。

&emsp;&emsp;如果主键提交失败，那么客户端将通过发送 KvBatchRollback 请求来回滚该事务。

## Part A

&emsp;&emsp;你在早期项目中实现的原始 API 直接将用户的 key 和 value 映射到存储在底层存储（Badger）中的 key 和 value 。由于 Badger 不知道分布式事务层，你必须在TinyKV 中处理事务，并将用户的 key 和 value 编码到底层存储中。这是用多版本并发控制（MVCC）实现的。在这个项目中，你将在 TinyKV 中实现 MVCC 层。

&emsp;&emsp;实现 MVCC 意味着使用一个简单的 key/value API来表示 事务API。TinyKV 不是为每个 key 存储一个值，而是为每个 key 存储每一个版本的值。例如，如果一个 key 的值是10，然后被设置为20，TinyKV 将存储这两个值（10和20）以及它们有效时的时间戳。

&emsp;&emsp;TinyKV使用三个列族（CF）：default用来保存用户值，lock用来存储锁，write用来记录变化。使用 userkey 可以访问 lock CF；它存储一个序列化的 Lock 数据结构（在lock.go 中定义）。默认的 CF 使用 user key 和写入事务的开始时间戳进行访问；它只存储 user value。写入 CF 使用 user key 和写入事务的提交时间戳进行访问；它存储一个写入数据结构（在 write.go 中定义）。

&emsp;&emsp;user key 和时间戳被组合成一个编码的 key 。 key 的编码方式是编码后的 key 的升序首先是 user key（升序），然后是时间戳（降序）。这就保证了对编码后的 key 进行迭代时，会先给出最新的版本。编码和解码 key 的辅助函数在 transaction.go 中定义。

&emsp;&emsp;这个练习需要实现一个叫做 MvccTxn 的结构。在 PartB & PartC，你将使用 MvccTxn API 来实现事务性API。MvccTxn 提供了基于 user key 和锁、写和值的逻辑表示的读写操作。修改被收集在 MvccTxn 中，一旦一个命令的所有修改被收集，它们将被一次性写到底层数据库中。这就保证了命令的成功或失败都是原子性的。请注意，MVCC 事务与 TinySQL 事务是不同的。一个 MVCC 事务包含了对单个命令的修改，而不是一连串的命令。

&emsp;&emsp;MvccTxn 是在 transaction.go 中定义的。有一个 stub 实现，以及一些用于编码和解码 key 的辅助函数。测试在 transaction_test.go 中。在这个练习中，你应该实现MvccTxn 的每一个方法，以便所有测试都能通过。每个方法都记录了它的预期行为。



> 提示：
> - 一个 MvccTxn 应该知道它所代表的请求的起始时间戳。
> - 最具挑战性的实现方法可能是 GetValue 和 retrieving writes 的方法。你将需要使用 StorageReader 来遍历CF。牢记编码 key 的顺序，并记住当决定一个值是否有效时，取决于事务的提交时间戳，而不是开始时间戳。


## Part B


&emsp;&emsp;在这一部分，你将使用 Part A 的 MvccTxn 来实现对 KvGet、KvPrewrite 和KvCommit 请求的处理。如上所述，KvGet 在提供的时间戳处从数据库中读取一个值。如果在 KvGet 请求的时候，要读取的 key 被另一个事务锁定，那么 TinyKV 应该返回一个错误。否则，TinyKV 必须搜索该 key 的版本以找到最新的、有效的值。

&emsp;&emsp;KvPrewrite 和 KvCommit 分两个阶段向数据库写值。这两个请求都对多个 key 进行操作，但是实现可以独立处理每个 key 。

&emsp;&emsp;KvPrewrite 是一个值被实际写入数据库的地方。一个 key 被锁定，一个 key 被存储。我们必须检查另一个事务没有锁定或写入同一个 key 。

&emsp;&emsp;KvCommit 并不改变数据库中的值，但它确实记录了该值被提交。如果 key 没有被锁定或被其他事务锁定，KvCommit 将失败。

&emsp;&emsp;你需要实现 server.go 中定义的 KvGet、KvPrewrite 和 KvCommit 方法。每个方法都接收一个请求对象并返回一个响应对象，你可以通过查看 kvrpcpb.proto 中的协议定义看到这些对象的内容（你应该不需要改变协议定义）。

&emsp;&emsp;TinyKV 可以同时处理多个请求，所以有可能出现局部竞争条件。例如，TinyKV 可能同时收到两个来自不同客户端的请求，其中一个提交了一个 key，而另一个回滚了同一个 key 。为了避免竞争条件，你可以锁定数据库中的任何 key。这个锁存器的工作原理很像一个每个 key 的 mutex。latches.go 定义了一个 Latches 对象，为其提供API。


> 提示：
> - 所有的命令都是一个事务的一部分。事务是由一个开始时间戳（又称开始版本）来识别的。
> - 任何请求都可能导致 region 错误，这些应该以相同的方式处理。


## Part C


&emsp;&emsp;在这一部分，你将实现 KvScan、KvCheckTxnStatus、KvBatchRollback 和KvResolveLock。在高层次上，这与 Part B 类似 - - 使用 MvccTxn 实现 server.go 中的 gRPC 请求处理程序。

&emsp;&emsp;KvScan 相当于 RawScan 的事务性工作，它从数据库中读取许多值。但和 KvGet 一样，它是在一个时间点上进行的。由于 MVCC 的存在，KvScan 明显比 RawScan 复杂得多 - - 由于多个版本和 key 编码的存在，你不能依靠底层存储来迭代值。

&emsp;&emsp;KvCheckTxnStatus、KvBatchRollback 和 KvResolveLock 是由客户端在试图写入事务时遇到某种冲突时使用的。每一个都涉及到改变现有锁的状态。

&emsp;&emsp;KvCheckTxnStatus 检查超时，删除过期的锁并返回锁的状态。

&emsp;&emsp;KvBatchRollback 检查一个 key 是否被当前事务锁定，如果是，则删除该锁，删除任何值，并留下一个回滚指示器作为写入。

&emsp;&emsp;KvResolveLock 检查一批锁定的 key ，并将它们全部回滚或全部提交。



> 提示：
> - 对于扫描，你可能会发现实现你自己的扫描器（迭代器）抽象是有帮助的，它对逻辑值进行迭代，而不是对底层存储的原始值。
> - 在扫描时，一些错误可以记录在单个 key 上，不应该导致整个扫描的停止。对于其他命令，任何引起错误的单个 key 都应该导致整个操作的停止。
> - 由于 KvResolveLock 可以提交或回滚它的 key，你应该能够与KvBatchRollback 和 KvCommit 实现共享代码。
> - 一个时间戳包括一个物理和一个逻辑部分。物理部分大致是壁钟时间的单调版本。通常情况下，我们使用整个时间戳，例如在比较时间戳是否相等时。然而，当计算超时时，我们必须只使用时间戳的物理部分。要做到这一点，你可能会发现 transaction.go 中的 PhysicalTime 函数很有用。



# Percolator：两阶段提交协议 2PC
## 介绍
&emsp;&emsp;Percolator 基于单行事务实现了多行事务，Google BigTable 能够提供单行事务。在这里 TinyKV 也会通过锁来保证单行数据的原子性

![在这里插入图片描述](https://img-blog.csdnimg.cn/131680c65d664b8eb750c4e5b20a443b.png)

&emsp;&emsp;Percolator 提供了 5 种 Column Family 分别为 lock，write，data，notify，ack_O。在 TinyKV 中我们只需要使用 Lock，Write 和 Data。其中 Data 使用 Default 替代。三个 CF 意义分别为：


- Default：实际的数据，存在多版本，版本号就是写入事务的 startTs，一个版本对应一次写入；
- Lock：锁标记，版本号就是写入事务的 startTs，同时每一个 lock 上含有 primary key 的值；
- Write：Write 上存在 startTs 和 commitTs，startTs 是指向对应 Data 中的版本，commitTs 是这个 Write 的创建时间；



&emsp;&emsp;Percolator 本质上是一个 2PC，但是传统 2PC 在 Coordinator 挂了的时候，就会留下一堆烂摊子，后续接任者需要根据烂摊子的状态来判断是继续 commit 还是 rollback。而 primary key 在这里的作用就是标记这个烂摊子是继续 commit 还是直接 rollback。

&emsp;&emsp;Percolator会不会产生死锁？不可能，因为你可以注意到，任何事务遇到冲突都是回滚自身，而不会等待。（破坏死锁成立条件之一：循环等待）。但是会有可能产生活锁。




&emsp;&emsp;一个事务要保证自身写的操作不会被别的事务干扰（防止写写冲突），也就是事务 T1 在修改 A 的时候，别的事务就不能修改 A。Percolator 在所有的 key 上加了 lock，而 lock 里面含有 primary key 信息，也就是将这些锁的状态和 primary key 绑定。Primary key 的 lock 在，它们就在；Primary key 的 lock 不在，那么它们的 lock 也不能在。


&emsp;&emsp;Primary key 是从写入的 keys 里面随机选的，在 commit 时需要优先处理 primary key。 当 primary key 执行成功后，其他的 key 可以异步并发执行，因为 primary key 写入成功代表已经决议完成了，后面的状态都可以根据 primary key 的状态进行判断。




## 数据写入


&emsp;&emsp;2PC 将数据的提交分为两段，一段为 prewrite，另一段为 commit。所有写入的数据会先被保存在 client 缓存中，只有当提交时，才触发上述两段提交。


**Prewrite**


对每个 key 需要执行如下操作：

>注意，如下 3 步操作需要保证原子性，也就是需要开启单行事务，BigTable 是支持的，不过在 TinyKV 中，我们使用 server.Latches.AcquireLatches() 实现。

1. 检查要写入的 key 是否存在大于 startTs 的 Write，如果存在，直接放弃，说明在事务开启后，已经存在写入并已提交，也就是存在写-写冲突；
2. 检查要写入 key 的数据是否存在 Lock（任意 startTs下的 Lock），即检测是否存在写写冲突，如果存在直接放弃；
3. 如果通过上面两项检查，写入 Lock 和 Data，时间戳为你的 startTs。Lock 里面包含着 primary key 信息；


为什么第2步要检查任意 startTs 下的 Lock：

- Lock 的 startTs 小于当前事务的 startTs：如果读了，就会产生脏读，因为前一个事务都没有 commit 就读了。
- Lock 的 startTs 大于当前事务的 startTs：如果读了并修改了然后提交，拥有这个 lock 的事务会产生不可重复读。
- Lock 的 startTs 等于当前事务的 startTs：不可能发生，因为当重启事务之后，是分配一个新的 startTs，不可能使用一个过去的 startTs 去执行重试操作；


**Commit**

优先 commit primary key，和 prewrite 一样，需要开启单行事务。



1. 从中心授时器获取一个 commitTs；
2. 检查 key 的 lock 的时间戳是否为事务的 startTs，不是直接放弃。因为存在如下可能，致使 Key 可能存在不属于当前事务的 Lock
   -  在当前 commit 的时候，前面的 prewrite 操作由于网络原因迟迟未到，导致 key 并没有加上该事务的锁。
   - 在当前 commit 的时候，前面的 prewrite 操作因为过于缓慢，超时，导致你的 lock 被其他事务 rollback 了。
3. 新增一条 Write，写入的 Write 包含了 startTs 和 commitTs，startTs 的作用是帮助你查找对应的 Default，因为 Default 的时间戳是 startTs。
4. 删除其对应的 lock；


之后开始提交其他所有的 key，步骤和 primary key 一样。（可以异步并发操作，加快速度）。



&emsp;&emsp;如果执行完 3 Write 后，在第 4 步清除 lock 挂了怎么办？不可能发生，我们开启了单行事务，如果第 4 步没有执行成功，那么 1,2,3 会被回滚，满足 atomicity。



## 数据读取

1. 读取某一个 key 的数据，检查其是否存在小于或等于 startTs 的 lock，如果存在说明在本次读取时还存在未 commit 的事务，先等一会，如果等超时了 lock 还在，则尝试 rollback。如果直接强行读会产生脏读，读取了未 commit 的数据；
2. 查询 commitTs 小于 startTs 的最新的 Write，如果不存在则返回数据不存在；
3. 根据 Write 的 startTs 从 Default 中获取数据；



## Rollback


回滚操作又 Primary key 的状态决定，存在如下四种情况：




1. Primary key 的 Lock 还在，代表之前的事务没有 commit，就选择回滚。通过 Write 给一个 rollback 标记；
2. Primary key 上面的 Lock 已经不存在，且有了 Write，那么代表 primary key 已经被 commit 了，这里我们选择继续推进 commit；
3. Primary key 既没有 Lock 也没有 Write，那么说明之前连 Prewrite 阶段都还没开始，客户端重试即可；
4. Primary key 的 Lock 还在，但并不是当前事务的，也即被其他事务 Lock 了，这样的话就 Write 一个 rollback 标记，然后返回即可；


为什么第 4 种情况下，key 被其他事务 Lock 了，仍然要给 rollback 标记？


- 在某些情况下，一个事务回滚之后，TinyKV 仍然有可能收到同一个事务的 prewrite 请求。比如，可能是网络原因导致该请求在网络上滞留比较久；或者由于 prewrite 的请求是并行发送的，客户端的一个线程收到了冲突的响应之后取消其它线程发送请求的任务并调用 rollback，此时其中一个线程的 prewrite 请求刚好刚发出去。也就是说，被回滚的事务，它的 prewrite 可能比 rollback 还要后到。

- 如果 rollback 发现 key 被其他事务 lock 了，并且不做任何处理。那么假设在 prewrite 到来时，这个 lock 已经没了，由于没有 rollback 标记，这个 prewrite 就会执行成功，则回滚操作就失败了。如果有 rollback 标记，那么 prewrite 看到它之后就会立刻放弃，从而不影响回滚的效果。

- 另外，打了 rollback 标记是没有什么影响的，即使没有上述网络问题。因为 rollback 是指向对应 start_ts 的 default 的，也就是该事务写入的 value，它并不会影响其他事务的写入情况，因此不管它就行。



# Project4A

&emsp;&emsp;project4A 实现一些基本操作，比如获取 value、给 lock、给 write 等等，供 project4B/C 调用，要完善的文件为 transaction.go。在进行具体实现之前，需要先了解三个前缀的结构：


## Lock

&emsp;&emsp;Lock 的 Key 仅仅由 Cf_Lock 和源 Key 拼接而成，不含 Ts 信息。Lock 的 Ts 信息同 Ttl、Kind、Primary Key 一并存在 Value 中。

![在这里插入图片描述](https://img-blog.csdnimg.cn/50b59caa0b974c82a31b31beb2d2cc15.png)




## Write

&emsp;&emsp;不同于 Lock，Write 的 Key 中是整合了 commitTs 的，首先通过 EncodeKey 将源 Key 和 commitTs 编码在一起，然后和 Cf_Write 拼接形成新的 Key。Write 的 StartTs 同 Kind 一并存在 Value 中。

![在这里插入图片描述](https://img-blog.csdnimg.cn/bd767116c7c44e93b42e185fe50e30a0.png)


## Default


&emsp;&emsp;不同于 Write，Default 的 Key 中整合的是 startTs，而不是 commitTs，用于 Write 进行索引，写入的值存在 Value 中即可。

![在这里插入图片描述](https://img-blog.csdnimg.cn/5d567685d9d64f2ca3540de1fe02584c.png)


## GetValue

查询当前事务下，传入 key 对应的 Value。

1. 通过 iter.Seek(EncodeKey(key, txn.StartTS)) 查找遍历 Write，找到 commitTs <= ts 最新 Write；
2. 判断找到 Write 的 key 是不是就是自己需要的 key，如果不是，说明不存在，直接返回；
3. 判断 Write 的 Kind 是不是 WriteKindPut，如果不是，说明不存在，直接返回；
4. 从 Default 中通过 EncodeKey(key, write.StartTS) 获取值；

```go
// GetValue finds the value for key, valid at the start timestamp of this transaction.
// I.e., the most recent value committed before the start of this transaction.
//查询当前事务下，传入 key 对应的 Value。
func (txn *MvccTxn) GetValue(key []byte) ([]byte, error) {
	// Your Code Here (4A).
	// 首先需要根据 write 确定最近版本是不是有效的
	// 如果是有效的，则从 default 中取出 value
	// Your Code Here (4A).
	// 1. 遍历 Write 通过 iter.Seek(EncodeKey(key, txn.StartTS)) 查找遍历 Write，找到 commitTs <= ts 最新 Write；
	iter := txn.Reader.IterCF(engine_util.CfWrite)
	defer iter.Close()
	iter.Seek(EncodeKey(key, txn.StartTS)) // 找到的总是最新版本的 user key
	if !iter.Valid() {
		return nil, nil
	}
	// 2. 判断找到的 key 是不是自己需要的 key
	userKey := DecodeUserKey(iter.Item().KeyCopy(nil))
	if !bytes.Equal(userKey, key) {
		return nil, nil
	}
	// 3. 判断 Write 的 Kind 是不是 WriteKindPut
	value, err := iter.Item().ValueCopy(nil)
	if err != nil {
		return nil, err
	}
	write, err := ParseWrite(value)
	if err != nil {
		return nil, err
	}
	// 4. 从 Default 中获取值
	if write.Kind == WriteKindPut {
		return txn.Reader.GetCF(engine_util.CfDefault, EncodeKey(key, write.StartTS))
	}
	return nil, nil
}
```


## CurrentWrite

查询当前事务下，传入 key 的最新 Write。


1. 通过 iter.Seek(EncodeKey(key, math.MaxUint64)) 查询该 key 的最新 Write；
2. 如果 write.StartTS > txn.StartTS，继续遍历，直到找到 write.StartTS == txn.StartTS 的 Write；
3. 返回这个 Write 和 commitTs；




```go
// CurrentWrite searches for a write with this transaction's start timestamp. It returns a Write from the DB and that
// write's commit timestamp, or an error.
//查询当前事务下，传入 key 的最新 Write。
func (txn *MvccTxn) CurrentWrite(key []byte) (*Write, uint64, error) {
	// Your Code Here (4A).
	iter := txn.Reader.IterCF(engine_util.CfWrite)
	defer iter.Close()
	//1. 通过 iter.Seek(EncodeKey(key, math.MaxUint64)) 查询该 key 的最新 Write；
	for iter.Seek(EncodeKey(key, ^uint64(0))); iter.Valid(); iter.Next() {
		item := iter.Item()
		gotKey := item.KeyCopy(nil)
		userKey := DecodeUserKey(gotKey)
		if !bytes.Equal(userKey, key) {
			return nil, 0, nil
		}
		value, err := item.ValueCopy(nil)
		if err != nil {
			return nil, 0, err
		}
		write, err := ParseWrite(value)
		if err != nil {
			return nil, 0, err
		}
		//2. 如果 write.StartTS > txn.StartTS，继续遍历，直到找到 write.StartTS == txn.StartTS 的 Write
		if write.StartTS == txn.StartTS {
			//3. 返回这个 Write 和 commitTs；
			return write, decodeTimestamp(gotKey), nil
		}
	}
	return nil, 0, nil
}
```


## MostRecentWrite


查询传入 key 的最新 Write，这里不需要考虑事务的 startTs。

1. 通过 iter.Seek(EncodeKey(key, math.MaxUint64)) 查找；
2. 判断目标 Write 的 key 是不是我们需要的，不是返回空，是直接返回该 Write；



```go
// MostRecentWrite finds the most recent write with the given key. It returns a Write from the DB and that
// write's commit timestamp, or an error.
//查询传入 key 的最新 Write，这里不需要考虑事务的 startTs。
func (txn *MvccTxn) MostRecentWrite(key []byte) (*Write, uint64, error) {
	// Your Code Here (4A).
	iter := txn.Reader.IterCF(engine_util.CfWrite)
	defer iter.Close()
	// 1. 通过 iter.Seek(EncodeKey(key, math.MaxUint64)) 查找；
	iter.Seek(EncodeKey(key, ^uint64(0)))
	if !iter.Valid() {
		return nil, 0, nil
	}
	userKey := DecodeUserKey(iter.Item().KeyCopy(nil))
	if !bytes.Equal(userKey, key) {
		return nil, 0, nil
	}
	value, err := iter.Item().ValueCopy(nil)
	if err != nil {
		return nil, 0, err
	}
	//2. 判断目标 Write 的 key 是不是我们需要的，不是返回空
	write, err := ParseWrite(value)
	if err != nil {
		return nil, 0, err
	}
	// 是直接返回该 Write；
	return write, decodeTimestamp(iter.Item().KeyCopy(nil)), nil
}
```



## 其他一些简单接口


```go
// PutValue adds a key/value write to this transaction.
func (txn *MvccTxn) PutValue(key []byte, value []byte) {
	// Your Code Here (4A).
	modify := storage.Modify{
		Data: storage.Put{
			Key:   EncodeKey(key, txn.StartTS),
			Cf:    engine_util.CfDefault,
			Value: value,
		},
	}
	txn.writes = append(txn.writes, modify)
}

// DeleteValue removes a key/value pair in this transaction.
func (txn *MvccTxn) DeleteValue(key []byte) {
	// Your Code Here (4A).
	modify := storage.Modify{
		Data: storage.Delete{
			Key: EncodeKey(key, txn.StartTS),
			Cf:  engine_util.CfDefault,
		},
	}
	txn.writes = append(txn.writes, modify)
}

// GetLock returns a lock if key is locked. It will return (nil, nil) if there is no lock on key, and (nil, err)
// if an error occurs during lookup.
func (txn *MvccTxn) GetLock(key []byte) (*Lock, error) {
	// Your Code Here (4A).
	value, err := txn.Reader.GetCF(engine_util.CfLock, key)
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, nil
	}
	lock, err := ParseLock(value) // 反序列化 Lock 结构体
	if err != nil {
		return nil, err
	}
	return lock, nil
}

// PutLock adds a key/lock to this transaction.
func (txn *MvccTxn) PutLock(key []byte, lock *Lock) {
	// Your Code Here (4A).
	modify := storage.Modify{
		Data: storage.Put{
			Key:   key,
			Cf:    engine_util.CfLock,
			Value: lock.ToBytes(), // 序列化 Lock 结构体
		},
	}
	txn.writes = append(txn.writes, modify)
}

// DeleteLock adds a delete lock to this transaction.
func (txn *MvccTxn) DeleteLock(key []byte) {
	// Your Code Here (4A).
	modify := storage.Modify{
		Data: storage.Delete{
			Key: key,
			Cf:  engine_util.CfLock,
		},
	}
	txn.writes = append(txn.writes, modify)
}

// PutWrite records a write at key and ts.
func (txn *MvccTxn) PutWrite(key []byte, ts uint64, write *Write) {
	// Your Code Here (4A).
	modify := storage.Modify{
		Data: storage.Put{
			Key:   EncodeKey(key, ts),
			Cf:    engine_util.CfWrite,
			Value: write.ToBytes(),
		},
	}
	txn.writes = append(txn.writes, modify)
}
```



# Project4B
&emsp;&emsp;在 Percolator 中，Google Table 提供了单行锁，其能保证单行的操作是原子的，但是在 TinyKV 中，并不提供这样的保证。因为我们其实是多行的数据，不是单行。所以我们需要 server.go 中提供的 Latches 来保证对同一个 key 修改的原子性。（为什么我们是多行？看 Project1。）

&emsp;&emsp;project4B 主要实现事务的两段提交，即 prewrite 和 commit，需要完善的文件是 server.go。要注意的是，这要需要通过 server.Latches 对 keys 进行加锁。


## KvGet
获取单个 key 的 Value。

1. 通过 Latches 上锁对应的 key；
2. 获取 Lock，如果 Lock 的 startTs 小于当前的 startTs，说明存在你之前存在尚未 commit 的请求，中断操作，返回 LockInfo；
3. 否则直接获取 Value，如果 Value 不存在，则设置 NotFound = true；

```go
// KvGet 只需要判断一下锁的状态，锁存在并且时间戳小于 txn.StartTS 就等待锁释放，返回给客户端
func (server *Server) KvGet(_ context.Context, req *kvrpcpb.GetRequest) (*kvrpcpb.GetResponse, error) {
	// Your Code Here (4B).
	resp := &kvrpcpb.GetResponse{}
	// 1. 获取 Reader
	reader, err := server.storage.Reader(req.Context)
	if regionErr, ok := err.(*raft_storage.RegionError); ok {
		resp.RegionError = regionErr.RequestErr
		return resp, nil
	}
	defer reader.Close()
	// 2. 创建事务，获取 Lock
	txn := mvcc.NewMvccTxn(reader, req.Version)
	lock, err := txn.GetLock(req.Key)
	if regionErr, ok := err.(*raft_storage.RegionError); ok {
		resp.RegionError = regionErr.RequestErr
		return resp, nil
	}
	// 3. Percolator 为了保证快照隔离的时候总是能读到已经 commit 的数据
	// 当发现准备读取的数据被锁定的时候，会等待解锁
	if lock != nil && req.Version >= lock.Ts {
		resp.Error = &kvrpcpb.KeyError{
			Locked: &kvrpcpb.LockInfo{
				PrimaryLock: lock.Primary,
				LockVersion: lock.Ts,
				Key:         req.Key,
				LockTtl:     lock.Ttl,
			},
		}
	}
	// 4. 获取 value
	value, err := txn.GetValue(req.Key)
	if err != nil {
		if regionErr, ok := err.(*raft_storage.RegionError); ok {
			resp.RegionError = regionErr.RequestErr
			return resp, nil
		}
		return nil, err
	}
	if value == nil {
		resp.NotFound = true
	}
	resp.Value = value
	return resp, nil
}
```


## KvPrewrite

进行 2PC 阶段中的第一阶段。


1. 对所有的 key 上锁；
2. 通过 MostRecentWrite 检查所有 key 的最新 Write，如果存在，且其 commitTs 大于当前事务的 startTs，说明存在 write conflict，终止操作；
3. 通过 GetLock() 检查所有 key 是否有 Lock，如果存在 Lock，说明当前 key 被其他事务使用中，终止操作；
4. 到这一步说明可以正常执行 Prewrite 操作了，写入 Default 数据和 Lock；



```go
// KvPrewrite 检测 key 是否出现冲突，如果没有的话写入 lock 和 default
// 冲突检测：1. 事务开始之后 write 列是否有数据 2. lock 列是否有数据，不需要在意 lock 的时间
func (server *Server) KvPrewrite(_ context.Context, req *kvrpcpb.PrewriteRequest) (*kvrpcpb.PrewriteResponse, error) {
	// Your Code Here (4B).
	resp := &kvrpcpb.PrewriteResponse{}
	reader, err := server.storage.Reader(req.Context)
	if regionErr, ok := err.(*raft_storage.RegionError); ok {
		resp.RegionError = regionErr.RequestErr
		return resp, nil
	}
	defer reader.Close()
	// 检测事务需要修改的 key 是否出现冲突
	txn := mvcc.NewMvccTxn(reader, req.StartVersion)
	var keyErrors []*kvrpcpb.KeyError
	for _, operation := range req.Mutations {
		write, ts, err := txn.MostRecentWrite(operation.Key)
		if err != nil {
			if regionErr, ok := err.(*raft_storage.RegionError); ok {
				resp.RegionError = regionErr.RequestErr
				return resp, nil
			}
			return nil, err
		}
		// 检测在 StartTS 之后是否有已经提交的 Write，如果有的话说明写冲突，需要 abort 当前的事务
		if write != nil && ts >= req.StartVersion {
			keyErrors = append(keyErrors, &kvrpcpb.KeyError{
				Conflict: &kvrpcpb.WriteConflict{
					StartTs:    req.StartVersion,
					ConflictTs: ts,
					Key:        operation.Key,
					Primary:    req.PrimaryLock,
				},
			})
			continue
		}
		// 检测 Key 是否有 Lock 锁住，如果有的话则说明别的事务可能正在修改
		lock, err := txn.GetLock(operation.Key)
		if err != nil {
			if regionErr, ok := err.(*raft_storage.RegionError); ok {
				resp.RegionError = regionErr.RequestErr
				return resp, nil
			}
			return nil, err
		}
		if lock != nil {
			keyErrors = append(keyErrors, &kvrpcpb.KeyError{
				Locked: &kvrpcpb.LockInfo{
					PrimaryLock: req.PrimaryLock,
					LockVersion: lock.Ts,
					Key:         operation.Key,
					LockTtl:     lock.Ttl,
				},
			})
			continue
		}
		// 暂存修改到 txn 中，然后对需要修改的 Key 进行加锁
		var kind mvcc.WriteKind
		switch operation.Op {
		case kvrpcpb.Op_Put:
			kind = mvcc.WriteKindPut
			txn.PutValue(operation.Key, operation.Value)
		case kvrpcpb.Op_Del:
			kind = mvcc.WriteKindDelete
			txn.DeleteValue(operation.Key)
		default:
			return nil, nil
		}
		txn.PutLock(operation.Key, &mvcc.Lock{
			Primary: req.PrimaryLock,
			Ts:      req.StartVersion,
			Ttl:     req.LockTtl,
			Kind:    kind,
		})
	}
	// 判断是否有 key 出错了，如果有的话需要 abort 事务
	if len(keyErrors) > 0 {
		resp.Errors = keyErrors
		return resp, nil
	}
	// 写入事务中暂存的修改到 storage 中
	err = server.storage.Write(req.Context, txn.Writes())
	if err != nil {
		if regionErr, ok := err.(*raft_storage.RegionError); ok {
			resp.RegionError = regionErr.RequestErr
			return resp, nil
		}
		return nil, err
	}
	return resp, nil
}
```


## KvCommit

进行 2PC 阶段中的第二阶段。

1. 通过 Latches 上锁对应的 key；
2. 尝试获取每一个 key 的 Lock，并检查 Lock.StartTs 和当前事务的 startTs 是否一致，不一致直接取消。因为存在这种情况，客户端 Prewrite 阶段耗时过长，Lock 的 TTL 已经超时，被其他事务回滚，所以当客户端要 commit 的时候，需要先检查一遍 Lock；
3. 如果成功则写入 Write 并移除 Lock；


```go
// KvCommit 检查 lock，没有冲突的话就写入 write 并清除 lock
// lock 必须要存在，并且时间戳需要等于当前事务的开始时间戳
func (server *Server) KvCommit(_ context.Context, req *kvrpcpb.CommitRequest) (*kvrpcpb.CommitResponse, error) {
	// Your Code Here (4B).
	resp := &kvrpcpb.CommitResponse{}
	reader, err := server.storage.Reader(req.Context)
	if err != nil {
		if regionErr, ok := err.(*raft_storage.RegionError); ok {
			resp.RegionError = regionErr.RequestErr
			return resp, nil
		}
		return nil, err
	}
	defer reader.Close()
	txn := mvcc.NewMvccTxn(reader, req.StartVersion)
	server.Latches.WaitForLatches(req.Keys)
	defer server.Latches.ReleaseLatches(req.Keys)
	for _, key := range req.Keys {
		// 1. 检测是否是重复提交（测试程序需要）
		write, _, err := txn.CurrentWrite(key)
		if err != nil {
			if regionErr, ok := err.(*raft_storage.RegionError); ok {
				resp.RegionError = regionErr.RequestErr
				return resp, nil
			}
			return nil, err
		}
		// Rollback 类型的 Write 表示是已经正确提交的，这个时候按照重复提交处理
		if write != nil && write.Kind != mvcc.WriteKindRollback && write.StartTS == req.StartVersion {
			return resp, nil
		}
		// 2. 检查每个 Key 的 Lock 是否还存在
		lock, err := txn.GetLock(key)
		if err != nil {
			if regionErr, ok := err.(*raft_storage.RegionError); ok {
				resp.RegionError = regionErr.RequestErr
				return resp, nil
			}
			return nil, err
		}
		// 如果 Lock 不存在，有两种情况：第一种是事务已经正确提交了，这次是一个重复提交；第二种是这个事务被别的事务清除了
		if lock == nil || lock.Ts != req.StartVersion {
			resp.Error = &kvrpcpb.KeyError{Retryable: "true"}
			return resp, nil
		}
		// 3. 第一次提交事务，正常处理
		txn.PutWrite(key, req.CommitVersion, &mvcc.Write{
			StartTS: req.StartVersion,
			Kind:    lock.Kind,
		})
		txn.DeleteLock(key)
	}
	err = server.storage.Write(req.Context, txn.Writes())
	if err != nil {
		if regionErr, ok := err.(*raft_storage.RegionError); ok {
			resp.RegionError = regionErr.RequestErr
			return resp, nil
		}
		return nil, err
	}
	return resp, nil
}
```


# Project4C

&emsp;&emsp;project4C 要在 project4B 的基础上，实现扫面、事务状态检查、批量回滚、清除锁这四个操作。



## KvScan


&emsp;&emsp;该方法要基于 project1 中的 iter 生成一个新的迭代器，不同于 project1，在 project4 中 key 是整和了 ts 的，所以这里的迭代器要把这个 ts 抽出来，只返回原有的 key 和对应的 value，如下：



![在这里插入图片描述](https://img-blog.csdnimg.cn/0dfd554364d84be49d442579d18c2878.png)







```go
func (server *Server) KvScan(_ context.Context, req *kvrpcpb.ScanRequest) (*kvrpcpb.ScanResponse, error) {
	// Your Code Here (4C).
	resp := &kvrpcpb.ScanResponse{}
	reader, err := server.storage.Reader(req.Context)
	if err != nil {
		if regionErr, ok := err.(*raft_storage.RegionError); ok {
			resp.RegionError = regionErr.RequestErr
			return resp, nil
		}
		return nil, err
	}
	txn := mvcc.NewMvccTxn(reader, req.Version)
	scanner := mvcc.NewScanner(req.StartKey, txn)
	defer reader.Close()
	defer scanner.Close()
	var pairs []*kvrpcpb.KvPair // 存储扫描到的 {key, value}
	// 查找 key
	for i := 0; i < int(req.Limit); {
		key, value, err := scanner.Next()
		if err != nil {
			if regionErr, ok := err.(*raft_storage.RegionError); ok {
				resp.RegionError = regionErr.RequestErr
				return resp, nil
			}
			return nil, err
		}
		// key 为空表示没有数据了
		if key == nil {
			break
		}

		lock, err := txn.GetLock(key)
		if err != nil {
			if regionErr, ok := err.(*raft_storage.RegionError); ok {
				resp.RegionError = regionErr.RequestErr
				return resp, nil
			}
			return nil, err
		}
		if lock != nil && req.Version >= lock.Ts {
			pairs = append(pairs, &kvrpcpb.KvPair{
				Error: &kvrpcpb.KeyError{
					Locked: &kvrpcpb.LockInfo{
						PrimaryLock: lock.Primary,
						LockVersion: lock.Ts,
						Key:         key,
						LockTtl:     lock.Ttl,
					},
				},
				Key: key,
			})
			i++
			continue
		}
		if value != nil {
			pairs = append(pairs, &kvrpcpb.KvPair{Key: key, Value: value})
			i++
		}
	}
	resp.Pairs = pairs
	return resp, nil
}
```

```go
// Scanner is used for reading multiple sequential key/value pairs from the storage layer. It is aware of the implementation
// of the storage layer and returns results suitable for users.
// Invariant: either the scanner is finished and cannot be used, or it is ready to return a value immediately.
type Scanner struct {
	// Your Data Here (4C).
	nextKey  []byte
	txn      *MvccTxn
	iter     engine_util.DBIterator
	finished bool
}

// NewScanner creates a new scanner ready to read from the snapshot in txn.
func NewScanner(startKey []byte, txn *MvccTxn) *Scanner {
	// Your Code Here (4C).
	return &Scanner{
		nextKey: startKey,
		txn:     txn,
		iter:    txn.Reader.IterCF(engine_util.CfWrite),
	}
}

func (scan *Scanner) Close() {
	// Your Code Here (4C).
	scan.iter.Close()
}

// Next returns the next key/value pair from the scanner. If the scanner is exhausted, then it will return `nil, nil, nil`.
func (scan *Scanner) Next() ([]byte, []byte, error) {
	// Your Code Here (4C).
	if scan.finished {
		return nil, nil, nil
	}
	// 查找 CfWrite
	key := scan.nextKey
	scan.iter.Seek(EncodeKey(key, scan.txn.StartTS))
	if !scan.iter.Valid() {
		scan.finished = true
		return nil, nil, nil
	}
	item := scan.iter.Item()
	gotKey := item.KeyCopy(nil)
	userKey := DecodeUserKey(gotKey)
	if !bytes.Equal(userKey, key) {
		scan.nextKey = userKey
		return scan.Next()
	}
	// 跳过所有相同的 key
	for {
		scan.iter.Next()
		if !scan.iter.Valid() {
			scan.finished = true
			break
		}
		item := scan.iter.Item()
		gotKey := item.KeyCopy(nil)
		userKey := DecodeUserKey(gotKey)
		if !bytes.Equal(userKey, key) {
			scan.nextKey = userKey
			break
		}
	}
	writeVal, err := item.ValueCopy(nil)
	if err != nil {
		return key, nil, err
	}
	write, err := ParseWrite(writeVal)
	if err != nil {
		return key, nil, err
	}
	if write.Kind == WriteKindDelete {
		return key, nil, nil
	}
	// 查找 CfDefault
	value, err := scan.txn.Reader.GetCF(engine_util.CfDefault, EncodeKey(key, write.StartTS))
	return key, value, err
}

```


## KvCheckTxnStatus

用于 Client failure 后，想继续执行时先检查 Primary Key 的状态，以此决定是回滚还是继续推进 commit。

1. 通过 CurrentWrite() 获取 primary key 的 Write，记为 write。通过 GetLock() 获取 primary key 的 Lock，记为 lock；
2. 如果 write 不是 WriteKindRollback，则说明已经被 commit 了，不用管，直接返回 commitTs
3. 如果 lock 为 nil，进入如下操作：
   - 如果 write 为 WriteKindRollback，则说明已经被回滚了，因此无需操作，返回 Action_NoAction 即可；
   - 否则，打上 rollback 标记（WriteKindRollback），返回 Action_LockNotExistRollback；
4. 如果 lock 存在，并且超时了。那么删除这个 lock，并且删除对应的 Value，同时打上 rollback 标记，然后返回 Action_TTLExpireRollback；
5. 如果 lock 存在，但是并没有超时，则直接返回，等待其自己超时；



```go
// KvCheckTxnStatus 检查超时时间，删除过期的锁并返回锁的状态
func (server *Server) KvCheckTxnStatus(_ context.Context, req *kvrpcpb.CheckTxnStatusRequest) (*kvrpcpb.CheckTxnStatusResponse, error) {
	// Your Code Here (4C).
	resp := &kvrpcpb.CheckTxnStatusResponse{}
	reader, err := server.storage.Reader(req.Context)
	if err != nil {
		if regionErr, ok := err.(*raft_storage.RegionError); ok {
			resp.RegionError = regionErr.RequestErr
			return resp, nil
		}
		return nil, err
	}
	defer reader.Close()
	txn := mvcc.NewMvccTxn(reader, req.LockTs)
	// 1. 检查 PrimaryKey 是否存在 Write，Write 的开始时间戳与 req.LockTs 相等
	write, ts, err := txn.CurrentWrite(req.PrimaryKey)
	if err != nil {
		if regionErr, ok := err.(*raft_storage.RegionError); ok {
			resp.RegionError = regionErr.RequestErr
			return resp, nil
		}
		return nil, err
	}
	if write != nil {
		// WriteKind 如果不是 WriteKindRollback 则说明已经被 commit
		if write.Kind != mvcc.WriteKindRollback {
			resp.CommitVersion = ts
		}
		return resp, nil
	}
	// 2. 检查 lock 是否存在
	lock, err := txn.GetLock(req.PrimaryKey)
	if err != nil {
		if regionErr, ok := err.(*raft_storage.RegionError); ok {
			resp.RegionError = regionErr.RequestErr
			return resp, nil
		}
		return nil, err
	}
	// lock 不存在，说明 primary key 已经被回滚了，创建一个 WriteKindRollback
	if lock == nil {
		txn.PutWrite(req.PrimaryKey, req.LockTs, &mvcc.Write{
			StartTS: req.LockTs,
			Kind:    mvcc.WriteKindRollback,
		})
		err = server.storage.Write(req.Context, txn.Writes())
		if err != nil {
			if regionErr, ok := err.(*raft_storage.RegionError); ok {
				resp.RegionError = regionErr.RequestErr
				return resp, nil
			}
			return nil, err
		}
		resp.Action = kvrpcpb.Action_LockNotExistRollback
		return resp, nil
	}
	// lock 不为空，检查 lock 是否超时，如果超时则移除 Lock 和 Value，创建一个 WriteKindRollback
	if mvcc.PhysicalTime(lock.Ts)+lock.Ttl <= mvcc.PhysicalTime(req.CurrentTs) {
		txn.DeleteLock(req.PrimaryKey)
		txn.DeleteValue(req.PrimaryKey)
		txn.PutWrite(req.PrimaryKey, req.LockTs, &mvcc.Write{
			StartTS: req.LockTs,
			Kind:    mvcc.WriteKindRollback,
		})
		err = server.storage.Write(req.Context, txn.Writes())
		if err != nil {
			if regionErr, ok := err.(*raft_storage.RegionError); ok {
				resp.RegionError = regionErr.RequestErr
				return resp, nil
			}
			return nil, err
		}
		resp.Action = kvrpcpb.Action_TTLExpireRollback
	}
	return resp, nil
}
```


## KvBatchRollback

用于批量回滚。


1. 首先对要回滚的所有 key 进行检查，通过 CurrentWrite 获取到每一个 key write，如果有一个 write 不为 nil 且不为 WriteKindRollback，则说明存在 key 已经提交，则拒绝回滚，将 Abort 赋值为 true，然后返回即可；
2. 通过 GetLock 获取到每一个 key 的 lock。如果 lock.Ts != txn.StartTs，则说明这个 key 被其他事务 lock 了，但仍然要给 rollback 标记，原因前文有述；
3. 如果 key 的 write 是 WriteKindRollback，则说明已经回滚完毕，跳过该 key；
4. 如果上两者都没有，则说明需要被回滚，那么就删除 lock 和 Value，同时打上 rollback 标记，然后返回即可；



```go
// KvBatchRollback 批量回滚 key
func (server *Server) KvBatchRollback(_ context.Context, req *kvrpcpb.BatchRollbackRequest) (*kvrpcpb.BatchRollbackResponse, error) {
	// Your Code Here (4C).
	resp := &kvrpcpb.BatchRollbackResponse{}
	reader, err := server.storage.Reader(req.Context)
	if err != nil {
		if regionErr, ok := err.(*raft_storage.RegionError); ok {
			resp.RegionError = regionErr.RequestErr
			return resp, nil
		}
		return nil, err
	}
	defer reader.Close()
	txn := mvcc.NewMvccTxn(reader, req.StartVersion)
	server.Latches.WaitForLatches(req.Keys)
	defer server.Latches.ReleaseLatches(req.Keys)
	for _, key := range req.Keys {
		// 1. 获取 key 的 Write，如果已经是 WriteKindRollback 则跳过这个 key
		write, _, err := txn.CurrentWrite(key)
		if err != nil {
			if regionErr, ok := err.(*raft_storage.RegionError); ok {
				resp.RegionError = regionErr.RequestErr
				return resp, nil
			}
			return nil, err
		}
		if write != nil {
			if write.Kind == mvcc.WriteKindRollback {
				continue
			} else {
				resp.Error = &kvrpcpb.KeyError{Abort: "true"}
				return resp, nil
			}
		}
		// 获取 Lock，如果 Lock 被清除或者 Lock 不是当前事务的 Lock，则中止操作
		// 这个时候说明 key 被其他事务占用
		// 否则的话移除 Lock、删除 Value，写入 WriteKindRollback 的 Write
		lock, err := txn.GetLock(key)
		if err != nil {
			if regionErr, ok := err.(*raft_storage.RegionError); ok {
				resp.RegionError = regionErr.RequestErr
				return resp, nil
			}
			return nil, err
		}
		if lock == nil || lock.Ts != req.StartVersion {
			txn.PutWrite(key, req.StartVersion, &mvcc.Write{
				StartTS: req.StartVersion,
				Kind:    mvcc.WriteKindRollback,
			})
			continue
		}
		txn.DeleteLock(key)
		txn.DeleteValue(key)
		txn.PutWrite(key, req.StartVersion, &mvcc.Write{
			StartTS: req.StartVersion,
			Kind:    mvcc.WriteKindRollback,
		})
	}
	err = server.storage.Write(req.Context, txn.Writes())
	if err != nil {
		if regionErr, ok := err.(*raft_storage.RegionError); ok {
			resp.RegionError = regionErr.RequestErr
			return resp, nil
		}
		return nil, err
	}
	return resp, nil
}
```



## KvResolveLock


&emsp;&emsp;这个方法主要用于解决锁冲突，当客户端已经通过 KvCheckTxnStatus() 检查了 primary key 的状态，这里打算要么全部回滚，要么全部提交，具体取决于 ResolveLockRequest 的 CommitVersion。


1. 通过 iter 获取到含有 Lock 的所有 key；
2. 如果 req.CommitVersion == 0，则调用 KvBatchRollback() 将这些 key 全部回滚；
3. 如果 req.CommitVersion > 0，则调用 KvCommit() 将这些 key 全部提交；





```go
func (server *Server) KvResolveLock(_ context.Context, req *kvrpcpb.ResolveLockRequest) (*kvrpcpb.ResolveLockResponse, error) {
	// Your Code Here (4C).
	resp := &kvrpcpb.ResolveLockResponse{}
	reader, err := server.storage.Reader(req.Context)
	if err != nil {
		if regionErr, ok := err.(*raft_storage.RegionError); ok {
			resp.RegionError = regionErr.RequestErr
			return resp, nil
		}
		return nil, err
	}
	iter := reader.IterCF(engine_util.CfLock)
	defer reader.Close()
	defer iter.Close()
	var keys [][]byte
	for ; iter.Valid(); iter.Next() {
		item := iter.Item()
		value, err := item.ValueCopy(nil)
		if err != nil {
			return resp, err
		}
		lock, err := mvcc.ParseLock(value)
		if err != nil {
			return resp, nil
		}
		if lock.Ts == req.StartVersion {
			key := item.KeyCopy(nil)
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return resp, nil
	}
	// 根据 request.CommitVersion 提交或者 Rollback
	if req.CommitVersion == 0 {
		resp1, err := server.KvBatchRollback(nil, &kvrpcpb.BatchRollbackRequest{
			Context:      req.Context,
			StartVersion: req.StartVersion,
			Keys:         keys,
		})
		resp.Error, resp.RegionError = resp1.Error, resp1.RegionError
		return resp, err
	} else {
		resp1, err := server.KvCommit(nil, &kvrpcpb.CommitRequest{
			Context:       req.Context,
			StartVersion:  req.StartVersion,
			Keys:          keys,
			CommitVersion: req.CommitVersion,
		})
		resp.Error, resp.RegionError = resp1.Error, resp1.RegionError
		return resp, err
	}
}
```


# 参考
- [sakura-ysy ^-^](https://github.com/sakura-ysy/TinyKV-2022-doc/blob/main/doc/project4.md)
- [Smith-Cruise](https://github.com/Smith-Cruise/TinyKV-White-Paper/blob/main/Project4-Transaction.md)