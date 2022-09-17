package server

import (
	"os"
	"testing"

	"github.com/pingcap-incubator/tinykv/kv/config"
	"github.com/pingcap-incubator/tinykv/kv/storage"
	"github.com/pingcap-incubator/tinykv/kv/storage/standalone_storage"
	"github.com/pingcap-incubator/tinykv/kv/util/engine_util"
	"github.com/pingcap-incubator/tinykv/proto/pkg/kvrpcpb"
	"github.com/stretchr/testify/assert"
)

func Set(s *standalone_storage.StandAloneStorage, cf string, key []byte, value []byte) error {
	return s.Write(nil, []storage.Modify{
		{
			Data: storage.Put{
				Cf:    cf,
				Key:   key,
				Value: value,
			},
		},
	})
}

func Get(s *standalone_storage.StandAloneStorage, cf string, key []byte) ([]byte, error) {
	reader, err := s.Reader(nil)
	if err != nil {
		return nil, err
	}
	return reader.GetCF(cf, key)
}

func Iter(s *standalone_storage.StandAloneStorage, cf string) (engine_util.DBIterator, error) {
	reader, err := s.Reader(nil)
	if err != nil {
		return nil, err
	}
	return reader.IterCF(cf), nil
}

func cleanUpTestData(conf *config.Config) error {
	if conf != nil {
		return os.RemoveAll(conf.DBPath)
	}
	return nil
}

/*
2022年9月17日 22点48分

root@wxf:/tinykv# make project1
GO111MODULE=on go test -v --count=1 --parallel=1 -p=1 ./kv/server -run 1
=== RUN   TestRawGet1
--- PASS: TestRawGet1 (0.86s)
=== RUN   TestRawGetNotFound1
--- PASS: TestRawGetNotFound1 (1.00s)
=== RUN   TestRawPut1
--- PASS: TestRawPut1 (0.83s)
=== RUN   TestRawGetAfterRawPut1
--- PASS: TestRawGetAfterRawPut1 (1.11s)
=== RUN   TestRawGetAfterRawDelete1
--- PASS: TestRawGetAfterRawDelete1 (1.14s)
=== RUN   TestRawDelete1
--- PASS: TestRawDelete1 (0.91s)
=== RUN   TestRawScan1
--- PASS: TestRawScan1 (0.89s)
=== RUN   TestRawScanAfterRawPut1
--- PASS: TestRawScanAfterRawPut1 (0.99s)
=== RUN   TestRawScanAfterRawDelete1
--- PASS: TestRawScanAfterRawDelete1 (1.10s)
=== RUN   TestIterWithRawDelete1
--- PASS: TestIterWithRawDelete1 (1.07s)
PASS
ok  	github.com/pingcap-incubator/tinykv/kv/server	9.923s
*/

func TestRawGet1(t *testing.T) {
	conf := config.NewTestConfig()
	s := standalone_storage.NewStandAloneStorage(conf)
	s.Start()
	server := NewServer(s)
	defer cleanUpTestData(conf)
	defer s.Stop()

	cf := engine_util.CfDefault
	Set(s, cf, []byte{99}, []byte{42})

	req := &kvrpcpb.RawGetRequest{
		Key: []byte{99},
		Cf:  cf,
	}
	resp, err := server.RawGet(nil, req)
	assert.Nil(t, err)
	assert.Equal(t, []byte{42}, resp.Value)
}

func TestRawGetNotFound1(t *testing.T) {
	conf := config.NewTestConfig()
	s := standalone_storage.NewStandAloneStorage(conf)
	s.Start()
	server := NewServer(s)
	defer cleanUpTestData(conf)
	defer s.Stop()

	cf := engine_util.CfDefault
	req := &kvrpcpb.RawGetRequest{
		Key: []byte{99},
		Cf:  cf,
	}
	resp, err := server.RawGet(nil, req)
	assert.Nil(t, err)
	//从这里就可以看出，上层不认为not found是一种err
	assert.True(t, resp.NotFound)
}

func TestRawPut1(t *testing.T) {
	conf := config.NewTestConfig()
	s := standalone_storage.NewStandAloneStorage(conf)
	s.Start()
	server := NewServer(s)
	defer cleanUpTestData(conf)
	defer s.Stop()

	cf := engine_util.CfDefault
	req := &kvrpcpb.RawPutRequest{
		Key:   []byte{99},
		Value: []byte{42},
		Cf:    cf,
	}

	_, err := server.RawPut(nil, req)

	got, err := Get(s, cf, []byte{99})
	assert.Nil(t, err)
	assert.Equal(t, []byte{42}, got)
}

func TestRawGetAfterRawPut1(t *testing.T) {
	conf := config.NewTestConfig()
	s := standalone_storage.NewStandAloneStorage(conf)
	s.Start()
	server := NewServer(s)
	defer cleanUpTestData(conf)
	defer s.Stop()

	put1 := &kvrpcpb.RawPutRequest{
		Key:   []byte{99},
		Value: []byte{42},
		Cf:    engine_util.CfDefault,
	}
	_, err := server.RawPut(nil, put1)
	assert.Nil(t, err)

	put2 := &kvrpcpb.RawPutRequest{
		Key:   []byte{99},
		Value: []byte{44},
		Cf:    engine_util.CfWrite,
	}
	_, err = server.RawPut(nil, put2)
	assert.Nil(t, err)

	get1 := &kvrpcpb.RawGetRequest{
		Key: []byte{99},
		Cf:  engine_util.CfDefault,
	}
	resp, err := server.RawGet(nil, get1)
	assert.Nil(t, err)
	assert.Equal(t, []byte{42}, resp.Value)

	get2 := &kvrpcpb.RawGetRequest{
		Key: []byte{99},
		Cf:  engine_util.CfWrite,
	}
	resp, err = server.RawGet(nil, get2)
	assert.Nil(t, err)
	assert.Equal(t, []byte{44}, resp.Value)
}

func TestRawGetAfterRawDelete1(t *testing.T) {
	conf := config.NewTestConfig()
	s := standalone_storage.NewStandAloneStorage(conf)
	s.Start()
	server := NewServer(s)
	defer cleanUpTestData(conf)
	defer s.Stop()

	cf := engine_util.CfDefault
	assert.Nil(t, Set(s, cf, []byte{99}, []byte{42}))

	delete := &kvrpcpb.RawDeleteRequest{
		Key: []byte{99},
		Cf:  cf,
	}
	get := &kvrpcpb.RawGetRequest{
		Key: []byte{99},
		Cf:  cf,
	}

	_, err := server.RawDelete(nil, delete)
	assert.Nil(t, err)

	resp, err := server.RawGet(nil, get)
	assert.Nil(t, err)
	assert.True(t, resp.NotFound)
}

func TestRawDelete1(t *testing.T) {
	conf := config.NewTestConfig()
	s := standalone_storage.NewStandAloneStorage(conf)
	s.Start()
	server := NewServer(s)
	defer cleanUpTestData(conf)
	defer s.Stop()

	cf := engine_util.CfDefault
	assert.Nil(t, Set(s, cf, []byte{99}, []byte{42}))

	req := &kvrpcpb.RawDeleteRequest{
		Key: []byte{99},
		Cf:  cf,
	}

	_, err := server.RawDelete(nil, req)
	assert.Nil(t, err)

	val, err := Get(s, cf, []byte{99})
	assert.Equal(t, nil, err)
	assert.Equal(t, []byte(nil), val)
}

func TestRawScan1(t *testing.T) {
	conf := config.NewTestConfig()
	s := standalone_storage.NewStandAloneStorage(conf)
	s.Start()
	server := NewServer(s)
	defer cleanUpTestData(conf)
	defer s.Stop()

	cf := engine_util.CfDefault

	Set(s, cf, []byte{1}, []byte{233, 1})
	Set(s, cf, []byte{2}, []byte{233, 2})
	Set(s, cf, []byte{3}, []byte{233, 3})
	Set(s, cf, []byte{4}, []byte{233, 4})
	Set(s, cf, []byte{5}, []byte{233, 5})

	req := &kvrpcpb.RawScanRequest{
		StartKey: []byte{1},
		Limit:    3,
		Cf:       cf,
	}

	resp, err := server.RawScan(nil, req)
	assert.Nil(t, err)

	assert.Equal(t, 3, len(resp.Kvs))
	expectedKeys := [][]byte{{1}, {2}, {3}}
	for i, kv := range resp.Kvs {
		assert.Equal(t, expectedKeys[i], kv.Key)
		assert.Equal(t, append([]byte{233}, expectedKeys[i]...), kv.Value)
	}
}

func TestRawScanAfterRawPut1(t *testing.T) {
	conf := config.NewTestConfig()
	s := standalone_storage.NewStandAloneStorage(conf)
	s.Start()
	server := NewServer(s)
	defer cleanUpTestData(conf)
	defer s.Stop()

	cf := engine_util.CfDefault
	assert.Nil(t, Set(s, cf, []byte{1}, []byte{233, 1}))
	assert.Nil(t, Set(s, cf, []byte{2}, []byte{233, 2}))
	assert.Nil(t, Set(s, cf, []byte{3}, []byte{233, 3}))
	assert.Nil(t, Set(s, cf, []byte{4}, []byte{233, 4}))

	put := &kvrpcpb.RawPutRequest{
		Key:   []byte{5},
		Value: []byte{233, 5},
		Cf:    cf,
	}

	scan := &kvrpcpb.RawScanRequest{
		StartKey: []byte{1},
		Limit:    10,
		Cf:       cf,
	}

	expectedKeys := [][]byte{{1}, {2}, {3}, {4}, {5}}

	_, err := server.RawPut(nil, put)
	assert.Nil(t, err)

	resp, err := server.RawScan(nil, scan)
	assert.Nil(t, err)
	assert.Equal(t, len(expectedKeys), len(resp.Kvs))
	for i, kv := range resp.Kvs {
		assert.Equal(t, expectedKeys[i], kv.Key)
		assert.Equal(t, append([]byte{233}, expectedKeys[i]...), kv.Value)
	}
}

func TestRawScanAfterRawDelete1(t *testing.T) {
	conf := config.NewTestConfig()
	s := standalone_storage.NewStandAloneStorage(conf)
	s.Start()
	server := NewServer(s)
	defer cleanUpTestData(conf)
	defer s.Stop()

	cf := engine_util.CfDefault
	assert.Nil(t, Set(s, cf, []byte{1}, []byte{233, 1}))
	assert.Nil(t, Set(s, cf, []byte{2}, []byte{233, 2}))
	assert.Nil(t, Set(s, cf, []byte{3}, []byte{233, 3}))
	assert.Nil(t, Set(s, cf, []byte{4}, []byte{233, 4}))

	delete := &kvrpcpb.RawDeleteRequest{
		Key: []byte{3},
		Cf:  cf,
	}

	scan := &kvrpcpb.RawScanRequest{
		StartKey: []byte{1},
		Limit:    10,
		Cf:       cf,
	}

	expectedKeys := [][]byte{{1}, {2}, {4}}

	_, err := server.RawDelete(nil, delete)
	assert.Nil(t, err)

	resp, err := server.RawScan(nil, scan)
	assert.Nil(t, err)
	assert.Equal(t, len(expectedKeys), len(resp.Kvs))
	for i, kv := range resp.Kvs {
		assert.Equal(t, expectedKeys[i], kv.Key)
		assert.Equal(t, append([]byte{233}, expectedKeys[i]...), kv.Value)
	}
}

func TestIterWithRawDelete1(t *testing.T) {
	conf := config.NewTestConfig()
	s := standalone_storage.NewStandAloneStorage(conf)
	s.Start()
	server := NewServer(s)
	defer cleanUpTestData(conf)
	defer s.Stop()

	cf := engine_util.CfDefault
	assert.Nil(t, Set(s, cf, []byte{1}, []byte{233, 1}))
	assert.Nil(t, Set(s, cf, []byte{2}, []byte{233, 2}))
	assert.Nil(t, Set(s, cf, []byte{3}, []byte{233, 3}))
	assert.Nil(t, Set(s, cf, []byte{4}, []byte{233, 4}))

	it, err := Iter(s, cf)
	defer it.Close()
	assert.Nil(t, err)

	delete := &kvrpcpb.RawDeleteRequest{
		Key: []byte{3},
		Cf:  cf,
	}
	_, err = server.RawDelete(nil, delete)
	assert.Nil(t, err)

	expectedKeys := [][]byte{{1}, {2}, {3}, {4}}
	i := 0
	for it.Seek([]byte{1}); it.Valid(); it.Next() {
		item := it.Item()
		key := item.Key()
		assert.Equal(t, expectedKeys[i], key)
		i++
	}
}
