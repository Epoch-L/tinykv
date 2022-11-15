# 前言

&emsp;&emsp;project1还是比较简单的，project2的难道就陡增了。对于project1来说，本文更多的还是讲一些不了解的概念。在每一个project1开始之前，仔细阅读文档。


# Project1 StandaloneKV 文档翻译
**==Project1 StandaloneKV==**

&emsp;&emsp;在这个项目中，我们将会在`列族`的支持下建立一个`独立的 key/value 存储 gRPC 服务`。Standalone 意味着只有一个节点，而不是一个分布式系统。`列族（CF, Column family）是一个类似 key 命名空间的术语，即同一个 key 在不同列族中的值是不同的。`你可以简单地将多个 CF 视为独立的小型数据库。CF 在 Project4 中被用来支持事务模型。

&emsp;&emsp;该服务支持四个基本操作：Put/Delete/Get/Scan，它维护了一个简单的 key/value Pairs 的数据库，其中 key and value 都是字符串。


- Put：替换数据库中指定 CF 的某个 key 的 value。
- Delete：删除指定 CF 的 key 的 value。
- Get：获取指定 CF 的某个 key 的当前值。
- Scan：获取指定 CF 的一系列 key 的 current value。

该项目可以分为2个步骤，包括:

1. 实现一个独立的存储引擎。
2. 实现原始的 key/value 服务处理程序。



**==代码==**

&emsp;&emsp;gRPC 服务在 kv/main.go 中被初始化，它包含一个 tinykv.Server，它提供了名为 TinyKv 的 gRPC 服务 。它由 proto/proto/tinykvpb.proto 中的 protocol-buffer 定义，rpc 请求和响应的细节被定义在 proto/proto/kvrpcpb.proto 中。


&emsp;&emsp;一般来说，不需要改变 proto 文件，因为所有必要的字段都已经被定义了。但如果仍然需要改变，可以修改 proto 文件并运行 make proto 来更新 proto/pkg/xxx/xxx.pb.go 中生成的 go 相关代码。


&emsp;&emsp;此外，`Server 依赖于一个 Storage`，这是一个`需要为独立存储引擎实现的接口`，位于 kv/storage/standalone_storage/standalone_storage.go 中。一旦 StandaloneStorage 中实现了`接口 Storage`，就可以用它实现Server 的原始 key/value 服务。




**==实现独立的存储引擎==**




&emsp;&emsp;第一个任务是实现 badger key/value 的 API。gRPC 服务依赖于一个在 kv/storage/storage.go 中定义的 Storage。在这种情况下，独立的存储引擎只是由两个方法提供的 badger key/value API。


```go
type Storage interface {
    // 省略其他的代码
    Write(ctx *kvrpcpb.Context, batch []Modify) error
    Reader(ctx *kvrpcpb.Context) (StorageReader, error)
}
```


- Write：应该提供一种方式，将一系列的修改应用到内部状态，`内部状态是一个 badger 的实例。`

- Reader：应该`返回一个 StorageReader`，支持 key/value 的单点读取和快照扫描的操作。

- 现在不需要考虑 kvrpcpb.Context，它在后面的项目中使用。

> 提示：
> - 使用 badger.Txn 来实现 Reader 函数，因为 badger 提供的事务处理程序可以提供 keys 和 values 的一致快照。
> - Badger 没有给出对 CF 的支持。engine_util 包（kv/util/engine_util）通过给 keys 添加前缀来模拟 CF。例如，一个属于特定 CF 的 key 被存储为 $ { cf } _ $ { key } 。它封装了 badger 以提供对 CF 的操作，还提供了许多有用的辅助函数。所以应该通过 engine_util 提供的方法进行所有读写操作。阅读 util/engine_util/doc.go 可以了解更多。
> - TinyKV使用了 badger 原始版本的分支，并进行了一些修正，所以应该使用 github.com/Connor1996/badger 而不是 github.com/dgraph-io/badger。
> - 不要忘记为 badger.Txn 调用 Discard()，并在销毁前关闭所有迭代器。

**==实现服务处理程序==**


&emsp;&emsp;这个项目的最后一步是`用实现的存储引擎`来构建`原始的 key/value 服务处理程序`，包括 RawGet / RawScan / RawPut / RawDelete。处理程序已经定义好了，只需要在 kv/server/raw_api.go 中补充实现。一旦完成，记得运行 make project1 以通过测试用例。



# 文档的重点内容




**==列族(CF Column family)==**

&emsp;&emsp;列族(CF Column family)：在文档的开始，所说的列族到底是什么意思？

> Column Family，也叫 CF，这个概念从 HBase 中来，就是将多个列合并为一个CF进行管理。这样读取一行数据时，你可以按照 CF 加载列，不需要加载所有列（通常同一个CF的列会保存在同一个文件中，所以这样有很高的效率）。此外因为同一列的数据格式相同，你可以针对某种格式采用高效的压缩算法。


```bash
default_wxf
default_www
default_xxx

write_wxf
write_www
write_xxx

lock_wxf
lock_www
lock_xxx
```

&emsp;&emsp;从上面的样例可以看出，CF的本质，就是key的前缀，就是一个字符串，起命名空间的作用。


```go
func KeyWithCF(cf string, key []byte) []byte {
	return append([]byte(cf+"_"), key...)
}
```


**==独立存储引擎==**




- Write：应该提供一种方式，将一系列的修改应用到内部状态，`内部状态是一个 badger 的实例。`

- Reader：应该`返回一个 StorageReader`，支持 key/value 的单点读取和快照扫描的操作。

&emsp;&emsp;看上面两句话的描述，其实核心就是，让我们使用基于badger实现write的操作，至于badger是怎么write的，我们不用管，我们只需要对badger进行一层封装即可。

&emsp;&emsp;对于Reader来说，核心就是让我们返回一个`StorageReader`接口，我们应该去实现这个接口然后返回，而这个接口如何实现，提示里也说了，使用` badger.Txn`去实现该接口。

&emsp;&emsp;实现了这两个接口，那么文档中所说的`独立存储引擎`就实现了。


**==原始的 key/value 服务处理程序==**

&emsp;&emsp;原始的 key/value 服务处理程序(Put/Delete/Get/Scan)：其实就是对存储引擎的一层上层封装，有了这层封装，我们可以随意改变底层的存储引擎。使用则无需关注底层细节，只要遵循上层接口使用规范即可。



# StandAloneStorage

&emsp;&emsp;通过文档中的提示2我们知道，engine_util对badger进行了封装，StandAloneStorage就是要在engine_util的基础上再封装一层。

```go
// Engines keeps references to and data for the engines used by unistore.
// All engines are badger key/value databases.
// the Path fields are the filesystem path to where the data is stored.
type Engines struct {
	// Data, including data which is committed (i.e., committed across other nodes) and un-committed (i.e., only present
	// locally).
	Kv     *badger.DB
	KvPath string
	// Metadata used by Raft.
	Raft     *badger.DB
	RaftPath string
}
```
&emsp;&emsp;那么我们就再封装一层即可，为什么这里要加一个conf？因为我们创建的时候需要用到conf里面的参数，因为不知道后续会不会用到这个config，所以先保存起来。
```go
type StandAloneStorage struct {
	// Your Data Here (1).
	engine *engine_util.Engines
	conf   *config.Config
}

func NewStandAloneStorage(conf *config.Config) *StandAloneStorage {
	// Your Code Here (1).
	dbPath := conf.DBPath

	kvPath := path.Join(dbPath, "kv")
	raftPath := path.Join(dbPath, "raft")

	kvDB := engine_util.CreateDB(kvPath, false)
	//project1没用到raft
	raftDB := engine_util.CreateDB(raftPath, true)

	//func CreateDB(path string, raft bool) *badger.DB
	//func NewEngines(kvEngine, raftEngine *badger.DB, kvPath, raftPath string) *Engines
	return &StandAloneStorage{
		engine: engine_util.NewEngines(kvDB, raftDB, kvPath, raftPath),
		conf:   conf,
	}
}
```
&emsp;&emsp;StandAloneStorage是一个实例，它应该去实现接口，即Storage接口。那么我们下面重点关注Write和Reader。
```go
type Storage interface {
	Start() error
	Stop() error
	Write(ctx *kvrpcpb.Context, batch []Modify) error
	Reader(ctx *kvrpcpb.Context) (StorageReader, error)
}
```

## Write
&emsp;&emsp;去看看Modify类型是什么
```go
Write(ctx *kvrpcpb.Context, batch []Modify) error
```

&emsp;&emsp;Modify 本质上代表着Put和Delete两种操作，通过下面的函数可以看到，它是通过断言来区分Put还是Delete的。一个Modify对应一个kv，所以可以看到上面Write接口中，Modify是一个切片，那么对于Write，我们需要遍历range这个切片进行操作。

```go
// Modify is a single modification to TinyKV's underlying storage.
type Modify struct {
	Data interface{}
}

type Put struct {
	Key   []byte
	Value []byte
	Cf    string
}

type Delete struct {
	Key []byte
	Cf  string
}

func (m *Modify) Key() []byte {
	switch m.Data.(type) {
	case Put:
		return m.Data.(Put).Key
	case Delete:
		return m.Data.(Delete).Key
	}
	return nil
}

func (m *Modify) Value() []byte {
	if putData, ok := m.Data.(Put); ok {
		return putData.Value
	}

	return nil
}

func (m *Modify) Cf() string {
	switch m.Data.(type) {
	case Put:
		return m.Data.(Put).Cf
	case Delete:
		return m.Data.(Delete).Cf
	}
	return ""
}

```


&emsp;&emsp;提示2中说了，engine_util包中提供了方法进行所有的读写操作，所以现在去看看提供了什么api供我们使用。

&emsp;&emsp;可以看到提供了两个函数给我们使用，看一下源码其实也能发现，在真实存储的时候，key被加了前缀：`cf_key`.
```go
func PutCF(engine *badger.DB, cf string, key []byte, val []byte) error {
	return engine.Update(func(txn *badger.Txn) error {
		return txn.Set(KeyWithCF(cf, key), val)
	})
}

func DeleteCF(engine *badger.DB, cf string, key []byte) error {
	return engine.Update(func(txn *badger.Txn) error {
		return txn.Delete(KeyWithCF(cf, key))
	})
}
```


&emsp;&emsp;在了解了engine_util提供的方法，以及Modify的含义后，Write怎么实现其实就迎刃而解了。



```go
func (s *StandAloneStorage) Write(ctx *kvrpcpb.Context, batch []storage.Modify) error {
	// Your Code Here (1).
	//func PutCF(engine *badger.DB, cf string, key []byte, val []byte)
	var err error
	for _, m := range batch {
		key, val, cf := m.Key(), m.Value(), m.Cf()
		if _, ok := m.Data.(storage.Put); ok {
			err = engine_util.PutCF(s.engine.Kv, cf, key, val)
		} else {
			err = engine_util.DeleteCF(s.engine.Kv, cf, key)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
```



## Reader

&emsp;&emsp;Reader函数需要我们返回一个StorageReader接口，那么我们就需要去实现一个StorageReader接口的结构体，来看看实现它需要哪些函数

```go
Reader(ctx *kvrpcpb.Context) (StorageReader, error)

type StorageReader interface {
	// When the key doesn't exist, return nil for the value
	GetCF(cf string, key []byte) ([]byte, error)
	IterCF(cf string) engine_util.DBIterator
	Close()
}
```

&emsp;&emsp;看一下engine_util包给我们提供了什么函数，可以看到从txn中读取或者遍历的方法不用自己写，直接使用api即可，那么上面的GetCF和IterCF就是对下面两个函数的封装。
```go
// get value
val, err := engine_util.GetCFFromTxn(txn, cf, key)

func GetCFFromTxn(txn *badger.Txn, cf string, key []byte) (val []byte, err error)

// get iterator
iter := engine_util.NewCFIterator(cf, txn)

func NewCFIterator(cf string, txn *badger.Txn) *BadgerIterator
```



&emsp;&emsp;可以看到engine_util提供的GetCFFromTxn和NewCFIterator两个函数都需要`badger.Txn`，那么这个Txn是什么呢？其实是一个事务。获取badger.Txn的函数engine_util并未给出，需要直接调用badger.DB.NewTransaction函数。


```go
txn *badger.Txn

//update为真表示Put/Delete两个写操作，为假表示Get/Scan两个读操作。
//NewTransaction creates a new transaction.
func (db *DB) NewTransaction(update bool) *Txn
```


&emsp;&emsp;所以不难发现，想要实现StorageReader接口，那么结构体中就要包含badger.Txn，继而去调用engine_util提供的api。在这里，可以把这个字段放在StandAloneStorage中；不过我还是把它拆到新的结构体里面了。


&emsp;&emsp;为什么呢？在上面的Write函数中，我们仅仅传入的是`*badger.DB`，其内部也是有`*badger.Txn`的，但是`Write对事务进行了屏蔽`。在StorageReader接口的两个函数中，也没有事务的身影，但是我们因为是读，所以需要用到新的读事务，只能去创建一个。

&emsp;&emsp;综合来看，这些接口的封装，很明显就是不想让我们去过多的使用事务的，所以干脆把其放在一个新结构体里面return掉好了。






```go
func (s *StandAloneStorage) Reader(ctx *kvrpcpb.Context) (storage.StorageReader, error) {
	// Your Code Here (1).

	//func (db *DB) NewTransaction(update bool) *Txn
	//For read-only transactions, set update to false.
	txn := s.engine.Kv.NewTransaction(false)
	return NewStandAloneStorageReader(txn), nil
}

//实现 type StorageReader interface 接口

type StandAloneStorageReader struct {
	KvTxn *badger.Txn
}

func NewStandAloneStorageReader(txn *badger.Txn) *StandAloneStorageReader {
	return &StandAloneStorageReader{
		KvTxn: txn,
	}
}

func (s *StandAloneStorageReader) GetCF(cf string, key []byte) ([]byte, error) {
	val, err := engine_util.GetCFFromTxn(s.KvTxn, cf, key)
	//StandAloneStorage是底层的一个系统,这里认为not found不是err
	//将not found屏蔽掉
	//上层不认为not found是err
	if err == badger.ErrKeyNotFound {
		return nil, nil
	}
	return val, err
}

func (s *StandAloneStorageReader) IterCF(cf string) engine_util.DBIterator {
	//func NewCFIterator(cf string, KvTxn *badger.Txn) *BadgerIterator
	return engine_util.NewCFIterator(cf, s.KvTxn)
}

func (s *StandAloneStorageReader) Close() {
	s.KvTxn.Discard()
}

```

# Server

&emsp;&emsp;前面以及说过了，原始的 key/value 服务处理程序(Put/Delete/Get/Scan)：其实就是对存储引擎的一层上层封装，有了这层封装，我们可以随意改变底层的存储引擎。使用则无需关注底层细节，只要遵循上层接口使用规范即可。

```go
// Server is a TinyKV server, it 'faces outwards', sending and receiving messages from clients such as TinySQL.
type Server struct {
	storage storage.Storage
	//...
}
```

&emsp;&emsp;这样做我觉得最大的好处就是，底层的存储引擎无论怎么换，对上层都是无感的。这让我想到了mysql的innodb和myisam，都是无感的。在project1中，我们实现的是单机的存储引擎，所以在后续的project中，我们就可以直接换掉它。




```go
func (server *Server) RawGet(_ context.Context, req *kvrpcpb.RawGetRequest) (*kvrpcpb.RawGetResponse, error)
func (server *Server) RawPut(_ context.Context, req *kvrpcpb.RawPutRequest) (*kvrpcpb.RawPutResponse, error)
func (server *Server) RawDelete(_ context.Context, req *kvrpcpb.RawDeleteRequest) (*kvrpcpb.RawDeleteResponse, error)
func (server *Server) RawScan(_ context.Context, req *kvrpcpb.RawScanRequest) (*kvrpcpb.RawScanResponse, error)
```


&emsp;&emsp;这4个接口都是对上面的StandAloneStorage中写的接口的再次封装。


- RawGet： 如果获取不到，返回时要标记 reply.NotFound=true。
- RawPut：把单一的 put 请求用 storage.Modify 包装成一个数组，一次性写入。
- RawDelete：和 RawPut 同理。
- RawScan：通过 reader 获取 iter，从 StartKey 开始，同时注意 limit。

&emsp;&emsp;需要特别注意的一点是：`RawGet： 如果获取不到，返回时要标记 reply.NotFound=true`


&emsp;&emsp;我所理解的Scan，是线性扫描。那么转入请求中的Limit，就是限制多少个。首先根据Seek定位到第一个key，然后向后读Limit个，同时要注意下一个的合法性，具体见代码。




```go
// The functions below are Server's Raw API. (implements TinyKvServer).
// Some helper methods can be found in sever.go in the current directory

// 上层允许err时返回nil结构体

// RawGet return the corresponding Get response based on RawGetRequest's CF and Key fields
func (server *Server) RawGet(_ context.Context, req *kvrpcpb.RawGetRequest) (*kvrpcpb.RawGetResponse, error) {
	// Your Code Here (1).
	reader, err := server.storage.Reader(req.Context)
	if err != nil {
		return nil, err
	}
	val, err := reader.GetCF(req.Cf, req.Key)
	if err != nil {
		return nil, err
	}
	resp := &kvrpcpb.RawGetResponse{
		Value:    val,
		NotFound: false,
	}
	if val == nil {
		resp.NotFound = true
	}
	return resp, nil
}

// RawPut puts the target data into storage and returns the corresponding response
func (server *Server) RawPut(_ context.Context, req *kvrpcpb.RawPutRequest) (*kvrpcpb.RawPutResponse, error) {
	// Your Code Here (1).
	// Hint: Consider using Storage.Modify to store data to be modified
	put := storage.Put{
		Key:   req.Key,
		Value: req.Value,
		Cf:    req.Cf,
	}
	batch := storage.Modify{Data: put}
	err := server.storage.Write(req.Context, []storage.Modify{batch})
	if err != nil {
		return nil, err

	}
	return &kvrpcpb.RawPutResponse{}, nil
}

// RawDelete delete the target data from storage and returns the corresponding response
func (server *Server) RawDelete(_ context.Context, req *kvrpcpb.RawDeleteRequest) (*kvrpcpb.RawDeleteResponse, error) {
	// Your Code Here (1).
	// Hint: Consider using Storage.Modify to store data to be deleted
	del := storage.Delete{
		Key: req.Key,
		Cf:  req.Cf,
	}
	batch := storage.Modify{Data: del}
	err := server.storage.Write(req.Context, []storage.Modify{batch})
	if err != nil {
		return nil, err
	}
	return &kvrpcpb.RawDeleteResponse{}, nil
}

// RawScan scan the data starting from the start key up to limit. and return the corresponding result
func (server *Server) RawScan(_ context.Context, req *kvrpcpb.RawScanRequest) (*kvrpcpb.RawScanResponse, error) {
	// Your Code Here (1).
	// Hint: Consider using reader.IterCF
	reader, err := server.storage.Reader(req.Context)
	if err != nil {
		return nil, err
	}
	var Kvs []*kvrpcpb.KvPair
	limit := req.Limit

	iter := reader.IterCF(req.Cf)
	defer iter.Close()


	for iter.Seek(req.StartKey); iter.Valid(); iter.Next() {
		item := iter.Item()
		key := item.Key()
		val, _ := item.Value()

		Kvs = append(Kvs, &kvrpcpb.KvPair{
			Key:   key,
			Value: val,
		})
		limit--
		if limit == 0 {
			break
		}
	}

	resp := &kvrpcpb.RawScanResponse{
		Kvs: Kvs,
	}
	return resp, nil
}

```


# 单元测试

pass咯~~~

```bash
root@wxf:/tinykv# make project1

GO111MODULE=on go test -v --count=1 --parallel=1 -p=1 ./kv/server -run 1
=== RUN   TestRawGet1
--- PASS: TestRawGet1 (0.85s)
=== RUN   TestRawGetNotFound1
--- PASS: TestRawGetNotFound1 (1.05s)
=== RUN   TestRawPut1
--- PASS: TestRawPut1 (2.91s)
=== RUN   TestRawGetAfterRawPut1
--- PASS: TestRawGetAfterRawPut1 (3.94s)
=== RUN   TestRawGetAfterRawDelete1
--- PASS: TestRawGetAfterRawDelete1 (3.69s)
=== RUN   TestRawDelete1
--- PASS: TestRawDelete1 (1.54s)
=== RUN   TestRawScan1
--- PASS: TestRawScan1 (1.04s)
=== RUN   TestRawScanAfterRawPut1
--- PASS: TestRawScanAfterRawPut1 (1.03s)
=== RUN   TestRawScanAfterRawDelete1
--- PASS: TestRawScanAfterRawDelete1 (0.88s)
=== RUN   TestIterWithRawDelete1
--- PASS: TestIterWithRawDelete1 (1.08s)
PASS
ok  	github.com/pingcap-incubator/tinykv/kv/server	18.027s
root@wxf:/tinykv# 
```
