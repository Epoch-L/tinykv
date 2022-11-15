# 前言
&emsp;&emsp;project2c 在 project2b 的基础上完成集群的快照功能。分为五个部分：日志压缩，快照生成，快照分发，快照接收，快照应用


# Project2 PartC RaftKV 文档翻译



&emsp;&emsp;就目前你的代码来看，对于一个长期运行的服务器来说，永远记住完整的Raft日志是不现实的。相反，服务器会检查Raft日志的数量，并不时地丢弃超过阈值的日志。

&emsp;&emsp;在这一部分，你将在上述两部分实现的基础上实现快照处理。一般来说，Snapshot 只是一个像 AppendEntries 一样的 Raft 消息，用来复制数据给 Follower，不同的是它的大小，Snapshot 包含了某个时间点的整个状态机数据，一次性建立和发送这么大的消息会消耗很多资源和时间，可能会阻碍其他 Raft 消息的处理，为了避免这个问题，Snapshot 消息会使用独立的连接，把数据分成几块来传输。这就是为什么TinyKV 服务有一个快照 RPC API 的原因。如果你对发送和接收的细节感兴趣，请查看 snapRunner 和参考资料 https://pingcap.com/blog-cn/tikv-source-code-reading-10/

&emsp;&emsp;你所需要修改的是基于 Part A 和Part B 的代码。


**==在Raft中实现==**


&emsp;&emsp;尽管我们需要对快照信息进行一些不同的处理，但从 Raft 算法的角度来看，应该没有什么区别。请看 proto 文件中 eraftpb.Snapshot 的定义，eraftpb.Snapshot 的数据字段并不代表实际的状态机数据，而是一些元数据，用于上层应用，你可以暂时忽略它。当领导者需要向跟随者发送快照消息时，它可以调用 Storage.Snapshot() 来获取 eraftpb.Snapshot ，然后像其他 raft 消息一样发送快照消息。状态机数据如何实际建立和发送是由 raftstore 实现的，它将在下一步介绍。你可以认为，一旦Storage.Snapshot() 成功返回，Raft 领导者就可以安全地将快照消息发送给跟随者，跟随者应该调用 handleSnapshot 来处理它，即只是从消息中的eraftpb.SnapshotMetadata 恢复 Raft 的内部状态，如term、commit index和成员信息等，之后快照处理的过程就结束了。



**==在raftstore中实现==**


&emsp;&emsp;在这一步，你需要学习 raftstore 的另外两个Worker : raftlog-gc Worker 和 region Worker。

&emsp;&emsp;Raftstore 根据配置 RaftLogGcCountLimit 检查它是否需要 gc 日志，见 onRaftGcLogTick()。如果是，它将提出一个 Raft admin 命令 CompactLogRequest，它被封装在 RaftCmdRequest 中，就像 project2 的 Part B 中实现的四种基本命令类型（Get/Put/Delete/Snap）。但与Get/Put/Delete/Snap命令写或读状态机数据不同，CompactLogRequest 是修改元数据，即更新RaftApplyState 中的 RaftTruncatedState。之后，你应该通过ScheduleCompactLog 给 raftlog-gc worker 安排一个任务。Raftlog-gc worker 将以异步方式进行实际的日志删除工作。

&emsp;&emsp;然后，由于日志压缩，Raft 模块可能需要发送一个快照。PeerStorage 实现了Storage.Snapshot()。TinyKV 生成快照并在 Region Worker 中应用快照。当调用Snapshot() 时，它实际上是向 Region Worker 发送一个任务 RegionTaskGen。region worker 的消息处理程序位于 kv/raftstore/runner/region_task.go 中。它扫描底层引擎以生成快照，并通过通道发送快照元数据。在下一次 Raft 调用 Snapshot时，它会检查快照生成是否完成。如果是，Raft应该将快照信息发送给其他 peer，而快照的发送和接收工作则由 kv/storage/raft_storage/snap_runner.go 处理。你不需要深入了解这些细节，只需要知道快照信息在收到后将由 onRaftMsg 处理。

&emsp;&emsp;然后，快照将反映在下一个Raft ready中，所以你应该做的任务是修改 Raft ready 流程以处理快照的情况。当你确定要应用快照时，你可以更新 peer storage 的内存状态，如 RaftLocalState、RaftApplyState 和 RegionLocalState。另外，不要忘记将这些状态持久化到 kvdb 和 raftdb，并从 kvdb 和 raftdb 中删除陈旧的状态。此外，你还需要将 PeerStorage.snapState 更新为 snap.SnapState_Applying，并通过PeerStorage.regionSched 将 runner.RegionTaskApply 任务发送给 region worker，等待 region worker 完成。

&emsp;&emsp;你应该运行make project2c来通过所有的测试。


# raft节点如何自动的compact压缩自己的entries日志

1. 在 `HandleMsg()` 中收到 `message.MsgTypeTick`，会调用 `onTick()` 函数触发 `d.onRaftGCLogTick()`，当发现`满足 compact 条件`时，创建一个 `CompactLogRequest` 通过 `d.proposeRaftCommand()` 提交。

```go
func (d *peerMsgHandler) HandleMsg(msg message.Msg) {
	switch msg.Type {
	case message.MsgTypeTick:
		d.onTick()
	}
}

func (d *peerMsgHandler) onTick() {
	if d.ticker.isOnTick(PeerTickRaftLogGC) {
		d.onRaftGCLogTick()
	}
}

func (d *peerMsgHandler) onRaftGCLogTick() {
	if appliedIdx > firstIdx && appliedIdx-firstIdx >= d.ctx.cfg.RaftLogGcCountLimit {
		compactIdx = appliedIdx
	}
	request := newCompactLogRequest(regionID, d.Meta, compactIdx, term)
	d.proposeRaftCommand(request, nil)
}

//将 client 的请求包装成 entry 传递给 raft 层
func (d *peerMsgHandler) proposeRaftCommand(msg *raft_cmdpb.RaftCmdRequest, cb *message.Callback) {
	// Your Code Here (2B).
	if msg.Requests != nil {
		d.proposeRequest(msg, cb)
	} else {
		d.proposeAdminRequest(msg, cb)
	}
}
```


2. 像普通请求一样，将 `CompactLogRequest` 请求 propose 到 Raft 中，等待 Raft Group 确认。
3. Commit 后，在 `HandleRaftReady()` 中开始 apply `CompactLogRequest` 请求。此时你可以修改自己 `RaftApplyState` 中的`RaftTruncatedState` 属性。

```go
// processAdminRequest 处理 commit 的 Admin Request 类型 command
func (d *peerMsgHandler) processAdminRequest(entry *pb.Entry, requests *raft_cmdpb.RaftCmdRequest, kvWB *engine_util.WriteBatch) *engine_util.WriteBatch {
	adminReq := requests.AdminRequest
	switch adminReq.CmdType {
	case raft_cmdpb.AdminCmdType_CompactLog: // CompactLog 类型请求不需要将执行结果存储到 proposal 回调
		// 记录最后一条被截断的日志（快照中的最后一条日志）的索引和任期
		if adminReq.CompactLog.CompactIndex > d.peerStorage.applyState.TruncatedState.Index {
			truncatedState := d.peerStorage.applyState.TruncatedState
			truncatedState.Index, truncatedState.Term = adminReq.CompactLog.CompactIndex, adminReq.CompactLog.CompactTerm
			// 调度日志截断任务到 raftlog-gc worker
			d.ScheduleCompactLog(adminReq.CompactLog.CompactIndex)
			log.Infof("%d apply commit, entry %v, type %s, truncatedIndex %v", d.peer.PeerId(), entry.Index, adminReq.CmdType, adminReq.CompactLog.CompactIndex)
		}
	}
	return kvWB
}
```


4. 调用 `d.ScheduleCompactLog()` 发送 `RaftLogGCTask` 任务。

```go
func (d *peerMsgHandler) ScheduleCompactLog(truncatedIndex uint64) {
	raftLogGCTask := &runner.RaftLogGCTask{
		RaftEngine: d.ctx.engine.Raft,
		RegionID:   d.regionId,
		StartIdx:   d.LastCompactedIdx,
		EndIdx:     truncatedIndex + 1,
	}
	d.LastCompactedIdx = raftLogGCTask.EndIdx
	d.ctx.raftLogGCTaskSender <- raftLogGCTask
}
```

5. `raftlog_gc.go` 里面收到 `RaftLogGCTask `并在 `gcRaftLog()` 函数中处理。它会清除 raftDB 中已经被持久化的 entries，因为已经被 compact 了，所以不需要了。

```go
// gcRaftLog does the GC job and returns the count of logs collected.
func (r *raftLogGCTaskHandler) gcRaftLog(raftDb *badger.DB, regionId, startIdx, endIdx uint64) (uint64, error) {

	raftWb := engine_util.WriteBatch{}
	for idx := firstIdx; idx < endIdx; idx += 1 {
		key := meta.RaftLogKey(regionId, idx)
		raftWb.DeleteMeta(key)
	}

	return endIdx - firstIdx, nil
}
```

6. 至此所有节点已经完成了日志的截断，以此确保 raftDB 中的 entries 不会无限制的占用空间（毕竟已经被 apply 了，留着也没多大作用）。
7. 在DB中已经完成了日志的压缩，那么在内存中，也就可以丢弃被压缩过的日志了

```go
// Advance通知RawNode，应用程序已经应用并保存了最后一个Ready结果中的进度。
func (rn *RawNode) Advance(rd Ready) {
	// Your Code Here (2A).

	rn.Raft.RaftLog.maybeCompact()        //丢弃被压缩的暂
}

// We need to compact the log entries in some point of time like
// storage compact stabled log entries prevent the log entries
// grow unlimitedly in memory
//我们需要在某个时间点压缩日志条目，例如
//存储压缩稳定日志条目阻止日志条目在内存中无限增长
func (l *RaftLog) maybeCompact() {
	// Your Code Here (2C).
	newFirst, _ := l.storage.FirstIndex()
	if newFirst > l.dummyIndex {
		//为了GC原来的 所以append
		entries := l.entries[newFirst-l.dummyIndex:]
		l.entries = make([]pb.Entry, 0)
		l.entries = append(l.entries, entries...)
	}
	l.dummyIndex = newFirst
}
```

# 生成快照与快照收收发

1. 当 Leader append 日志给落后 node 节点时，发现对方所需要的 entry 已经被 compact。此时 Leader 会发送 Snapshot 过去。

```go
// sendAppend sends an append RPC with new entries (if any) and the
// current commit index to the given peer. Returns true if a message was sent.
// sendAppend发送一个带有新条目（如果有）的append RPC
// 给跟随者的当前提交索引。如果如果发送了消息，则返回true。
func (r *Raft) sendAppend(to uint64) bool {
	// Your Code Here (2A).
	// 有错误，说明 nextIndex 存在于快照中，此时需要发送快照给 followers
	// 2C
	r.sendSnapshot(to)
	log.Infof("[Snapshot Request]%d to %d, prevLogIndex %v, dummyIndex %v", r.id, to, prevLogIndex, r.RaftLog.dummyIndex)

	return false
}
```
2. 当 Leader 需要发送 Snapshot 时，调用 `r.RaftLog.storage.Snapshot()` 生成 Snapshot。因为 Snapshot 很大，不会马上生成，这里为了避免阻塞，如果 Snapshot 还没有生成好，Snapshot 会先返回 `raft.ErrSnapshotTemporarilyUnavailable` 错误，Leader 就应该放弃本次 Snapshot，等待下一次再次请求 Snapshot。

```go
// sendSnapshot 发送快照给别的节点
func (r *Raft) sendSnapshot(to uint64) {
	snapshot, err := r.RaftLog.storage.Snapshot()
	if err != nil {
		// 生成 Snapshot 的工作是由 region worker 异步执行的，如果 Snapshot 还没有准备好
		// 此时会返回 ErrSnapshotTemporarilyUnavailable 错误，此时 leader 应该放弃本次 Snapshot Request
		// 等待下一次再请求 storage 获取 snapshot（通常来说会在下一次 heartbeat response 的时候发送 snapshot）
		return
	}
	r.msgs = append(r.msgs, pb.Message{
		MsgType:  pb.MessageType_MsgSnapshot,
		From:     r.id,
		To:       to,
		Term:     r.Term,
		Snapshot: &snapshot,
	})
	r.Prs[to].Next = snapshot.Metadata.Index + 1
}
```

3. 生成 Snapshot 请求是异步的，通过发送 `RegionTaskGen` 到 `region_task.go` 中处理，会异步的生成 Snapshot。

```go
func (ps *PeerStorage) Snapshot() (eraftpb.Snapshot, error) {

	// schedule snapshot generate task
	ps.regionSched <- &runner.RegionTaskGen{
		RegionId: ps.region.GetId(),
		Notifier: ch,
	}
	return snapshot, raft.ErrSnapshotTemporarilyUnavailable
}

```


4. 下一次 Leader 请求 `Snapshot()` 时，因为已经创建完成，直接拿出来，包装为 `pb.MessageType_MsgSnapshot` 发送至目标节点。
5. 发送 Snapshot 并不是和普通发送消息一样。在 `peer_msg_handler.go` 中通过 `Send()` 发送，这个函数最后会调用 `transport.go` 中的 `Send()`，你根据方法链调用，看到在 `WriteData()` 函数中，Snapshot 是通过 `SendSnapshotSock()` 方法发送。后面它就会将 Snapshot 切成小块，发送到目标 RaftStore 上面去。这里不细说，太复杂了。

```go
func (d *peerMsgHandler) HandleRaftReady() {
	//3. 调用 d.Send() 方法将 Ready 中的 Msg 发送出去；
	d.Send(d.ctx.trans, ready.Messages)
}
```

6. 目标 RaftStore 在 `server.go` 中的 `Snapshot()` 接收发送过来的 Snapshot。之后生成一个 `recvSnapTask` 请求到 `snapWorker`中。`snap_runner.go` 中收到 `recvSnapTask` 请求，开始下载发送过来的 Snapshot 并保存在本地（此时还没应用），同时生成 `pb.MessageType_MsgSnapshot` 发送到要接收的 peer 上。
7. 目标 peer 会在 `OnRaftMsg()` 中像处理普通 msg 一样，将 `pb.MessageType_MsgSnapshot` 通过 `Step()` 输入 RawNode。
8. Raft 中则根据 Snapshot 中的 metadata 更新当前自己的各种状态并设置 `pendingSnapshot`，然后返回一个 `MessageType_MsgAppendResponse` 给 Leader。

```go
// handleSnapshot handle Snapshot RPC request
//从 SnapshotMetadata 中恢复 Raft 的内部状态，例如 term、commit、membership information
func (r *Raft) handleSnapshot(m pb.Message) {
	// Your Code Here (2C).
	resp := pb.Message{
		MsgType: pb.MessageType_MsgAppendResponse,
		From:    r.id,
		Term:    r.Term,
	}
	meta := m.Snapshot.Metadata

	// 1. 如果 term 小于自身的 term 直接拒绝这次快照的数据
	if m.Term < r.Term {
		resp.Reject = true
	} else if r.RaftLog.committed >= meta.Index {
		// 2. 如果已经提交的日志大于等于快照中的日志，也需要拒绝这次快照
		// 因为 commit 的日志必定会被 apply，如果被快照中的日志覆盖的话就会破坏一致性
		resp.Reject = true
		resp.Index = r.RaftLog.committed
	} else {
		// 3. 需要安装日志
		r.becomeFollower(m.Term, m.From)
		// 更新日志数据
		r.RaftLog.dummyIndex = meta.Index + 1
		r.RaftLog.committed = meta.Index
		r.RaftLog.applied = meta.Index
		r.RaftLog.stabled = meta.Index
		r.RaftLog.pendingSnapshot = m.Snapshot
		r.RaftLog.entries = make([]pb.Entry, 0)
		// 更新集群配置
		r.Prs = make(map[uint64]*Progress)
		for _, id := range meta.ConfState.Nodes {
			r.Prs[id] = &Progress{Next: r.RaftLog.LastIndex() + 1}
		}
		// 更新 response，提示 leader 更新 nextIndex
		resp.Index = meta.Index
	}
	r.msgs = append(r.msgs, resp)
}
```

9. 接收节点在 `HandleRaftReady()`→`SaveReadyState()`→`ApplySnapshot()` 中根据 `pendingSnapshot` 的 Metadata 更新自己的 `RaftTruncatedState` 和 `RaftLocalState`，然后发送一个 `RegionTaskApply` 请求到 `region_task.go` 中。此时它会异步的把刚刚收到保存的 Snapshot 应用到 kvDB 中。

```go
// Apply the peer with given snapshot
// 应用一个快照之后 RaftLog 里面只包含了快照中的日志，并且快照中的数据都是已经被应用了的

func (ps *PeerStorage) ApplySnapshot(snapshot *eraftpb.Snapshot, kvWB *engine_util.WriteBatch, raftWB *engine_util.WriteBatch) (*ApplySnapResult, error) {
	log.Infof("%v begin to apply snapshot", ps.Tag)
	snapData := new(rspb.RaftSnapshotData)
	if err := snapData.Unmarshal(snapshot.Data); err != nil {
		return nil, err
	}

	// Hint: things need to do here including: update peer storage state like raftState and applyState, etc,
	// and send RegionTaskApply task to region worker through ps.regionSched, also remember call ps.clearMeta
	// and ps.clearExtraData to delete stale data
	//提示：这里需要做的事情包括：更新对等存储状态，如raftState和applyState等，
	//并通过ps.regionSched将RegionTaskApply任务发送给区域工作者，还请记住调用ps.clearMeta
	//和ps.clearExtraData删除过时数据
	// Your Code Here (2C).

	// 1. 删除过时数据
	if ps.isInitialized() {
		ps.clearMeta(kvWB, raftWB)
		ps.clearExtraData(snapData.Region)
	}
	// 2. 更新 peer_storage 的内存状态，包括：
	// (1). RaftLocalState: 已经「持久化」到DB的最后一条日志设置为快照的最后一条日志
	// (2). RaftApplyState: 「applied」和「truncated」日志设置为快照的最后一条日志
	// (3). snapState: SnapState_Applying
	ps.raftState.LastIndex, ps.raftState.LastTerm = snapshot.Metadata.Index, snapshot.Metadata.Term
	ps.applyState.AppliedIndex = snapshot.Metadata.Index
	ps.applyState.TruncatedState.Index, ps.applyState.TruncatedState.Term = snapshot.Metadata.Index, snapshot.Metadata.Term
	ps.snapState.StateType = snap.SnapState_Applying
	if err := kvWB.SetMeta(meta.ApplyStateKey(ps.region.Id), ps.applyState); err != nil {
		log.Panic(err)
	}
	// 3. 发送 runner.RegionTaskApply 任务给 region worker，并等待处理完毕
	ch := make(chan bool, 1)
	ps.regionSched <- &runner.RegionTaskApply{
		RegionId: ps.region.Id,
		Notifier: ch,
		SnapMeta: snapshot.Metadata,
		StartKey: snapData.Region.GetStartKey(),
		EndKey:   snapData.Region.GetEndKey(),
	}
	<-ch
	log.Infof("%v end to apply snapshot, metaDataIndex %v, truncatedStateIndex %v", ps.Tag, snapshot.Metadata.Index, ps.applyState.TruncatedState.Index)
	result := &ApplySnapResult{PrevRegion: ps.region, Region: snapData.Region}
	meta.WriteRegionState(kvWB, snapData.Region, rspb.PeerState_Normal)
	return result, nil
}
```


10. 最后在 `Advance()` 的时候，清除 `pendingSnapshot`。至此，整个 Snapshot 接收流程结束。


&emsp;&emsp;可以看到 Snapshot 的 msg 发送不像传统的 msg，因为 Snapshot 通常很大，如果和普通方法一样发送，会占用大量的内网宽带。同时如果你不切片发送，中间一旦一部分失败了，就全功尽气。这个迅雷的切片下载一个道理。




**==ApplySnapshot==**


1. 负责应用 Snapshot，先通过 `ps.clearMeta` 和 `ps.clearExtraData()` 清除原来的数据，因为新的 snapshot 会包含新的 meta 信息，需要先清除老的。
2. 根据 Snapshot 的 Metadata 信息更新当前的 raftState 和 applyState。并保存信息到 WriteBatch 中，可以等到 `SaveReadyState()` 方法结束时统一写入 DB。
3. 发送 `RegionTaskApply` 到 `regionSched` 安装 snapshot，因为 Snapshot 很大，所以这里通过异步的方式安装。我这里是等待 Snapshot 安装完成后才继续执行，相当于没异步，以防出错。



# 日志压缩与快照收发总结


日志压缩：

1. 日志压缩和生成快照是两步。每当节点的 entries 满足一定条件时，就会触发日志压缩。
2. 在 HandleMsg( ) 会中收到 message.MsgTypeTick，然后进入 onTick( ) ，触发 d.onRaftGCLogTick( ) 方法。这个方法会检查 appliedIdx - firstIdx >= d.ctx.cfg.RaftLogGcCountLimit，即未应用的 entry 数目是否大于等于你的配置。如果是，就开始进行压缩。
3. 该方法会通过 proposeRaftCommand( ) 提交一个 AdminRequest 下去，类型为 AdminCmdType_CompactLog。然后 proposeRaftCommand( ) 就像处理其他 request 一样将该 AdminRequest 封装成 entry 交给 raft 层来同步。
4. 当该 entry 需要被 apply 时，HandleRaftReady( ) 开始执行该条目中的命令 。这时会首先修改相关的状态，然后调用 d.ScheduleCompactLog( ) 发送 raftLogGCTask 任务给 raftlog_gc.go。raftlog_gc.go 收到后，会删除 raftDB 中对应 index 以及之前的所有已持久化的 entries，以实现压缩日志。




快照生成与快照分发：

1. 当 Leader 发现要发给 Follower 的 entry 已经被压缩时（ Next < r.RaftLog.FirstIndex() )，就会通过 r.RaftLog.storage.Snapshot() 生成一份 Snapshot，并将生成的 Snapshot 发送给对应节点。
2. 因为 Snapshot 有点大，r.RaftLog.storage.Snapshot( ) 不是瞬时完成的，而是异步的，通过发送 RegionTaskGen 到 region_task.go 中处理，会异步地生成 Snapshot。第一次调用 r.RaftLog.storage.Snapshot( ) 时，很可能因为时间原因 Snapshot 未生成完毕，其会返回一个 nil，这时候 leader 需要停止快照发送，然后等到下一次需要发送时再次调用 ，r.RaftLog.storage.Snapshot( ) 生成，这时如果其在之前已经生成好了，直接返回那个 Snapshot，然后 leader 将其发送给 follower。
3. 每个节点有个字段叫 pendingSnapshot，可以理解为待应用的 Snapshot，如果 leader 发快照时pendingSnapshot 有值，那就直接发这个，否者通过第2步生成一个新的发过去。
4. 快照发送的底层实现不用管，TinyKV 已经把 Snapshot 的发送封装成和普通 Msg 那样了，用同样的接口就行。但是底层发送 Snapshot 和发送普通 Msg 的逻辑却不一样，前者像 raft 论文写的那样实现了分块发送，后者就没有。因此这里不需要自己手动实现 Snapshot_Msg 的分块发送。

快照接收：

1. Follower 收到 Leader 发来的 pb.MessageType_MsgSnapshot 之后，会根据其中的 Metadata 来更新自己的 committed、applied、stabled 等等指针。然后将在 Snapshot 之前的 entry 均从 RaftLog.entries 中删除。之后，根据其中的 ConfState 更新自己的 Prs 信息。做完这些操作好，把 pendingSnapshot 置为该 Snapshot，等待 raftNode 通过 Ready( ) 交给 peer 层处理。

快照应用：

1. 在 HandleRaftReady( ) 中，如果收到的 Ready 包含 SnapState，就需要对其进行应用。调用链为HandleRadtReady( ) -> SaveReadyState( ) -> ApplySnaptshot( )
2. 在  ApplySnaptshot( ) 中，首先要通过 ps.clearMeta 和 ps.clearExtraData 来清空旧的数据。然后更新根据 Snapshot 更新 raftState 和 applyState。其中，前者需要把自己的 LastIndex 和 LastTerm 置为 Snapshot.Metadata 中的 Index 和 Term。后者同理需要更改自己的 AppliedIndex 以及 TruncatedState。按照文档的说法，还需要给 snapState.StateType 赋值为 snap.SnapState_Applying
3. 接下来就是将 Snapshot 中的 K-V 存储到底层。这一步不需要我们来完成，只需要生成一个 RegionTaskApply，传递给 ps.regionSched 管道即可。region_task.go 会接收该任务，然后异步的将 Snapshot 应用到 kvDB 中去。


# 疑难杂症


**d.ScheduleCompactLog 是干什么的**

- 该方法用来给 raftlog-gc worker 发送压缩日志的任务，当 raftlog-gc worker 收到后，它将会异步的执行日志压缩，即将 index 之前的所有以持久化日志都删了。

**Snapshot() 生成快照不是瞬时完成的**

- 当 Leader 需要发送 Snapshot 时，调用 r.RaftLog.storage.Snapshot() 生成 Snapshot。因为 Snapshot 很大，不会马上生成，这里为了避免阻塞，生成操作设为异步，如果 Snapshot 还没有生成好，Snapshot 会先返回 raft.ErrSnapshotTemporarilyUnavailable 错误，Leader 就应该放弃本次 Snapshot，等待下一次再次请求 Snapshot。所以这个错误必须处理，否则返回的 Snapshot 就为 nil，在发送快照时就会报错。



**对快照中包含的 K-V 存储交给** **region_task.go** **完成**

- 在 applySnapshot 的最后（更改完状态机），需要创建一个 &runner.RegionTaskApply 交给管道 ps.regionSched。region_task.go 会收取该任务，然后执行底层的 K-V 存储操作，不需要我们手动完成。