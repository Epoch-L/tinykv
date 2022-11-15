# 前言


&emsp;&emsp;Project3是整个项目最难的部分，3a是对3b的铺垫，比较简单，就是对之前的代码增加领导禅让



#  Project3 PartA Multi-raft KV 文档翻译



&emsp;&emsp;在 Project2 中，你建立了一个基于Raft的高可用的kv服务器，做得很好！但还不够，这样的kv服务器是由单一的 raft 组支持的，不能无限扩展，并且每一个写请求都要等到提交后再逐一写入 badger，这是保证一致性的一个关键要求，但也扼杀了任何并发性。


![在这里插入图片描述](https://img-blog.csdnimg.cn/8f25307c0f8e4d6a9ab6ba712360a47e.png)


&emsp;&emsp;在这个项目中，你将实现一个带有平衡调度器的基于 multi Raft 的kv服务器，它由多个 Raft group 组成，每个 Raft group 负责一个单独的 key 范围，在这里被命名为 region ，布局将看起来像上图。对单个 region 的请求的处理和以前一样，但多个 region 可以同时处理请求，这提高了性能，但也带来了一些新的挑战，如平衡每个 region 的请求，等等。


这个项目有3个部分，包括：

- 对 Raft 算法实现成员变更和领导变更
- 在 raftstore 上实现Conf change和 region split
- 引入 scheduler


==PartA==

&emsp;&emsp;在这一部分中，你将在基本的 Raft 算法上实现成员变更和领导者变更，这些功能是后面两部分所需要的。成员变更，即conf变更，用于添加或删除 peer 到Raft Group，这可能会改变 Raft 组的法定人数，所以要小心。领导权变更，即领导权转移，用于将领导权转移给另一个 peer，这对平衡非常有用。

==代码==


&emsp;&emsp;你需要修改的代码都是关于 raft/raft.go 和 raft/rawnode.go 的，也可以参见proto/proto/eraft.proto 以了解你需要处理的新信息。conf change 和 leader transfer 都是由上层程序触发的，所以你可能想从 raft/rawnode.go 开始。



==实现领导者转移==

&emsp;&emsp;为了实现领导者的转移，让我们引入两个新的消息类型。MsgTransferLeader 和MsgTimeoutNow。为了转移领导权，你需要首先在当前领导上调用带有MsgTransferLeader 消息的 raft.Raft.Step，为了确保转移的成功，当前领导应该首先检查被转移者（即转移目标）的资格，比如：被转移者的日志是否为最新的，等等。如果被转移者不合格，当前领导可以选择放弃转移或者帮助被转移者，既然放弃对程序本身没有帮助，就选择帮助被转移者吧。如果被转移者的日志不是最新的，当前的领导者应该向被转移者发送 MsgAppend 消息，并停止接受新的 propose，以防我们最终会出现循环。因此，如果被转移者符合条件（或者在现任领导的帮助下），领导应该立即向被转移者发送 MsgTimeoutNow 消息，在收到 MsgTimeoutNow 消息后，被转移者应该立即开始新的选举，无论其选举超时与否，被转移者都有很大机会让现任领导下台，成为新领导。







==实现成员变更==



&emsp;&emsp;这里要实现的 conf change 算法不是扩展Raft论文中提到的联合共识算法，联合共识算法可以一次性增加和/或移除任意 peer，相反，这个算法只能一个一个地增加或移除 peer，这更简单，更容易推理。此外，Conf Change从调用领导者的raft.RawNode.ProposeConfChange 开始，它将提出一个日志，其中pb.Entry.EntryType 设置为 EntryConfChange，pb.Entry.Data 设置为输入 pb.ConfChange 。当 EntryConfChange 类型的日志被提交时，你必须通过RawNode.ApplyConfChange 与日志中的 pb.ConfChange 一起应用它，只有这样你才能根据 pb.ConfChange 通过 raft.Raft.addNode 和 raft.Raft.removeNode 向这个Raft 子节点添加或删除 peer。


> 提示：
> - MsgTransferLeader 消息是本地消息，不是来自网络的。
> - 将 MsgTransferLeader 消息的 Message.from 设置为被转移者（即转移目标）。
> - 要立即开始新的选举，你可以用 MsgHup 消息调用 Raft.Step
> - 调用 pb.ConfChange.Marshal 来获取 pb.ConfChange 的字节表示，并将其放入 pb.Entry.Data。



# Add / Remove


&emsp;&emsp;在 raft 层中，这两各操作仅仅会影响到 r.Prs[ ]，因此新增节点就加一个，删除节点就少一个。

&emsp;&emsp;需要注意的是，在 removeNode 之后，Leader 要重新计算 committedIndex。正常情况下，节点收到 appendEntry 会返回一个 appendResponse，当 Leader 收到这个 appendResponse 时会视情况更新自己的 committedIndex。但是呢，如果在节点返回 appendResponse 就被 removeNode 了，那么 leader 就不知道本次 entries 的同步情况了，也就不会再去重算 committedIndex。

&emsp;&emsp;在 TestCommitAfterRemoveNode3A 中，节点1（Leader）发送 entry3 给节点2，节点2还没有给回应就被 removeNode 了。此时集群中只剩一个节点，那么 entry3 肯定是算作大多数节点同步了的，理应被纳入 committedIndex 中，但是由于 leader 没有收到 appendResponse ，就不会更新 committedIndex ，导致出错。解决办法是，在 removeNode 之后，直接重算 committedIndex，当然也不能忘了向集群同步。

```go
// addNode add a new node to raft group
func (r *Raft) addNode(id uint64) {
	// Your Code Here (3A).
	if _, ok := r.Prs[id]; !ok {
		r.Prs[id] = &Progress{Next: r.RaftLog.LastIndex() + 1}
		r.PendingConfIndex = None // 清除 PendingConfIndex 表示当前没有未完成的配置更新
	}
}

// removeNode remove a node from raft group
func (r *Raft) removeNode(id uint64) {
	// Your Code Here (3A).
	if _, ok := r.Prs[id]; ok {
		delete(r.Prs, id)
		// 如果是删除节点，由于有节点被移除了，这个时候可能有新的日志可以提交
		// 这是必要的，因为 TinyKV 只有在 handleAppendRequestResponse 的时候才会判断是否有新的日志可以提交
		// 如果节点被移除了，则可能会因为缺少这个节点的回复，导致可以提交的日志无法在当前任期被提交
		if r.State == StateLeader && r.maybeCommit() {
			log.Infof("[removeNode commit] %v leader commit new entry, commitIndex %v", r.id, r.RaftLog.committed)
			r.broadcastAppendEntry() // 广播更新所有 follower 的 commitIndex
		}
	}
	r.PendingConfIndex = None // 清除 PendingConfIndex 表示当前没有未完成的配置更新
}
```


# LeaderTransfer

- 非 Leader 收到 MsgTransfer 之后要移交给 Leader：非 Leader 节点无法进行 LeaderTransfer，但是应该把收到的 MsgTransfer 发送给自己的 Leader，从而保证集群的 Leader 转移。

```go
//follower
case pb.MessageType_MsgTransferLeader:
	//Local Msg，用于上层请求转移 Leader
	//TODO Follower No processing required
	// 非 leader 收到领导权禅让消息，需要转发给 leader
	if r.Lead != None {
		m.To = r.Lead
		r.msgs = append(r.msgs, m)
	}
//candidate
case pb.MessageType_MsgTransferLeader:
	//Local Msg，用于上层请求转移 Leader
	//要求领导转移其领导权
	//TODO Candidate No processing required
	// 非 leader 收到领导权禅让消息，需要转发给 leader
	if r.Lead != None {
		m.To = r.Lead
		r.msgs = append(r.msgs, m)
	}
//leader
case pb.MessageType_MsgTransferLeader:
	//Local Msg，用于上层请求转移 Leader
	//要求领导转移其领导权
	//TODO project3
	r.handleTransferLeader(m)
```

&emsp;&emsp;首先，raft 层多了两种 MsgType 为 MessageType_MsgTransferLeader 和 MessageType_MsgTimeoutNow。上层会发送该消息给 leader，要求其转换 leader，不过 project3a 没有上层，全是在 raft 层内部测试的。当 leader 要转换时，首先需要把 r.leadTransferee 置为 m.From，表明转换操作正在执行。接着，会判断目标节点的日志是否和自己一样新，如果是，就给它发一个 MsgTimeoutNow，如果不是，就先 append 同步日志，然后再发送 MsgTimeoutNow。当节点收到 MsgTimeoutNow 后，立刻开始选举，因为它的日志至少和原 leader 一样新，所以一定会选举成功。当 leader 正在进行转换操作时，所有的 propose 请求均被拒绝。


```go
func (r *Raft) handleTransferLeader(m pb.Message) {
	// 判断 transferee 是否在集群中
	if _, ok := r.Prs[m.From]; !ok {
		return
	}
	// 如果 transferee 就是 leader 自身，则无事发生
	if m.From == r.id {
		return
	}
	// 判断是否有转让流程正在进行，如果是相同节点的转让流程就返回，否则的话终止上一个转让流程
	if r.leadTransferee != None {
		if r.leadTransferee == m.From {
			return
		}
		r.leadTransferee = None
	}
	r.leadTransferee = m.From
	r.transferElapsed = 0
	if r.Prs[m.From].Match == r.RaftLog.LastIndex() {
		// 日志是最新的，直接发送 TimeoutNow 消息
		r.sendTimeoutNow(m.From)
	} else {
		// 日志不是最新的，则帮助 leadTransferee 匹配最新的日志
		r.sendAppend(m.From)
	}
}
```

&emsp;&emsp;在这里看到，如果走的是r.sendAppend(m.From)，那么在什么时候继续进行禅让呢？在接收到日志复制的response的时候继续禅让。

```go
func (r *Raft) handleAppendEntriesResponse(m pb.Message) {
	//...
	
	//3A
	if r.leadTransferee == m.From && r.Prs[m.From].Match == r.RaftLog.LastIndex() {
		// AppendEntryResponse 回复来自 leadTransferee，检查日志是否是最新的
		// 如果 leadTransferee 达到了最新的日志则立即发起领导权禅让
		r.sendTimeoutNow(m.From)
	}
}

func (r *Raft) sendTimeoutNow(to uint64) {
	r.msgs = append(r.msgs, pb.Message{MsgType: pb.MessageType_MsgTimeoutNow, From: r.id, To: to})
}
```
&emsp;&emsp;在接收到timeout后立刻开始进行选举
```go
//follower
case pb.MessageType_MsgTimeoutNow:
	//Local Msg，节点收到后清空 r.electionElapsed，并即刻发起选举
	r.handleTimeoutNowRequest(m)
}

func (r *Raft) handleTimeoutNowRequest(m pb.Message) {
	if _, ok := r.Prs[r.id]; !ok {
		return
	}
	// 直接发起选举
	if err := r.Step(pb.Message{MsgType: pb.MessageType_MsgHup}); err != nil {
		log.Panic(err)
	}
}
```