# 前言


&emsp;&emsp;project2部分又分成了a、b、c三个小部分。可以说从project1到project2，难度剧增。本文介绍project2的Part A的内容。开始做project2a之前，需要`多读几遍论文`，否则其中的细节会让你非常头疼。而熟悉了论文之后，很多地方按照论文所叙述的去实现即可。比如日志复制与选举安全性。

&emsp;&emsp;本文前面介绍的都是project2a怎么做，初学者可以先去看文末“raft协议我认为重要的地方”


# Project2 RaftKV 文档翻译


&emsp;&emsp;Raft 是一种共识算法，其设计理念是易于理解。我们可以在Raft网站上阅读关于 Raft 的材料，Raft 的交互式可视化，以及其他资源，包括Raft的扩展论文。


&emsp;&emsp;在这个项目中，将实现一个基于raft的高可用kv服务器，这不仅需要实现 Raft 算法，还需要实际使用它，这会带来更多的挑战，比如用 badger 管理 Raft 的持久化状态，为快照信息添加流控制等。



该项目有3个部分需要去实现，包括：

- 实现基本的 Raft 算法（本文）
- 在 Raft 之上建立一个容错的KV服务
- 增加对 raftlog GC 和快照的支持


**==PartA==**


&emsp;&emsp;在这一部分，将实现基本的 Raft 算法。需要实现的代码在 raft/ 下。在 raft/ 里面，有一些框架代码和测试案例。在这里实现的 Raft 算法有一个与上层应用程序的接口。此外，它使用一个逻辑时钟（这里命名为 tick ）来测量选举和心跳超时，而不是物理时钟。也就是说，`不要在Raft模块本身设置一个计时器，上层应用程序负责通过调用 RawNode.Tick() 来推进逻辑时钟`。除此之外，消息的发送和接收以及其他事情都是异步处理的，何时真正做这些事情也是由上层应用决定的（更多细节见下文）。例如，Raft 不会在阻塞等待任何请求消息的响应。


&emsp;&emsp;在实现之前，请先查看这部分的提示。另外，你应该粗略看一下proto文件 proto/proto/`eraftpb.proto` 。那里定义了 Raft 发送和接收消息以及相关的`结构`，你将使用它们来实现。`注意，与 Raft 论文不同，它将心跳和 AppendEntries 分为不同的消息，以使逻辑更加清晰。`



这一部分可以分成3个步骤，包括：

- 领导者选举
- 日志复制
- 原始节点接口


**==实现 Raft 算法==**




&emsp;&emsp;raft/raft.go 中的 raft.Raft 提供了 Raft 算法的核心，包括消息处理、驱动逻辑时钟等。关于更多的实现指南，请查看 raft/doc.go ，其中包含了概要设计和这些MessageTypes 负责的内容。


&emsp;&emsp;领导者选举：为了实现领导者选举，你可能想从 raft.Raft.tick() 开始，它被用来通过一个 tick 驱动内部逻辑时钟，从而驱动选举超时或心跳超时。你现在不需要关心消息的发送和接收逻辑。如果你需要发送消息，只需将其推送到 raft.Raft.msgs ，Raft 收到的所有消息将被传递到 raft.Raft.Step()。测试代码将从 raft.Raft.msgs 获取消息，并通过raft.Raft.Step() 传递响应消息。raft.Raft.Step() 是消息处理的入口，你应该处理像MsgRequestVote、MsgHeartbeat 这样的消息及其响应。也请实现 test stub 函数，并让它们被正确调用，如raft.Raft.becomeXXX，当 Raft 的角色改变时，它被用来更新 Raft 的内部状态。


你可以运行make project2aa来测试实现，并在这部分的最后看到一些提示。



&emsp;&emsp;日志复制：为了实现日志复制，你可能想从处理发送方和接收方的 MsgAppend 和MsgAppendResponse 开始。查看 raft/log.go 中的 raft.RaftLog，这是一个辅助结构，可以帮助你管理 raft 日志，在这里还需要通过 raft/storage.go 中定义的 Storage 接口与上层应用进行交互，以获得日志项和快照等持久化数据。


你可以运行make project2ab来测试实现，并在这部分的最后看到一些提示。



**==实现原始节点接口==**


&emsp;&emsp;raft/rawnode.go 中的 raft.RawNode 是与上层应用程序交互的接口，raft.RawNode 包含 raft.Raft 并提供一些封装函数，如 RawNode.Tick() 和 RawNode.Step() 。它还提供了 RawNode.Propose() 来让上层应用提出新的 Raft 日志。


&emsp;&emsp;另一个重要的结构 Ready 也被定义在这里。在处理消息或推进逻辑时钟时，raft.Raft 可能需要与上层应用进行交互，比如：

- 向其他 peer 发送消息
- 将日志项保存到稳定存储中
- 将term、commit index 和 vote 等 hard state 保存到稳定存储中
- 将已提交的日志条目应用于状态机
- 等等



&emsp;&emsp;但是这些交互不会立即发生，相反，它们被封装在 Ready 中并由 RawNode.Ready() 返回给上层应用程序。这取决于上层应用程序何时调用 RawNode.Ready() 并处理它。在处理完返回的 Ready 后，上层应用程序还需要调用一些函数，如 RawNode.Advance() 来更新 raft.Raft 的内部状态，如apply\ied index、stabled log index等。


你可以运行make project2ac来测试实现，运行make project2a来测试整个A部分。


> 提示：
> - 在 raft.Raft、raft.RaftLog、raft.RawNode 和 eraftpb.proto 上添加任何你需要的状态。
> - 测试假设第一次启动的 Raft 应该有 term 0。
> - 测试假设新当选的领导应该在其任期内附加一个 noop 日志项。
> - 测试没有为本地消息、MessageType_MsgHup、MessageType_MsgBeat和 MessageType_MsgPropose 设置term。
> - 在领导者和非领导者之间，追加的日志项是相当不同的，有不同的来源、检查和处理，要注意这一点。
> - 不要忘了选举超时在 peers 之间应该是不同的。
> - rawnode.go 中的一些封装函数可以用 raft.Step(local message) 实现。
> - 当启动一个新的 Raft 时，从 Storage 中获取最后的稳定状态来初始化raft.Raft 和 raft.RaftLog。



# Project2A重点内容抛出
&emsp;&emsp;在Project2A中，我们需要完成三个目标，这三个目标分别对应三个文件。说白了，我们需要实现raft算法，并提供接口给上层使用。


- 领导者选举 - raft.go
- 日志复制 - log.go
- 原始节点接口 - rawnode.go

![图片来源sakura-ysy](https://img-blog.csdnimg.cn/590d0338d01247979b9b7012bd82632f.png)
ps：图片来源：https://github.com/Smith-Cruise/TinyKV-White-Paper/blob/main/Project2-RaftKV.md

&emsp;&emsp;在raftlog中，存储了raft的所有日志信息。在raft中，主要是用来raft集群同步。在rawnode主要是封装接口，进行持久化，应用到状态机等。所以我们需要先从raftlog入手，先看一看raftlog是如何封装的。


# RaftLog

## RaftLog结构体字段详解

&emsp;&emsp;我刚开始看到这个结构体的时候非常头疼，看不懂字段的意思，下来我来简单介绍一下。

首先要明白raft的`2个提交阶段`和`快照，持久化`是什么意思：
- commit：`一个日志被大多数节点接收`，那么它就算被commit提交了
- apply：只有被commit的日志，才能被apply，apply就是把日志放入状态机去执行，状态机是什么？后文再说，总之，就是`执行被commit的日志，被执行了就是被apply了`。那么必然 commit>=apply
- 持久化：我们知道正在运行的节点，日志肯定是先存到`entries []pb.Entry`里面的，那么问题来了，如果服务器宕机了，那日志不就没了？？？所以这些在内存里面的日志叫做未持久化。节点所有的日志，我们都需要存到磁盘里面进行持久化。那么下次重启节点的时候，只需要去磁盘中读取一下（`storage字段`）就可以还原出上一次运行的状态。那么什么时候持久化呢？在rawnode.go 进行 Advance 的时候来推进；怎么持久化，这点需要你在 peer_msg_handler.go 中实现
- 快照snapshot：那如果有10000w条日志，难道把这么多日志都存储起来里吗？显然不是，为了防止节点 '存储' 了过多日志，加入快照功能。说白了就是把一些已经完成同步的日志直接给删了，然后把`状态`存下来，这样就可以避免存在磁盘中的日志过多，所以会`删除一定的持久化日志并生成 snapshot`

&emsp;&emsp;有了上面的理解，再去理解RaftLog的字段就会简单很多

- **storage**：自从上次快照以来，所有的持久化条目
- **commited**：就是论文里面的committedIndex，节点认为哪些是已经提交的日志 `的索引`
- **applied**：论文中的 lastApplied，即节点最新应用到状态机的日志 `的索引`
- **stabled**：被持久化日志的最后一条日志的索引，后面开始就是未持久化日志的索引
- **entries**：所有未被 compact 的日志，包括持久化与非持久化。理解为没有持久化的日志即可，被持久化的日志要从storage字段去取
- **pendingSnapshot**：待处理快照，partC再介绍
- **dummyIndex**：这个字段是自己加的，我认为其最主要的作用就是记录`entries[0]`对应的日志的索引是多少，因为切片是从零开始的，我们需要记录日志的索引，所有需要加一个这个字段才能够算出日志的真实索引是多少。如果这里对日志索引有疑惑的读者，可以先去看一下论文。


&emsp;&emsp;在etcd中，已持久化日志和未持久化日志是分开记录的，在tinykv中，就全部放在entries中的，那么怎么区分呢？通过`stabled`字段来区分即可，已持久化的就是`stabled`前面的日志，没持久化的就是后面的日志。




```go
// RaftLog manage the log entries, its struct look like:
//
//  snapshot/first.....applied....committed....stabled.....last
//  --------|------------------------------------------------|
//                            log entries
//
// for simplify the RaftLog implement should manage all log entries
// that not truncated
type RaftLog struct {
	// storage contains all stable entries since the last snapshot.
	// 存储包含自上次快照以来的所有稳定条目。
	storage Storage

	// committed is the highest log position that is known to be in
	// stable storage on a quorum of nodes.
	// 已知已提交的最高的日志条目的索引     committedIndex
	committed uint64

	// applied is the highest log position that the application has
	// been instructed to apply to its state machine.
	// Invariant: applied <= committed
	// 已经被应用到状态机的最高的日志条目的索引 appliedIndex
	applied uint64

	// log entries with index <= stabled are persisted to storage.
	// It is used to record the logs that are not persisted by storage yet.
	// Everytime handling `Ready`, the unstabled logs will be included.
	// stabled 保存的是已经持久化到 storage 的 index
	stabled uint64

	// all entries that have not yet compact.
	// 尚未压缩的所有条目
	entries []pb.Entry

	// the incoming unstable snapshot, if any.
	// 2C
	pendingSnapshot *pb.Snapshot

	// Your Data Here (2A).
	dummyIndex uint64
}
```


## RaftLog核心函数详解

&emsp;&emsp;前面也说过storage 的作用了，那么我们现在就可以理解为现在是一台重启的节点，我们所有的数据都需要从持久化的磁盘中取。

```go
// newLog returns log using the given storage. It recovers the log
// to the state that it just commits and applies the latest snapshot.
// newLog使用给定存储返回日志。它将日志恢复到刚刚提交并应用最新快照的状态。
func newLog(storage Storage) *RaftLog {
	// Your Code Here (2A).
	firstIndex, _ := storage.FirstIndex()
	lastIndex, _ := storage.LastIndex()
	entries, _ := storage.Entries(firstIndex, lastIndex+1)
	hardState, _, _ := storage.InitialState()

	rl := &RaftLog{
		storage:   storage,
		committed: hardState.Commit,
		applied:   firstIndex - 1,
		stabled:   lastIndex,
		entries:   entries,
		//2C
		pendingSnapshot: nil,
		dummyIndex:      firstIndex,
	}

	return rl
}
```

&emsp;&emsp;剩下的几个函数按照要求和结构体去推理就能写出来了，不过这里需要注意，有点函数在后面partB，partC中是需要修改的，因为partA本文是不涉及到快照的。所以本文只能通过make project2a的测试。


```go
// allEntries return all the entries not compacted.
// note, exclude any dummy entries from the return value.
// note, this is one of the test stub functions you need to implement.
// allEntries返回所有未压缩的条目。
// 注意，从返回值中排除任何虚拟条目。
// 注意，这是您需要实现的测试存根函数之一。
func (l *RaftLog) allEntries() []pb.Entry {
	// Your Code Here (2A).
	return l.entries
}

// unstableEntries return all the unstable entries
// 返回所有未持久化到 storage 的日志
func (l *RaftLog) unstableEntries() []pb.Entry {
	// Your Code Here (2A).
	return l.getEntries(l.stabled+1, 0)
}

// nextEnts returns all the committed but not applied entries
// 返回所有已经提交但没有应用的日志
// 返回在(applied，committed]之间的日志
func (l *RaftLog) nextEnts() (ents []pb.Entry) {
	// Your Code Here (2A).
	//fst applied=5 , committed=5 , dummyIndex=6
	//sec applied=5 , committed=10 , dummyIndex=6
	//want [6,7,8,9,10]
	//idx  [0,1,2,3,4 , 5) ===>[0,5)
	//diff = dummyIndex - 1 =5

	diff := l.dummyIndex - 1
	return l.entries[l.applied-diff : l.committed-diff]
}

// LastIndex return the last index of the log entries
// 返回日志项的最后一个索引
func (l *RaftLog) LastIndex() uint64 {
	// Your Code Here (2A).
	return l.dummyIndex - 1 + uint64(len(l.entries))
}

// Term return the term of the entry in the given index
// 返回给定索引中log的term
func (l *RaftLog) Term(i uint64) (uint64, error) {
	// Your Code Here (2A).
	if i >= l.dummyIndex {
		return l.entries[i-l.dummyIndex].Term, nil
	} else {
		// 否则的话 i 只能是快照中的日志
		term, err := l.storage.Term(i)
		return term, err
	}
	//2C
}


//其他辅助函数见github
```


&emsp;&emsp;RaftLog理清楚之后，如何对日志进行操作呢？在raft里面涉及到日志赋值等内容，在rawnode设计持久化，快照，应用状态机等内容。所以在此要被RaftLog的字段`理解透彻`了再看下文。


# Raft

&emsp;&emsp;在raft模块里面，首先要明白两个部分

- raft是如何驱动的
- Msg消息的作用


##  Raft 驱动规则

&emsp;&emsp;`tick()`：Raft 的时钟是一个逻辑时钟，上层通过不断调用 Tick() 来模拟时间的递增。对于如何递增，以及递增之后超时的各种处理，后文再说。tcik会被上层接口rawnode调用，所以目前不需要我们去调用tick，只需要去实现tick即可。其实这里就是进行计时，判断有没有超时，超时的发起选举之类


&emsp;&emsp;`Step(m pb.Message)`：上层接口rownode通过调用step将RPC消息传递给raft，raft再通过msg的类型和自己的类型回调不同的函数。下面将Msg介绍完之后再具体介绍Step


##  Msg的作用与含义


&emsp;&emsp;Msg 分为两种，分别为 Local message 和Common message。Local message 是本地发起的 message，比如 propose 数据，发起选举等等。 Common message是其他节点通过网络发来的 msg。一个很简单的判断方法：是否需要广播到整个集群，如果要广播，那就是Common message，用来做集群之间的同步。

&emsp;&emsp;TinyKV 通过 pb.Message 结构定义了所有的 Msg，即共用一个结构体。下面将详细介绍每一种类型的作用与处理

```go
const (
	// 'MessageType_MsgHup' is a local message used for election. If an election timeout happened,
	// the node should pass 'MessageType_MsgHup' to its Step method and start a new election.
	MessageType_MsgHup MessageType = 0
	// 'MessageType_MsgBeat' is a local message that signals the leader to send a heartbeat
	// of the 'MessageType_MsgHeartbeat' type to its followers.
	MessageType_MsgBeat MessageType = 1
	// 'MessageType_MsgPropose' is a local message that proposes to append data to the leader's log entries.
	MessageType_MsgPropose MessageType = 2
	// 'MessageType_MsgAppend' contains log entries to replicate.
	MessageType_MsgAppend MessageType = 3
	// 'MessageType_MsgAppendResponse' is response to log replication request('MessageType_MsgAppend').
	MessageType_MsgAppendResponse MessageType = 4
	// 'MessageType_MsgRequestVote' requests votes for election.
	MessageType_MsgRequestVote MessageType = 5
	// 'MessageType_MsgRequestVoteResponse' contains responses from voting request.
	MessageType_MsgRequestVoteResponse MessageType = 6
	// 'MessageType_MsgSnapshot' requests to install a snapshot message.
	MessageType_MsgSnapshot MessageType = 7
	// 'MessageType_MsgHeartbeat' sends heartbeat from leader to its followers.
	MessageType_MsgHeartbeat MessageType = 8
	// 'MessageType_MsgHeartbeatResponse' is a response to 'MessageType_MsgHeartbeat'.
	MessageType_MsgHeartbeatResponse MessageType = 9
	// 'MessageType_MsgTransferLeader' requests the leader to transfer its leadership.
	MessageType_MsgTransferLeader MessageType = 11
	// 'MessageType_MsgTimeoutNow' send from the leader to the leadership transfer target, to let
	// the transfer target timeout immediately and start a new election.
	MessageType_MsgTimeoutNow MessageType = 12
)
```


## Msg的 收发与处理 流程
### MsgHup

Local Msg，用于请求节点开始选举

| 字段    | 作用                  |
| ------- | --------------------- |
| MsgType | pb.MessageType_MsgHup |

当节点收到该 Msg 后，会进行相应的判断，如果条件成立，即刻开始选举。判断流程大致为：

1. 将自己置为候选者
2. 如果集群只有一个节点，那把自己置为领导者 return
3. 遍历集群所有节点，发起选举投票消息MsgRequestVoteResponse
4. 投票给自己

至此，MsgHup 的处理流程完毕。

### MsgBeat

Local Msg，用于告知 Leader 该发送心跳了

| 字段    | 值                     |
| ------- | ---------------------- |
| MsgType | pb.MessageType_MsgBeat |

当 Leader 接收到 MsgBeat 时，向其他所有节点发送心跳MessageType_MsgHeartbeat。

### MsgPropose

Local Msg，用于上层请求 propose 条目。字段如下：

| 字段    | 作用                      |
| ------- | ------------------------- |
| MsgType | pb.MessageType_MsgPropose |
| Entries | 要 propose 的条目         |
| To      | 发送给哪个节点            |

该 Msg 只有 Leader 能够处理，其余角色收到后直接返回 ErrProposalDropped。Leader 的处理流程如下：

1. 判断 r.leadTransferee(禅让) 是否等于 None，如果不是，返回 ErrProposalDropped，因为此时集群正在转移 Leader。如果是，往下
2. 把 m.Entries 追加到自己的 Entries 中，在propose中 m.Entries为空
3. 向其他所有节点发送追加日志 RPC，即 MessageType_MsgAppend，用于集群同步
4. 如果集群中只有自己一个节点，则直接更新自己的 committedIndex

至此，MsgPropose 处理流程完毕。

### MsgAppend（日志复制，重点）

Common Msg，用于 Leader 给其他节点同步日志条目，字段如下：

| 字段    | 作用                                                      |
| ------- | --------------------------------------------------------- |
| MsgType | pb.MessageType_MsgAppend                                  |
| To      | 目标节点                                                  |
| From    | 当前节点（LeaderId）                                      |
| Term    | 当前节点的 Term                                           |
| LogTerm | 要发送的条目的前一个条目的 Term，即论文中的 prevLogTerm   |
| Index   | 要发送的条目的前一个条目的 Index，即论文中的 prevLogIndex |
| Entries | 要发送的日志条目                                          |
| Commit  | 当前节点的 committedIndex                                 |

Leader发送：

1. 前置判断：如果要发送的 Index 已经被压缩了，转为发送快照，否则往下；
2. 当 Leader 收到 MsgPropose 后，它会给其他所有节点发送 MsgAppend；
3. 当 MsgAppend 被接收者拒绝时，Leader 会调整 next，重新进行前置判断，如果无需发快照，则按照新的 next 重新发送 MsgAppend ；

follower和candidate 接收与处理：

1. 判断Msg的Term是否大于等于自己的Term，大于等于才往下，如果小于则拒绝(Reject = true)
2. 判断prevLogIndex 是否 大于 自己的最后一条日志的索引，大于说明该节点漏消息了。
3. 判断prevLogTerm 和自己最后一条日志的任期是否相等，不相等说明出现日志冲突
4. 如果漏消息或者日志冲突了，都需要将prevLogIndex缩小，再次去匹配前一个log，直到匹配成功后，将entry追加或者覆盖到本节点上
5. 如果每次都是减1，那么效率太慢，可以找到冲突日志的任期，将返回的index设置为冲突任期的上一个任期的最后一个日志的idx位置
6. 这里是按照论文中来做的，其实不优化也是可以的。
7. 上述冲突都会拒绝(Reject = true)
8. 拒绝后leader收到对应的响应，会把Msg中的idx取出来，设置为下一次的prevLogIndex，然后发送日志复制
9. 如果没有冲突，说明prevLogIndex这个位置匹配上了，如果在follower节点的后面有日志，那么直接截断，再追加Msg传来的日志。
10. 如果进行截断操作，那么要更新持久化的索引stabled，比如之前持久化了10条数据，现在同步之后只剩3条了，那么就要更新，stabled = min(r.RaftLog.stabled, idx-1)  在这里的stabled，只会缩小，变大要被持久化的时候才能变大
11. 更新committedIndex ，取min（leader.commited，m.Index+len(m.entries)）
12. 同意接收，回复MessageType_MsgAppendResponse




其中，不管是接受还是拒绝，都要返回一个 MsgAppendResponse 给 Leader，让 Leader 知道追加是否成功并且更新相关信息。

### MsgAppendResponse（NextIndex，MatchIndex重点）

Common Msg，用于节点告诉 Leader 日志同步是否成功，和 MsgAppend 对应，字段如下：

| 字段    | 值                                                         |
| ------- | ---------------------------------------------------------- |
| MsgType | pb.MessageType_MsgAppendResponse                           |
| Term    | 当前节点的 Term                                            |
| To      | to                                                         |
| Reject  | 是否拒绝                                                   |
| From    | 当前节点的 Id                                              |
| Index   | r.RaftLog.LastIndex()；该字段用于 Leader 更快地去更新 next |

发送：

1. 不管节点接受或是拒绝，都要发送一个 MsgAppendResponse 给 Leader，调整 Reject 字段即可，其他字段固定；

接收与处理：

1. 只有leader才会处理MessageType_MsgAppendResponse
2. 如果被拒绝reject了，那么看一下对方的任期term是否比自己大，比自己大说明可能出现网络分区的情况了，自己变成follower
3. 否则就是prevlog日志冲突所以被拒绝了，那么调整Nextindex和prevlogindex，然后再次发送日志复制MessageType_MsgAppend 
4. 如果没有被拒绝，说明日志复制成功了，则更新match和next（后文会讲），match顾名思义就是匹配，它记录了跟随者返回的日志索引，所以正常情况下，match + 1 = next
5. 论文中也写了，在这个时候尝试更新commit索引
6. 我在这里的处理是：将所有节点的match排序，取中位数，判断这个位置的日志的term和当前term是否一致，一致则更新
7. 为什么要判断是不是当前term？为了一致性，Raft 永远不会通过计算副本的方式提交之前任期的日志，只能通过提交当前任期的日志一并提交之前所有的日志
8. 因为论文里是这样写的，对着论文实现即可




### MsgRequestVote

Common Msg，用于 Candidate 请求投票，字段如下：

| 字段    | 值                            |
| ------- | ----------------------------- |
| MsgType | pb.MessageType_MsgRequestVote |
| Term    | 当前节点的 Term               |
| LogTerm | 节点的最后一条日志的 Term     |
| Index   | 节点的最后一条日志的 Index    |
| To      | 发给谁                        |
| From    | 当前节点的 Id                 |

发送：

1. 当节点开始选举并成为 Candidate 时，立刻向其他所有节点发送 MsgRequestVote；

接收与处理：

1. 判断 Msg 的 Term 是否大于等于自己的 Term，是则变成follower
2. 下面就是投票的条件：按照论文里面来实现即可
3. 如果 votedFor 不为空或者不等于 candidateID，则说明该节点以及投过票了，直接拒绝。否则往下
4. Candidate 的日志至少和自己一样新，那么就给其投票，否者拒绝。新旧判断逻辑如下：
   - 如果两份日志最后的条目的任期号不同，那么任期号大的日志更加新
   - 如果两份日志最后的条目任期号相同，那么日志比较长的那个就更加新；



&emsp;&emsp;Candidate 会通过 r.votes 记录下都有哪些节点同意哪些节点拒绝，当同意的票数过半时，即可成为 Leader，当拒绝的票数过半时，则转变为 Follower。

### MsgRequestVoteResponse

Common Msg，用于节点告诉 Candidate 投票结果，字段如下：

| 字段    | 值                                     |
| ------- | -------------------------------------- |
| MsgType | pb.MessageType_MsgRequestVoteResponsev |
| Term    | 当前节点的 Term                        |
| Reject  | 是否拒绝                               |
| To      | 发给谁                                 |
| From    | 当前节点 Id                            |

发送：

1. 节点收到 MsgRequestVote 时，会将结果通过 MsgRequestVoteResponse 发送给 Candidate；

接收与处理：

1. 只有 Candidate 会处理该 Msg，其余节点收到后直接忽略；
2. 根据 m.Reject 更新 r.votes[m.From]，即记录投票结果；
3. 算出同意的票数 agrNum 和拒绝的票数 denNum；
4. 如果同意的票数过半，那么直接成为 Leader；
5. 如果拒绝的票数过半，那么直接成为 Follower；


**MsgSnapshot**

Common Msg，用于 Leader 将快照发送给其他节点，Project2C 中使用。字段如下：

| 字段     | 值                         |
| -------- | -------------------------- |
| MsgType  | pb.MessageType_MsgSnapshot |
| Term     | 当前节点的 Term            |
| Snapshot | 要发送的快照               |
| To       | 要发给谁                   |
| From     | 当前节点 ID                |

快照部分在 Project2C 中才会实现，所以该 Msg 在 Project2C 处再详述。

**MsgHeartbeat**

Common Msg，即 Leader 发送的心跳。不同于论文中使用空的追加日志 RPC 代表心跳，TinyKV 给心跳一个单独的 MsgType。其字段如下：

| 字段    | 值                          |
| ------- | --------------------------- |
| MsgType | pb.MessageType_MsgHeartbeat |
| Term    | 当前节点的 Term             |
| Commit  | util.RaftInvalidIndex       |
| To      | 发给谁                      |
| From    | 当前节点 ID                 |

其中，Commit 字段必须固定为 util.RaftInvalidIndex，即 0，否则无法通过 Msg 的初始化检查。

发送：

1. 每当 Leader 的 heartbeatTimeout 达到时，就会给其余所有节点发送 MsgHeartbeat；

接收与处理：

1. 判断 Msg 的 Term 是否大于等于自己的 Term，是则变成跟随者，否则拒绝
2. 重置选举计时  r.electionElapsed
3. 发送 MsgHeartbeatResponse

**MsgHeartbeatResponse**

Common Msg，即节点对心跳的回应。字段如下：

| 字段    | 值                                  |
| ------- | ----------------------------------- |
| MsgType | pb.MessageType_MsgHeartbeatResponse |
| Term    | 当前节点的 Term                     |
| To      | 发给谁                              |
| From    | 当前节点 ID                         |
| Commit  | 当前节点的 committedIndex           |

其中，Commit 字段用于告诉 Leader 自己是否落后。

发送：

1. 当节点收到 MsgHeartbeat 时，会相应的回复 MsgHeartbeatResponse；

接收与处理：

1. 只有 Leader 会处理 MsgHeartbeatResponse，其余角色直接忽略
2. 通过 m.Commit 判断节点是否落后了，如果是，则进行日志追加
3. 我这里直接通过Match去判断，问题不大

**MsgTransferLeader**

Local Msg，用于上层请求转移 Leader，Project3 使用。字段如下：

| 字段    | 值                               |
| ------- | -------------------------------- |
| MsgType | pb.MessageType_MsgTransferLeader |
| From    | 由谁转移                         |
| To      | 转移给谁                         |

详细说明将在 Project3 章节叙述。

**MsgTimeoutNow**

Local Msg，节点收到后清空 r.electionElapsed，并即刻发起选举，字段如下：

| 字段    | 值                           |
| ------- | ---------------------------- |
| MsgType | pb.MessageType_MsgTimeoutNow |
| From    | 由谁发的                     |
| To      | 发给谁的                     |



## Raft的驱动

### 计时器 tick

&emsp;&emsp;该函数起到计时器的作用，即逻辑时钟。每调用一次，就要增加节点的心跳计时（ r.electionElapsed），如果是 Leader，就要增加自己的选举计时（ r.heartbeatElapsed），然后，应按照角色进行对应的操作。




```go
// tick advances the internal logical clock by a single tick.
// 推动时间流逝
// 每调用一次，就要增加节点的心跳计时
func (r *Raft) tick() {
	// Your Code Here (2A).
	switch r.State {
	case StateLeader:
		r.leaderTick()
	case StateCandidate:
		r.candidateTick()
	case StateFollower:
		r.followerTick()
	}
}
func (r *Raft) leaderTick() {
	r.heartbeatElapsed++
	if r.heartbeatElapsed >= r.heartbeatTimeout {
		// MessageType_MsgBeat 属于内部消息，不需要经过 RawNode 处理
		r.Step(pb.Message{From: r.id, To: r.id, MsgType: pb.MessageType_MsgBeat})
	}
	//TODO 选举超时 判断心跳回应数量

	//TODO 3A 禅让机制
}
func (r *Raft) candidateTick() {
	r.electionElapsed++
	// 选举超时 发起选举
	if r.electionElapsed >= r.randomElectionTimeout {
		r.electionElapsed = 0
		// MessageType_MsgHup 属于内部消息，也不需要经过 RawNode 处理
		r.Step(pb.Message{From: r.id, To: r.id, MsgType: pb.MessageType_MsgHup})
	}
}
func (r *Raft) followerTick() {
	r.electionElapsed++
	// 选举超时 发起选举
	if r.electionElapsed >= r.randomElectionTimeout {
		r.electionElapsed = 0
		// MessageType_MsgHup 属于内部消息，也不需要经过 RawNode 处理
		r.Step(pb.Message{From: r.id, To: r.id, MsgType: pb.MessageType_MsgHup})
	}
}
```


### 推进器 Step


&emsp;&emsp;Step() 作为驱动器，用来接收上层发来的 Msg，然后根据不同的角色和不同的 MsgType 进行不同的处理。首先，通过 switch-case 将 Step() 按照角色分为三个函数，分别为：FollowerStep() 、CandidateStep()、LeaderStep() 。接着，按照不同的 MsgTaype，将每个 XXXStep() 分为 12 个部分，用来处理不同的 Msg。


![在这里插入图片描述](https://img-blog.csdnimg.cn/529990e958ac41a0b214e9fc19ea494a.png)
&emsp;&emsp;至于最终根据Msg的类型不同调用的不同header，则根据上文的逻辑去编写即可。代码太多就不放出来了。

```go

// Step the entrance of handle message, see `MessageType`
// on `eraftpb.proto` for what msgs should be handled
// 该函数接收一个 Msg，然后根据节点的角色和 Msg 的类型调用不同的处理函数。
func (r *Raft) Step(m pb.Message) error {
	// Your Code Here (2A).
	switch r.State {
	case StateFollower:
		//Follower 可以接收到的消息：
		//MsgHup、MsgRequestVote、MsgHeartBeat、MsgAppendEntry
		r.followerStep(m)
	case StateCandidate:
		//Candidate 可以接收到的消息：
		//MsgHup、MsgRequestVote、MsgRequestVoteResponse、MsgHeartBeat
		r.candidateStep(m)
	case StateLeader:
		//Leader 可以接收到的消息：
		//MsgBeat、MsgHeartBeatResponse、MsgRequestVote
		r.leaderStep(m)
	}
	return nil
}
func (r *Raft) followerStep(m pb.Message) {
	//Follower 可以接收到的消息：
	//MsgHup、MsgRequestVote、MsgHeartBeat、MsgAppendEntry
	switch m.MsgType {
	case pb.MessageType_MsgHup:
		//Local Msg，用于请求节点开始选举，仅仅需要一个字段。
		//TODO MsgHup
		// 成为候选者，开始发起投票
		r.handleStartElection(m)
	case pb.MessageType_MsgBeat:
		//Local Msg，用于告知 Leader 该发送心跳了，仅仅需要一个字段。
		//TODO Follower No processing required
	case pb.MessageType_MsgPropose:
		//Local Msg，用于上层请求 propose 条目
		//TODO Follower No processing required
	case pb.MessageType_MsgAppend:
		//Common Msg，用于 Leader 给其他节点同步日志条目
		//TODO MsgAppendEntry
		r.handleAppendEntries(m)
	case pb.MessageType_MsgAppendResponse:
		//Common Msg，用于节点告诉 Leader 日志同步是否成功，和 MsgAppend 对应
		//TODO Follower No processing required
	case pb.MessageType_MsgRequestVote:
		//Common Msg，用于 Candidate 请求投票
		// TODO MsgRequestVote
		r.handleRequestVote(m)
	case pb.MessageType_MsgRequestVoteResponse:
		//Common Msg，用于节点告诉 Candidate 投票结果
		//TODO Follower No processing required
	case pb.MessageType_MsgSnapshot:
		//Common Msg，用于 Leader 将快照发送给其他节点
		//TODO Follower No processing required
	case pb.MessageType_MsgHeartbeat:
		//Common Msg，即 Leader 发送的心跳。
		//不同于论文中使用空的追加日志 RPC 代表心跳，TinyKV 给心跳一个单独的 MsgType
		//TODO MsgHeartbeat
		// 接收心跳包，重置超时，称为跟随者，回发心跳包的resp
		r.handleHeartbeat(m)
	case pb.MessageType_MsgHeartbeatResponse:
		//Common Msg，即节点对心跳的回应
		//TODO Follower No processing required
	case pb.MessageType_MsgTransferLeader:
		//Local Msg，用于上层请求转移 Leader
		//TODO Follower No processing required
	case pb.MessageType_MsgTimeoutNow:
		//Local Msg，节点收到后清空 r.electionElapsed，并即刻发起选举
		//TODO Follower No processing required
	}
}
func (r *Raft) candidateStep(m pb.Message) {
	//Candidate 可以接收到的消息：
	//MsgHup、MsgRequestVote、MsgRequestVoteResponse、MsgHeartBeat
	switch m.MsgType {
	case pb.MessageType_MsgHup:
		//Local Msg，用于请求节点开始选举，仅仅需要一个字段。
		//TODO MsgHup
		// 成为候选者，开始发起投票
		r.handleStartElection(m)
	case pb.MessageType_MsgBeat:
		//Local Msg，用于告知 Leader 该发送心跳了，仅仅需要一个字段。
		//TODO Candidate No processing required
	case pb.MessageType_MsgPropose:
		//Local Msg，用于上层请求 propose 条目
		//TODO Candidate No processing required
	case pb.MessageType_MsgAppend:
		//Common Msg，用于 Leader 给其他节点同步日志条目
		//TODO MsgAppendEntry
		r.handleAppendEntries(m)
	case pb.MessageType_MsgAppendResponse:
		//Common Msg，用于节点告诉 Leader 日志同步是否成功，和 MsgAppend 对应
		//TODO Candidate No processing required
	case pb.MessageType_MsgRequestVote:
		//Common Msg，用于 Candidate 请求投票
		// TODO MsgRequestVote
		r.handleRequestVote(m)
	case pb.MessageType_MsgRequestVoteResponse:
		//Common Msg，用于节点告诉 Candidate 投票结果
		// TODO MsgRequestVoteResponse
		r.handleRequestVoteResponse(m)
	case pb.MessageType_MsgSnapshot:
		//Common Msg，用于 Leader 将快照发送给其他节点
		//TODO Candidate No processing required
	case pb.MessageType_MsgHeartbeat:
		//Common Msg，即 Leader 发送的心跳。
		//不同于论文中使用空的追加日志 RPC 代表心跳，TinyKV 给心跳一个单独的 MsgType
		//TODO MsgHeartbeat
		// 接收心跳包，重置超时，称为跟随者，回发心跳包的resp
		r.handleHeartbeat(m)
	case pb.MessageType_MsgHeartbeatResponse:
		//Common Msg，即节点对心跳的回应
		//TODO Candidate No processing required
	case pb.MessageType_MsgTransferLeader:
		//Local Msg，用于上层请求转移 Leader
		//要求领导转移其领导权
		//TODO Candidate No processing required
	case pb.MessageType_MsgTimeoutNow:
		//Local Msg，节点收到后清空 r.electionElapsed，并即刻发起选举
		//从领导发送到领导转移目标，以让传输目标立即超时并开始新的选择。
		//TODO Candidate No processing required
	}
}
func (r *Raft) leaderStep(m pb.Message) {
	//Leader 可以接收到的消息：
	//MsgBeat、MsgHeartBeatResponse、MsgRequestVote、MsgPropose、MsgAppendResponse、MsgAppend
	switch m.MsgType {
	case pb.MessageType_MsgHup:
		//Local Msg，用于请求节点开始选举，仅仅需要一个字段。
		//TODO Leader No processing required
	case pb.MessageType_MsgBeat:
		//Local Msg，用于告知 Leader 该发送心跳了，仅仅需要一个字段。
		//TODO MsgBeat
		r.broadcastHeartBeat()
	case pb.MessageType_MsgPropose:
		//Local Msg，用于上层请求 propose 条目
		//TODO MsgPropose
		r.handlePropose(m)
	case pb.MessageType_MsgAppend:
		//Common Msg，用于 Leader 给其他节点同步日志条目
		//TODO 网络分区的情况，也是要的
		r.handleAppendEntries(m)
	case pb.MessageType_MsgAppendResponse:
		//Common Msg，用于节点告诉 Leader 日志同步是否成功，和 MsgAppend 对应
		//TODO MsgAppendResponse
		r.handleAppendEntriesResponse(m)
	case pb.MessageType_MsgRequestVote:
		//Common Msg，用于 Candidate 请求投票
		// TODO MsgRequestVote
		r.handleRequestVote(m)

	case pb.MessageType_MsgRequestVoteResponse:
		//Common Msg，用于节点告诉 Candidate 投票结果
		//TODO Leader No processing required
	case pb.MessageType_MsgSnapshot:
		//Common Msg，用于 Leader 将快照发送给其他节点
		//3A
		//TODO project2C

	case pb.MessageType_MsgHeartbeat:
		//Common Msg，即 Leader 发送的心跳。
		//不同于论文中使用空的追加日志 RPC 代表心跳，TinyKV 给心跳一个单独的 MsgType
		//TODO Leader No processing required
	case pb.MessageType_MsgHeartbeatResponse:
		//Common Msg，即节点对心跳的回应
		//TODO MsgHeartBeatResponse
		r.handleHeartbeatResponse(m)
	case pb.MessageType_MsgTransferLeader:
		//Local Msg，用于上层请求转移 Leader
		//要求领导转移其领导权
		//TODO project3
	case pb.MessageType_MsgTimeoutNow:
		//Local Msg，节点收到后清空 r.electionElapsed，并即刻发起选举
		//从领导发送到领导转移目标，以让传输目标立即超时并开始新的选择。
		//TODO project3
	}
}
```


### follower、candidate、leader

&emsp;&emsp;这里的逻辑比较简单，看一下becomeleader在称为leader后立即发一条entry的原因吧。主要是论文中的要求。

```go
// becomeFollower transform this peer's state to Follower
func (r *Raft) becomeFollower(term uint64, lead uint64) {
	// Your Code Here (2A).

	if term > r.Term {
		// 只有 Term > currentTerm 的时候才需要对 Vote 进行重置
		// 这样可以保证在一个任期内只会进行一次投票
		r.Vote = None
	}
	r.Term = term
	r.State = StateFollower
	r.Lead = lead
	r.electionElapsed = 0
	r.leadTransferee = None
	r.resetRandomizedElectionTimeout()
}

// becomeCandidate transform this peer's state to candidate
func (r *Raft) becomeCandidate() {
	// Your Code Here (2A).
	r.State = StateCandidate // 0. 更改自己的状态
	r.Term++                 // 1. 增加自己的任期
	r.Vote = r.id            // 2. 投票给自己
	r.votes[r.id] = true

	r.electionElapsed = 0 // 3. 重置超时选举计时器
	r.resetRandomizedElectionTimeout()
}

// becomeLeader transform this peer's state to leader
func (r *Raft) becomeLeader() {
	// Your Code Here (2A).
	// NOTE: Leader should propose a noop entry on its term
	//领导者应在其任期内提出noop条目
	r.State = StateLeader
	r.Lead = r.id
	//初始化 nextIndex 和 matchIndex
	for id := range r.Prs {
		r.Prs[id].Next = r.RaftLog.LastIndex() + 1 // 初始化为 leader 的最后一条日志索引（后续出现冲突会往前移动）
		r.Prs[id].Match = 0                        // 初始化为 0 就可以了
	}

	// 成为 Leader 之后立马在日志中追加一条 noop 日志，这是因为
	// 在 Raft 论文中提交 Leader 永远不会通过计算副本的方式提交一个之前任期、并且已经被复制到大多数节点的日志
	// 通过追加一条当前任期的 noop 日志，可以快速的提交之前任期内所有被复制到大多数节点的日志
	r.Step(pb.Message{MsgType: pb.MessageType_MsgPropose, Entries: []*pb.Entry{{}}})
}
```

# RawNode
## Ready 结构体
&emsp;&emsp;刚开始我看这个RawNode真的很懵逼，Ready结构体是干嘛的？Advance是干嘛？


&emsp;&emsp;我们现在可以想一下，在RaftLog中，我们同步了很多日志，那么现在所有节点都是一样的日志，我们如何apply呢？如何向上提交到状态机。如果我们的日志太多，我们如何进行压缩呢？在上面我们发送的Msg，如何去转发呢？


&emsp;&emsp;RawNode 作为一个信息传递的模块，主要就是上层信息的下传和下层信息的上传。既负责从 Raft 中取出数据，也负责向 Raft 中塞数据。

&emsp;&emsp;RawNode 通过生成 Ready 的方式给上层传递信息。这里主要说一下 SoftState 和 HardState。SoftState 不需要被持久化，存粹用在HasReady() 方法中的判断，其判断是否有产生新的 Ready。而 HardState 需要上层进行持久化存储。

&emsp;&emsp;Advance(rd Ready) 是上层处理完了 Ready，通知用于通知 RawNode，以推进整个状态机。

&emsp;&emsp;那么其核心为一个 Ready 结构体。



```go
// Ready encapsulates the entries and messages that are ready to read,
// be saved to stable storage, committed or sent to other peers.
// All fields in Ready are read-only.
//Ready 结构体用于保存已经处于 ready 状态的日志和消息
//这些都是准备保存到持久化存储、提交或者发送给其他节点的
type Ready struct {
	// The current volatile state of a Node.
	// SoftState will be nil if there is no update.
	// It is not required to consume or store SoftState.
	// 不需要被持久化，存粹用在HasReady()中做判断
	*SoftState

	// The current state of a Node to be saved to stable storage BEFORE
	// Messages are sent.
	// HardState will be equal to empty state if there is no update.
	//发送消息之前要保存到稳定存储器的节点的当前状态。
	//如果没有更新，则HardState将等于空状态。
	//HardState包含需要验证的节点的状态，包括当前term、提交索引和投票记录
	pb.HardState

	// Entries specifies entries to be saved to stable storage BEFORE
	// Messages are sent.
	// 需要在发送消息之前被写入到 Storage 的日志
	// 待持久化
	Entries []pb.Entry

	// Snapshot specifies the snapshot to be saved to stable storage.
	// 快照指定要保存到稳定存储的快照。
	Snapshot pb.Snapshot

	// CommittedEntries specifies entries to be committed to a
	// store/state-machine. These have previously been committed to stable
	// store.
	// 需要被输入到状态机中的日志，这些日志之前已经被保存到 Storage 中了
	// 待 apply
	CommittedEntries []pb.Entry

	// Messages specifies outbound messages to be sent AFTER Entries are
	// committed to stable storage.
	// If it contains a MessageType_MsgSnapshot message, the application MUST report back to raft
	// when the snapshot has been received or has failed by calling ReportSnapshot.
	// 如果它包含MessageType_MsgSnapshot消息，则当接收到快照或调用ReportSnapshop失败时，应用程序必须向raft报告。
	// 在日志被写入到 Storage 之后需要发送的消息
	// 待发送
	Messages []pb.Message
}
```

## HasReady

&emsp;&emsp;通过Ready的几个字段我们可以分析出它的核心功能

&emsp;&emsp;RawNode 通过 HasReady() 来判断 Raft 模块是否已经有同步完成并且需要上层处理的信息

- 是否有需要持久化的硬状态
- 是否有需要持久化的日志
- 是否有需要应用的快照
- 是否有需要应用的日志
- 是否有需要发送的 Msg

&emsp;&emsp;如果 HasReady() 返回 true，那么上层就会调用 Ready() 来获取具体要做的事情，和上述 HasReady() 的判断一一对应。该方法直接调用 rn.newReady() 生成一个 Ready() 结构体然后返回即可。

```go
// HasReady called when RawNode user need to check if any Ready pending.
//判断 Raft 模块是否已经有同步完成并且需要上层处理的信息
func (rn *RawNode) HasReady() bool {
	// Your Code Here (2A).
	return 	len(rn.Raft.msgs) > 0 || //是否有需要发送的 Msg
			rn.isHardStateUpdate() || //是否有需要持久化的状态
			len(rn.Raft.RaftLog.unstableEntries()) > 0 || //是否有需要应用的条目
			len(rn.Raft.RaftLog.nextEnts()) > 0 || //是否有需要持久化的条目
			!IsEmptySnap(rn.Raft.RaftLog.pendingSnapshot) //是否有需要应用的快照
}
```
## Advance

&emsp;&emsp;当上层处理完 Ready 后，调用 Advance() 来推进整个状态机。Advance() 的实现就按照 Ready 结构体一点点更改 RawNode 的状态即可，包括：

- prevHardSt 变更
- stabled 指针变更
- applied 指针变更
- 清空 rn.Raft.msgs
- 丢弃被压缩的暂存日志
- 清空 pendingSnapshot

```go
// Advance notifies the RawNode that the application has applied and saved progress in the
// last Ready results.
// Advance通知RawNode，应用程序已经应用并保存了最后一个Ready结果中的进度。
func (rn *RawNode) Advance(rd Ready) {
	// Your Code Here (2A).

	// 不等于 nil 说明上次执行 Ready 更新了 softState
	if rd.SoftState != nil {
		rn.prevSoftSt = rd.SoftState
	}
	// 检查 HardState 是否是默认值，默认值说明没有更新，此时不应该更新 prevHardSt
	if !IsEmptyHardState(rd.HardState) {
		rn.prevHardSt = rd.HardState //prevHardSt 变更；
	}
	// 更新 RaftLog 状态
	if len(rd.Entries) > 0 {
		rn.Raft.RaftLog.stabled += uint64(len(rd.Entries)) //stabled 指针变更；
	}
	if len(rd.CommittedEntries) > 0 {
		rn.Raft.RaftLog.applied += uint64(len(rd.CommittedEntries)) //applied 指针变更；
	}
	rn.Raft.RaftLog.maybeCompact()        //丢弃被压缩的暂存日志；
	rn.Raft.RaftLog.pendingSnapshot = nil //清空 pendingSnapshot；
	rn.Raft.msgs = nil                    //清空 rn.Raft.msgs；
}
```

## 工作流程

1. 上层会不停的调用 RawNode 的 tick() 函数，RawNode 触发 Raft 的 tick() 函数。

```go
// Tick advances the internal logical clock by a single tick.
func (rn *RawNode) Tick() {
	rn.Raft.tick()
}
```

2. 上层会定时从 RawNode 获取 Ready，首先上层通过 HasReady() 进行判断，如果有新的 Ready，上层会调用RawNode 的` Ready()方法进行获取`，RawNode 从 Raft 中 获取信息生成相应的 Ready 返回给上层应用，Raft 的信息则是存储在 RaftLog 之中。上层应用处理完 Ready 后，会调用 RawNode 的 `Advance() 方法进行推进`，告诉 RawNode 之前的 Ready 已经被处理完成，然后你可以执行一些操作，比如修改 applied，stabled 等信息。

```go
// NewRawNode returns a new RawNode given configuration and a list of raft peers.
func NewRawNode(config *Config) (*RawNode, error) {
	// Your Code Here (2A).
	rn := &RawNode{
		Raft: newRaft(config), // 创建底层 Raft 节点
	}
	rn.prevHardSt, _, _ = config.Storage.InitialState()
	rn.prevSoftSt = &SoftState{
		Lead:      rn.Raft.Lead,
		RaftState: rn.Raft.State,
	}
	return rn, nil
}
```

3. 上层应用可以直接调用 RawNode 提供的 Propose(data []byte) ，Step(m pb.Message) 等方法，RawNode 会将这些请求统一包装成 Message，通过 Raft 提供的 Step(m pb.Message) 输入信息。


```go
// Propose proposes data be appended to the raft log.
func (rn *RawNode) Propose(data []byte) error {
	ent := pb.Entry{Data: data}
	return rn.Raft.Step(pb.Message{
		MsgType: pb.MessageType_MsgPropose,
		From:    rn.Raft.id,
		Entries: []*pb.Entry{&ent}})
}
// Step advances the state machine using the given message.
//步骤使用给定消息推进状态机。
func (rn *RawNode) Step(m pb.Message) error {
	// ignore unexpected local messages receiving over network
	if IsLocalMsg(m.MsgType) {
		return ErrStepLocalMsg
	}
	if pr := rn.Raft.Prs[m.From]; pr != nil || !IsResponseMsg(m.MsgType) {
		return rn.Raft.Step(m)
	}
	return ErrStepPeerNotFound
}
```




# Raft协议我认为重要的地方

## 日志复制

### 先再看一遍论文

**3.5 日志复制**


&emsp;&emsp;一旦一个领导者被选举出来，他就开始为客户端提供服务。客户端的每一个请求都包含一条被复制状态机执行的指令。`领导者把这条指令作为一条新的日志条目附加到日志中去`，然后并行的发起附加条目 RPCs 给其他的服务器，让他们复制这条日志条目。当这条日志条目`被安全的复制`（下面会介绍），`领导者会应用这条日志条目到它的状态机中然后把执行的结果返回给客户端。`如果跟随者崩溃或者运行缓慢，再或者网络丢包，`领导者会不断的重复尝试附加日志条目 RPCs （尽管已经回复了客户端）直到所有的跟随者都最终存储了所有的日志条目。`


&emsp;&emsp;日志以图 3.5 展示的方式组织。每一个日志条目存储一条状态机`指令`和从领导者收到这条指令时的`任期号`。日志中的任期号用来检查是否出现不一致的情况，同时也用来保证图 3.2 中的某些性质。每一条日志条目同时也都有一个整数索引值来表明它在日志中的位置。


![在这里插入图片描述](https://img-blog.csdnimg.cn/514e484d8091494b86330934c9b3c9c9.png)

> 图 3.5：日志由有序序号标记的条目组成。每个条目都包含创建时的任期号（图中框中的数字），和一个状态机需要执行的指令。一个条目当可以安全的被应用到状态机中去的时候，就认为是可以提交了。


&emsp;&emsp;领导者来决定什么时候把日志条目应用到状态机中是安全的；这种日志条目被称为已提交。Raft 算法保证所有已提交的日志条目都是持久化的并且最终会被所有可用的状态机执行。在领导者将创建的日志条目复制到大多数的服务器上的时候，日志条目就会被提交（例如在图 3.5 中的条目 7）。同时，领导者的日志中之前的所有日志条目也都会被提交，包括由其他领导者创建的条目。第 3.6 节会讨论某些当在领导者改变之后应用这条规则的隐晦内容，同时他也展示了这种提交的定义是安全的。领导者跟踪了最大的将会被提交的日志项的索引，并且索引值会被包含在未来的所有附加日志 RPCs （包括心跳包），这样其他的服务器才能最终知道领导者的提交位置。一旦跟随者知道一条日志条目已经被提交，那么他也会将这个日志条目应用到本地的状态机中（按照日志的顺序）。


&emsp;&emsp;我们设计了 Raft 的日志机制来维护一个不同服务器的日志之间的高层次的一致性。这么做不仅简化了系统的行为也使得更加可预计，同时他也是安全性保证的一个重要组件。Raft 维护着以下的特性，这些同时也组成了图 3.2 中的日志匹配特性：

- `如果在不同的日志中的两个条目拥有相同的索引和任期号，那么他们存储了相同的指令。`
- `如果在不同的日志中的两个条目拥有相同的索引和任期号，那么他们之前的所有日志条目也全部相同。`


&emsp;&emsp;&emsp;&emsp;第一个特性来自这样的一个事实，领导者最多在一个任期里在指定的一个日志索引位置创建一条日志条目，同时日志条目在日志中的位置也从来不会改变。第二个特性由附加日志 RPC 的一个简单的`一致性检查`所保证。`在发送附加日志 RPC 的时候，领导者会把新的日志条目紧接着之前的条目的索引位置和任期号包含在里面。如果跟随者在它的日志中找不到包含相同索引位置和任期号的条目，那么他就会拒绝接收新的日志条目。`一致性检查就像一个归纳步骤：一开始空的日志状态肯定是满足日志匹配特性的，然后一致性检查保护了日志匹配特性当日志扩展的时候。因此，`每当附加日志 RPC 返回成功时，领导者就知道跟随者的日志一定是和自己相同的了。`



&emsp;&emsp;在正常的操作中，领导者和跟随者的日志保持一致性，所以附加日志 RPC 的一致性检查从来不会失败。然而，领导者崩溃的情况会使得日志处于不一致的状态（老的领导者可能还没有完全复制所有的日志条目）。这种不一致问题会在领导者和跟随者的一系列崩溃下加剧。图 3.6 展示了跟随者的日志可能和新的领导者不同的方式。跟随者可能会丢失一些在新的领导者中有的日志条目，他也可能拥有一些领导者没有的日志条目，或者两者都发生。丢失或者多出日志条目可能会持续多个任期。
![在这里插入图片描述](https://img-blog.csdnimg.cn/4f48f6a9326545dc9c5f78d21b5da31f.png)

> 图 3.6：当一个领导者成功当选时，跟随者可能是任何情况（a-f）。每一个盒子表示是一个日志条目；里面的数字表示任期号。跟随者可能会缺少一些日志条目（a-b），可能会有一些未被提交的日志条目（c-d），或者两种情况都存在（e-f）。例如，场景 f 可能会这样发生，某服务器在任期 2 的时候是领导者，已附加了一些日志条目到自己的日志中，但在提交之前就崩溃了；很快这个机器就被重启了，在任期 3 重新被选为领导者，并且又增加了一些日志条目到自己的日志中；在任期 2 和任期 3 的日志被提交之前，这个服务器又宕机了，并且在接下来的几个任期里一直处于宕机状态。



&emsp;&emsp;在 Raft 算法中，`领导者处理不一致是通过强制跟随者直接复制自己的日志来解决了。`这意味着在`跟随者中的冲突的日志条目会被领导者的日志覆盖。`第 3.6 节会阐述如何通过增加一些限制来使得这样的操作是安全的。




&emsp;&emsp;要使得跟随者的日志进入和自己一致的状态，领导者必须找到最后两者达成一致的地方，然后删除从那个点之后的所有日志条目，发送自己的日志给跟随者。所有的这些操作都在进行附加日志 RPCs 的一致性检查时完成。`领导者针对每一个跟随者维护了一个 nextIndex，这表示下一个需要发送给跟随者的日志条目的索引地址。`当一个领导者刚获得权力的时候，`他初始化所有的 nextIndex 值为自己的最后一条日志的 index 加 1（图 3.6 中的 11）。如果一个跟随者的日志和领导者不一致，那么在下一次的附加日志 RPC 时的一致性检查就会失败。在被跟随者拒绝之后，领导者就会减小 nextIndex 值并进行重试。最终 nextIndex 会在某个位置使得领导者和跟随者的日志达成一致。当这种情况发生，附加日志 RPC 就会成功，这时就会把跟随者冲突的日志条目全部删除并且加上领导者的日志。一旦附加日志 RPC 成功，那么跟随者的日志就会和领导者保持一致，并且在接下来的任期里一直继续保持。`



&emsp;&emsp;在领导者发现它与跟随者的日志匹配位置之前，领导者可以发送不带任何条目（例如心跳）的附加日志 RPCs 以节省带宽。 然后，`一旦 matchIndex 恰好比 nextIndex 小 1，则领导者应开始发送实际条目。`



&emsp;&emsp;如果需要的话，算法可以通过减少被拒绝的附加日志 RPCs 的次数来优化。例如，`当附加日志 RPC 的请求被拒绝的时候，`跟随者可以包含冲突的条目的任期号和自己存储的那个任期的最早的索引地址。借助这些信息，`领导者可以减小 nextIndex 越过所有那个任期冲突的所有日志条目`；这样就变成每个任期需要一次附加条目 RPC 而不是每个条目一次。在实践中，`我们十分怀疑这种优化是否是必要的`，因为失败是很少发生的并且也不大可能会有这么多不一致的日志。



&emsp;&emsp;通过这种机制，领导者在获得权力的时候就不需要任何特殊的操作来恢复一致性。他只需要进行正常的操作，然后日志就能自动的在回复附加日志 RPC 的一致性检查失败的时候自动趋于一致。`领导者从来不会覆盖或者删除自己的日志（图 3.2 的领导者只附加特性）。`


&emsp;&emsp;日志复制机制展示出了第 2.1 节中形容的一致性特性：Raft 能够接受，复制并应用新的日志条目只要大部分的机器是工作的；在通常的情况下，新的日志条目可以在一次 RPC 中被复制给集群中的大多数机器；并且单个的缓慢的跟随者不会影响整体的性能。由于附加日志 RPCs 请求的大小是可管理的（领导者无需在单个附加日志 RPC 请求中发送多个条目来赶进度），所以日志复制算法也是容易实现的。一些其他的一致性算法的描述中需要通过网络发送整个日志，这对开发者增加了优化此点的负担。


### 我的总结

&emsp;&emsp;在leader节点中，会记录所有跟随者的NextIndex和MacthIndex


- nextIndex 对于每个节点，待发送到该节点的下一个日志条目的索引，初值为领导人最后的日志条目索引 + 1
- matchIndex 对于每个节点，已知的已经同步到该节点的最高日志条目的索引，初值为0，表示没有


![在这里插入图片描述](https://img-blog.csdnimg.cn/ddfcca05ad4541c5a770f7ee56c0dc72.png)
&emsp;&emsp;在新上任的leader的时候，会默认发送一条noop空日志，用于快速提交之前任期的日志。但是它不会通过计算副本的方式去提交以前的日志，而是通过这个空日志去提交。

&emsp;&emsp;那么日志复制可能会出现冲突，比如网络分区，宕机等。如何保证日志的一致？leader就是一切，就相信leader（为什么相信leader在下面选举限制会写）。所以leader会把follower冲突的日志覆盖掉。


&emsp;&emsp;leader怎么知道哪些日志冲突了？通过nextIndex，在发送日志复制的时候，发送nextIndex的前一条日志的idx和term。与follower的日志去对比


&emsp;&emsp;如果不一样，说明肯定有日志冲突，那么简单的做法是直接拒绝，ledaer那边nextIndex--，然后重新发

&emsp;&emsp;但是效率低，论文里提出了跳过这个冲突日志所有条目，从上一个日志开始，再看看一致吗，不一致被拒绝了再跳过，直到匹配成功

&emsp;&emsp;终于找到了prevLogIdx的位置了，那么就同意这个日志，返回给leader，leader就可以更新matchIndex了，那么nextIndex=matchIndex+1，然后发送剩下没发完的日志



![在这里插入图片描述](https://img-blog.csdnimg.cn/10f455c3d070450e8614eddb6f00ce54.png)

![在这里插入图片描述](https://img-blog.csdnimg.cn/7a006466f3a748e2a2d532d23e7b74e3.png)
![在这里插入图片描述](https://img-blog.csdnimg.cn/898823738f7441af9949fff434b4fbfa.png)

&emsp;&emsp;这里可能有读者会有疑问，上面不是说的是nextIndex 吗，怎么现在用的都是nextIndex 的前一个prevlogindex？

&emsp;&emsp;nextIndex 代表的是待发送到该节点的日志的索引，而prev呢？是用来保证日志一致性，是用来匹配的，只有prev匹配成功了，才能开始日志复制。否则日志都不一致，还复制啥？




## 选举限制

### 再看一遍论文


&emsp;&emsp;在任何基于领导者的一致性算法中，`领导者都必须存储所有已经提交的日志条目。`在某些一致性算法中，例如 Viewstamped Replication，某个节点即使是一开始并没有包含所有已经提交的日志条目，它也能被选为领导者。这些算法都包含一些额外的机制来识别丢失的日志条目并把他们传送给新的领导者，要么是在选举阶段要么在之后很快进行。不幸的是，这种方法会导致相当大的额外的机制和复杂性。`Raft 使用了一种更加简单的方法，它可以保证所有之前的任期号中已经提交的日志条目在选举的时候都会出现在新的领导者中`，不需要传送这些日志条目给领导者。`这意味着日志条目的传送是单向的，只从领导者传给跟随者，并且领导者从不会覆盖自身本地日志中已经存在的条目。`




&emsp;&emsp;`Raft 使用投票的方式来阻止一个候选人赢得选举除非这个候选人包含了所有已经提交的日志条目`。候选人为了赢得选举必须联系集群中的大部分节点，`这意味着每一个已经提交的日志条目在这些服务器节点中肯定存在于至少一个节点上。如果候选人的日志至少和大多数的服务器节点一样新（这个新的定义会在下面讨论），那么他一定持有了所有已经提交的日志条目。`请求投票 RPC 实现了这样的限制： RPC 中包含了候选人的日志信息，然后投票人会拒绝掉那些日志没有自己新的投票请求。



&emsp;&emsp;`Raft 通过比较两份日志中最后一条日志条目的索引值和任期号定义谁的日志比较新。如果两份日志最后的条目的任期号不同，那么任期号大的日志更加新。如果两份日志最后的条目任期号相同，那么日志比较长的那个就更加新。`

### 我的总结

&emsp;&emsp;你投票赞同的人，一定是日志比你新的人，如果不比你新，即使任期term比你大，也是拒绝的。只不过遇到任期比自己大的要变成follower



&emsp;&emsp;来看下面的例子，如果搞明白了，选举限制也就明白了，下面5台机器就S1还存活，candidate选举了11次了，现在term是11


![在这里插入图片描述](https://img-blog.csdnimg.cn/7a471fbacb5349dea8d129fde913d0ab.png)
&emsp;&emsp;现在S1宕机，别的机器日志都是最新的

![在这里插入图片描述](https://img-blog.csdnimg.cn/fff6769bbd8b4cc29ed78cd315cbb005.png)
&emsp;&emsp;恢复S1，由于S2接收到S1的心跳包回复，发现他的term比自己大，进入follower

![在这里插入图片描述](https://img-blog.csdnimg.cn/732f10fa7e5648fb9514e4bc9974fbec.png)

&emsp;&emsp;S1重启之后，超时，开始选举了
![在这里插入图片描述](https://img-blog.csdnimg.cn/92fe63d7cbd64baeb60ca6aa6cbd7a8d.png)
&emsp;&emsp;接收到投票请求，发现term比自己大，都进入follower状态，更新term。由于日志没有自己新，投否决票。
![在这里插入图片描述](https://img-blog.csdnimg.cn/2894b47c29384792a5e47559899bc31a.png)

&emsp;&emsp;现在S3开始选举投票，自增term
![在这里插入图片描述](https://img-blog.csdnimg.cn/50d42ac6aec641eba00c19b0a795dfd0.png)
&emsp;&emsp;因为term比别人大，所以别人进入follower，而又因为日志新（至少不比别人旧），所以别人都投赞同票
![在这里插入图片描述](https://img-blog.csdnimg.cn/232c4387291048618e31ab9fd410dcfd.png)

&emsp;&emsp;成为leader后，把日志复制给S1
![在这里插入图片描述](https://img-blog.csdnimg.cn/17193972676d4f3197ddce8cf352fdac.png)


## 提交之前任期内的日志条目


### 再看一遍论文



&emsp;&emsp;如同 3.5 节介绍的那样，领导者知道一条当前任期内的日志记录是可以被提交的，只要它被存储到了大多数的服务器上。如果一个领导者在提交日志条目之前崩溃了，未来后续的领导者会继续尝试复制这条日志记录。然而，一个领导者不能断定一个之前任期里的日志条目被保存到大多数服务器上的时候就一定已经提交了。图 3.7 展示了一种情况，一条已经被存储到大多数节点上的老日志条目，也依然有可能会被未来的领导者覆盖掉。

![在这里插入图片描述](https://img-blog.csdnimg.cn/3b6d8c95de3143fe9bc88eda41afc166.png)

> 图 3.7：如图的时间序列展示了为什么领导者无法决定对老任期号的日志条目进行提交。在 (a) 中，S1 是领导者，部分的复制了索引位置 2 的日志条目。在 (b) 中，S1 崩溃了，然后 S5 在任期 3 里通过 S3、S4 和自己的选票赢得选举，然后从客户端接收了一条不一样的日志条目放在了索引 2 处。然后到 (c)，S5 又崩溃了；S1 重新启动，选举成功，开始复制日志。在这时，来自任期 2 的那条日志已经被复制到了集群中的大多数机器上，但是还没有被提交。如果 S1 在 (d1) 中又崩溃了，S5 可以重新被选举成功（通过来自 S2，S3 和 S4 的选票），然后覆盖了他们在索引 2 处的日志。反之，如果在崩溃之前，S1 把自己主导的新任期里产生的日志条目复制到了大多数机器上，就如 (d2) 中那样，那么在后面任期里面这些新的日志条目就会被提交（因为 S5 就不可能选举成功）。 这样在同一时刻就同时保证了，之前的所有老的日志条目就会被提交。




&emsp;&emsp;为了消除图 3.7 里描述的情况，`Raft 永远不会通过计算副本数目的方式去提交一个之前任期内的日志条目。只有领导者当前任期里的日志条目通过计算副本数目可以被提交；一旦当前任期的日志条目以这种方式被提交，那么由于日志匹配特性，之前的日志条目也都会被间接的提交。`在某些情况下，领导者可以安全的知道一个老的日志条目是否已经被提交（例如，该条目是否存储到所有服务器上），但是 Raft 为了简化问题使用一种更加保守的方法。



&emsp;&emsp;当领导者复制之前任期里的日志时，Raft 会为所有日志保留原始的任期号, 这在提交规则上产生了额外的复杂性。在其他的一致性算法中，如果一个新的领导者要重新复制之前的任期里的日志时，它必须使用当前新的任期号。Raft 使用的方法更加容易辨别出日志，因为它可以随着时间和日志的变化对日志维护着同一个任期编号。另外，和其他的算法相比，Raft 中的新领导者只需要发送更少日志条目（其他算法中必须在他们被提交之前发送更多的冗余日志条目来为他们重新编号）。但是，这在实践中可能并不十分重要，因为领导者更换很少


### 我的总结

&emsp;&emsp;其实上面的核心就是，`Raft 永远不会通过计算副本数目的方式去提交一个之前任期内的日志条目。只有领导者当前任期里的日志条目通过计算副本数目可以被提交；`


&emsp;&emsp;为什么不计算之前任期副本数量？因为可能会发生上面论文的情况（我反正没看懂）



&emsp;&emsp;`一旦当前任期的日志条目以这种方式被提交，那么由于日志匹配特性，之前的日志条目也都会被间接的提交。`


&emsp;&emsp;因为上面日志复制里面写过的日志匹配限制，之前的日志会被间接的提交。因为leader是日志就是最新的，不管别的。为什么leader是最新的？上面选举限制解释过了，在选举的时候大多数人都认为我是最新的，所以我leader就是最新的。如果你日志与我不匹配，那你就被我覆盖就好了。





# 参考


- [raft中文论文](https://github.com/OneSizeFitsQuorum/raft-thesis-zh_cn/blob/master/raft-thesis-zh_cn.md) ps：别人的总结始终是没有论文详细的，真想搞懂需要多读论文


- [sakura-ysy project2.md](https://github.com/sakura-ysy/TinyKV-2022-doc/blob/main/doc/project2.md) ps：本文很多都是CV来自该文，感谢这位作者，有些不懂的内容还是请教该作者的


- [Smith-Cruise Project2-RaftKV.md](https://github.com/Smith-Cruise/TinyKV-White-Paper/blob/main/Project2-RaftKV.md) ps：git上star最多的project文档



- [Raft算法原理](https://www.codedump.info/post/20180921-raft/#raft%E7%AE%97%E6%B3%95%E6%A6%82%E8%BF%B0) ps：写的有点浅，需要结合论文去看。etcd写的很好

- [raft动态演示](https://raft.github.io/) ps：通过这个网站可以非常直观的观察raft协议集群的工作
