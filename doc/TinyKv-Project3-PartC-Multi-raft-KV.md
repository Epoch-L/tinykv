# 前言

&emsp;&emsp;3C要求我们实现调度,3c按照文档直接写即可，比较简单。

![在这里插入图片描述](https://img-blog.csdnimg.cn/c426e67e55f14b12bd31b7622f960b69.png)

# Project3 PartC Multi-raft KV 文档翻译

==C部分==

&emsp;&emsp;正如我们上面所介绍的，我们的kv存储中的所有数据被分割成几个 region，每个region 都包含多个副本。一个问题出现了：我们应该把每个副本放在哪里？我们怎样才能找到副本的最佳位置？谁来发送以前的 AddPeer 和 RemovePeer 命令？Scheduler承担了这个责任。

&emsp;&emsp;为了做出明智的决定，Scheduler 应该拥有关于整个集群的一些信息。它应该知道每个 region 在哪里。它应该知道它们有多少个 key。它应该知道它们有多大…为了获得相关信息，Scheduler 要求每个 region 定期向 Scheduler 发送一个心跳请求。你可以在 /proto/proto/schedulerpb.proto 中找到心跳请求结构 RegionHeartbeatRequest。在收到心跳后，调度器将更新本地 region 信息。

&emsp;&emsp;同时，调度器会定期检查 region 信息，以发现我们的 TinyKV 集群中是否存在不平衡现象。例如，如果任何 store 包含了太多的 region，region 应该从它那里转移到其他 store 。这些命令将作为相应 region 的心跳请求的响应被接收。

&emsp;&emsp;在这一部分，你将需要为 Scheduler 实现上述两个功能。按照我们的指南和框架，这不会太难。

==代码==

&emsp;&emsp;需要修改的代码都是关于 scheduler/server/cluster.go 和 scheduler/server/schedulers/balance_region.go 的。如上所述，当调度器收到一个 region 心跳时，它将首先更新其本地 region 信息。然后，它将检查是否有这个 region 的未决命令。如果有，它将作为响应被发送回来。

&emsp;&emsp;你只需要实现 processRegionHeartbeat 函数，其中 Scheduler 更新本地信息；以及平衡- region 调度器的 Scheduler 函数，其中 Scheduler 扫描存储并确定是否存在不平衡以及它应该移动哪个 region。

==收集区域心跳==

&emsp;&emsp;正如你所看到的，processRegionHeartbeat 函数的唯一参数是一个 regionInfo。它包含了关于这个心跳的发送者 region 的信息。Scheduler 需要做的仅仅是更新本地region 记录。但是，它应该为每次心跳更新这些记录吗？

&emsp;&emsp;肯定不是！有两个原因。有两个原因。一个是当这个 region 没有变化时，更新可能被跳过。更重要的一个原因是，Scheduler 不能相信每一次心跳。特别是说，如果集群在某个部分有分区，一些节点的信息可能是错误的。

&emsp;&emsp;例如，一些 Region 在被分割后会重新启动选举和分割，但另一批孤立的节点仍然通过心跳向 Scheduler 发送过时的信息。所以对于一个 Region 来说，两个节点中的任何一个都可能说自己是领导者，这意味着 Scheduler 不能同时信任它们。

&emsp;&emsp;哪一个更可信呢？Scheduler 应该使用 conf_ver 和版本来确定，即 RegionEpoch。Scheduler 应该首先比较两个节点的 Region 版本的值。如果数值相同，Scheduler 会比较配置变化版本的数值。拥有较大配置变更版本的节点必须拥有较新的信息。


简单地说，你可以按以下方式组织检查程序：

- 检查本地存储中是否有一个具有相同 Id 的 region。如果有，并且至少有一个心跳的 conf_ver 和版本小于它，那么这个心跳 region 就是过时的。

- 如果没有，则扫描所有与之重叠的区域。心跳的 conf_ver 和版本应该大于或等于所有的，否则这个 region 是陈旧的。


那么 Scheduler 如何确定是否可以跳过这次更新？我们可以列出一些简单的条件。

- 如果新的版本或 conf_ver 大于原来的版本，就不能被跳过。

- 如果领导者改变了，它不能被跳过

- 如果新的或原来的有挂起的 peer，它不能被跳过。

- 如果近似大小发生变化，则不能跳过。
- ...



&emsp;&emsp;不要担心。你不需要找到一个严格的充分和必要条件。冗余的更新不会影响正确性。

&emsp;&emsp;如果 Scheduler 决定根据这个心跳来更新本地存储，有两件事它应该更新：region tree 和存储状态。你可以使用 RaftCluster.core.PutRegion 来更新 region-tree ，并使用 RaftCluster.core.UpdateStoreStatus 来更新相关存储的状态（如领导者数量、区域数量、待处理的同伴数量…）。


==实现 region balance 调度器==



&emsp;&emsp;在调度器中可以有许多不同类型的调度器在运行，例如，平衡-区域调度器和平衡-领导调度器。这篇学习材料将集中讨论平衡区域调度器。

&emsp;&emsp;每个调度器都应该实现了 Scheduler 接口，你可以在 /scheduler/server/schedule/scheduler.go 中找到它。调度器将使用 GetMinInterval 的返回值作为默认的时间间隔来定期运行 Schedule 方法。如果它的返回值为空（有几次重试），Scheduler 将使用 GetNextInterval 来增加间隔时间。通过定义 GetNextInterval，你可以定义时间间隔的增加方式。如果它返回一个操作符，Scheduler 将派遣这些操作符作为相关区域的下一次心跳的响应。

&emsp;&emsp;Scheduler 接口的核心部分是 Schedule 方法。这个方法的返回值是操作符，它包含多个步骤，如 AddPeer 和 RemovePeer。例如，MovePeer 可能包含 AddPeer、transferLeader 和 RemovePeer，你在前面的部分已经实现了。以下图中的第一个RaftGroup为例。调度器试图将 peer 从第三个 store 移到第四个 store。首先，它应该为第四个 store 添加 peer。然后它检查第三家是否是领导者，发现不是，所以不需要转移领导者。然后，它删除第三个 store 的 peer。


&emsp;&emsp;你可以使用 scheduler/server/schedule/operator 包中的CreateMovePeerOperator 函数来创建一个 MovePeer 操作。

![在这里插入图片描述](https://img-blog.csdnimg.cn/0a18d858da2742f98394c4098de75afb.png)

![在这里插入图片描述](https://img-blog.csdnimg.cn/3fbed2eb8ca6402c9662bab9483c727b.png)

&emsp;&emsp;在这一部分，你需要实现的唯一函数是scheduler/server/schedulers/balance_region.go 中的 Schedule 方法。这个调度器避免了在一个 store 里有太多的 region。首先，Scheduler 将选择所有合适的 store。然后根据它们的 region 大小进行排序。然后，调度器会尝试从 reigon 大小最大的 store 中找到要移动的 region。

&emsp;&emsp;调度器将尝试找到最适合在 store 中移动的 region。首先，它将尝试选择一个挂起的 region，因为挂起可能意味着磁盘过载。如果没有一个挂起的 region，它将尝试找到一个 Follower region。如果它仍然不能挑选出一个 region，它将尝试挑选领导 region。最后，它将挑选出要移动的 region，或者 Scheduler 将尝试下一个 region 大小较小的存储，直到所有的存储都将被尝试。

&emsp;&emsp;在您选择了一个要移动的 region 后，调度器将选择一个 store 作为目标。实际上，调度器将选择 region 大小最小的 store 。然后，调度程序将通过检查原始 store 和目标 store 的 region 大小之间的差异来判断这种移动是否有价值。如果差异足够大，Scheduler 应该在目标 store 上分配一个新的 peer 并创建一个移动 peer 操作。


正如你可能已经注意到的，上面的例程只是一个粗略的过程。还剩下很多问题：


- 哪些存储空间适合移动？


简而言之，一个合适的 store 应该是 Up 的，而且 down 的时间不能超过集群的MaxStoreDownTime，你可以通过 cluster.GetMaxStoreDownTime() 得到。


- 如何选择区域？


Scheduler 框架提供了三种方法来获取区域。GetPendingRegionsWithLock, GetFollowersWithLock 和 GetLeadersWithLock。Scheduler 可以从中获取相关region。然后你可以选择一个随机的region。


- 如何判断这个操作是否有价值？


如果原始 region 和目标 region 的 region 大小差异太小，在我们将 region 从原始 store 移动到目标 store 后，Scheduler 可能希望下次再移动回来。所以我们要确保这个差值必须大于 region 近似大小的2倍，这样才能保证移动后，目标 store 的 region 大小仍然小于原 store。


# processRegionHeartbeat

&emsp;&emsp;首先要实现的是一个 processRegionHeartbeat()，用来让集群调度器同步 regions 信息。每个 region 都会周期性的发送心跳给调度器，调度器会检查收到心跳中的region 信息是否合适，如果合适，以此更新自己的记录的 regions 信息。至于怎么检查，怎么更新，官方文档里写的很清晰，Version 和 ConfVer 均最大的，即为最新。





&emsp;&emsp;在 processRegionHeartbeat() 收到汇报来的心跳，先检查一下 RegionEpoch 是否是最新的，如果是新的则调用 c.putRegion() 和 c.updateStoreStatusLocked() 进行更新。


```go
// processRegionHeartbeat updates the region information.
func (c *RaftCluster) processRegionHeartbeat(region *core.RegionInfo) error {
	// Your Code Here (3C).
	epoch := region.GetRegionEpoch()
	if epoch == nil {
		return errors.Errorf("region has no epoch")
	}
	// 1. 检查是否有两个 region 的 id 是一样的
	oldRegion := c.GetRegion(region.GetID())
	if oldRegion != nil {
		oldEpoch := oldRegion.GetRegionEpoch()
		if epoch.ConfVer < oldEpoch.ConfVer || epoch.Version < oldEpoch.Version {
			return errors.Errorf("region is stale")
		}
	} else {
		// 2. 扫描所有重叠的 region
		regions := c.ScanRegions(region.GetStartKey(), region.GetEndKey(), -1)
		for _, r := range regions {
			rEpoch := r.GetRegionEpoch()
			if epoch.ConfVer < rEpoch.ConfVer || epoch.Version < rEpoch.Version {
				return errors.Errorf("region is stale")
			}
		}
	}
	// region 是最新的，更新 region tree 和 store status
	c.putRegion(region)
	for i := range region.GetStoreIds() {
		c.updateStoreStatusLocked(i)
	}
	return nil
}

```

# Schedule


接下来实现 region 调度，该部分用来让集群中的 stores 所负载的 region 趋于平衡，避免一个 store 中含有很多 region 而另一个 store 中含有很少 region 的情况。比如store1想把region1调度到store3上，那么现在store3上增加一个副本，再把原来store1的副本删掉即可






流程官方文档也说的很清楚，大致如下：


1. 选出 DownTime() < `MaxStoreDownTime` 的 store 作为 suitableStores，并按照 regionSize 降序排列；
2. 获取 regionSize 最大的 suitableStore，作为源 store，然后依次调用 `GetPendingRegionsWithLock`()、`GetFollowersWithLock`()、`GetLeadersWithLock`()，如果找到了一个待转移 region，执行下面的步骤，否则尝试下一个 suitableStore；
3. 判断待转移 region 的 store 数量，如果小于 `cluster.GetMaxReplicas`()，放弃转移；
4. 取出 regionSize 最小的 suitableStore 作为目标 store，并且该 store 不能在待转移 region 中，如果在，尝试次小的 suitableStore，以此类推；
5. 判断两 store 的 regionSize 差别是否过小，如果是小于`2*ApproximateSize`，放弃转移。因为如果此时接着转移，很有可能过不了久就重新转了回来；
6. 在目标 store 上创建一个 peer，然后调用 `CreateMovePeerOperator` 生成转移请求；




```go
// Schedule 避免太多 region 堆积在一个 store
func (s *balanceRegionScheduler) Schedule(cluster opt.Cluster) *operator.Operator {
	// Your Code Here (3C).
	// 1. 选出所有的 suitableStores
	stores := make(storeSlice, 0)
	for _, store := range cluster.GetStores() {
		// 适合被移动的 store 需要满足停机时间不超过 MaxStoreDownTime
		if store.IsUp() && store.DownTime() < cluster.GetMaxStoreDownTime() {
			stores = append(stores, store)
		}
	}
	if len(stores) < 2 {
		return nil
	}
	// 2. 遍历 suitableStores，找到目标 region 和 store
	sort.Sort(stores)
	var fromStore, toStore *core.StoreInfo
	var region *core.RegionInfo
	for i := len(stores) - 1; i >= 0; i-- {
		var regions core.RegionsContainer
		cluster.GetPendingRegionsWithLock(stores[i].GetID(), func(rc core.RegionsContainer) { regions = rc })
		region = regions.RandomRegion(nil, nil)
		if region != nil {
			fromStore = stores[i]
			break
		}
		cluster.GetFollowersWithLock(stores[i].GetID(), func(rc core.RegionsContainer) { regions = rc })
		region = regions.RandomRegion(nil, nil)
		if region != nil {
			fromStore = stores[i]
			break
		}
		cluster.GetLeadersWithLock(stores[i].GetID(), func(rc core.RegionsContainer) { regions = rc })
		region = regions.RandomRegion(nil, nil)
		if region != nil {
			fromStore = stores[i]
			break
		}
	}
	if region == nil {
		return nil
	}
	// 3. 判断目标 region 的 store 数量，如果小于 cluster.GetMaxReplicas 直接放弃本次操作
	storeIds := region.GetStoreIds()
	if len(storeIds) < cluster.GetMaxReplicas() {
		return nil
	}
	// 4. 再次从 suitableStores 里面找到一个目标 store，目标 store 不能在原来的 region 里面
	for i := 0; i < len(stores); i++ {
		if _, ok := storeIds[stores[i].GetID()]; !ok {
			toStore = stores[i]
			break
		}
	}
	if toStore == nil {
		return nil
	}
	// 5. 判断两个 store 的 region size 差值是否小于 2*ApproximateSize，是的话放弃 region 移动
	if fromStore.GetRegionSize()-toStore.GetRegionSize() < region.GetApproximateSize() {
		return nil
	}
	// 6. 创建 CreateMovePeerOperator 操作并返回
	newPeer, _ := cluster.AllocPeer(toStore.GetID())
	desc := fmt.Sprintf("move-from-%d-to-%d", fromStore.GetID(), toStore.GetID())
	op, _ := operator.CreateMovePeerOperator(desc, cluster, region, operator.OpBalance, fromStore.GetID(), toStore.GetID(), newPeer.GetId())
	return op
}

```