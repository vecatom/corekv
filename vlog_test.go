// Copyright 2021 hardcore-os Project Authors
//
// Licensed under the Apache License, Version 2.0 (the "License")
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package corekv

import (
	"bytes"
	"math/rand"
	"os"
	"testing"

	"github.com/hardcore-os/corekv/utils"
	"github.com/stretchr/testify/require"
)

var (
	// 初始化opt
	opt = &Options{
		WorkDir:          "./work_test",
		SSTableMaxSz:     1024,
		MemTableSize:     1024,
		ValueLogFileSize: 1024 * 10,
		ValueThreshold:   0,
		MaxBatchCount:    10000,
		MaxBatchSize:     1 << 20,
	}
)

func TestVlogBase(t *testing.T) {
	//clearDir()
	db := Open(opt)
	defer db.Close()
	log := db.vlog
	var err error
	// Use value big enough that the value log writes them even if SyncWrites is false.
	const val1 = "sampleval012345678901234567890123"
	const val2 = "samplevalb012345678901234567890123"
	require.True(t, int64(len(val1)) >= db.opt.ValueThreshold)

	e1 := &utils.Entry{
		Key:   []byte("samplekey"),
		Value: []byte(val1),
		Meta:  utils.BitValuePointer,
	}
	e2 := &utils.Entry{
		Key:   []byte("samplekeyb"),
		Value: []byte(val2),
		Meta:  utils.BitValuePointer,
	}

	b := new(request)
	b.Entries = []*utils.Entry{e1, e2}

	log.write([]*request{b})
	require.Len(t, b.Ptrs, 2)
	t.Logf("Pointer written: %+v %+v\n", b.Ptrs[0], b.Ptrs[1])
	buf1, lf1, err1 := log.readValueBytes(b.Ptrs[0])
	buf2, lf2, err2 := log.readValueBytes(b.Ptrs[1])
	require.NoError(t, err1)
	require.NoError(t, err2)
	defer utils.RunCallback(log.getUnlockCallback(lf1))
	defer utils.RunCallback((log.getUnlockCallback(lf2)))
	e1, err = lf1.DecodeEntry(buf1, b.Ptrs[0].Offset)
	require.NoError(t, err)
	e2, err = lf1.DecodeEntry(buf2, b.Ptrs[1].Offset)
	require.NoError(t, err)
	readEntries := []utils.Entry{*e1, *e2}
	require.EqualValues(t, []utils.Entry{
		{
			Key:    []byte("samplekey"),
			Value:  []byte(val1),
			Meta:   utils.BitValuePointer,
			Offset: b.Ptrs[0].Offset,
		},
		{
			Key:    []byte("samplekeyb"),
			Value:  []byte(val2),
			Meta:   utils.BitValuePointer,
			Offset: b.Ptrs[1].Offset,
		},
	}, readEntries)
}

func clearDir() {
	_, err := os.Stat(opt.WorkDir)
	if err == nil {
		os.RemoveAll(opt.WorkDir)
	}
	os.Mkdir(opt.WorkDir, os.ModePerm)
}

func TestValueGC(t *testing.T) {
	clearDir()
	opt.ValueLogFileSize = 1 << 20
	kv := Open(opt)
	defer kv.Close()
	sz := 32 << 10
	kvList := []*utils.Entry{}
	for i := 0; i < 100; i++ {
		e := newRandEntry(sz)
		kvList = append(kvList, &utils.Entry{
			Key:       e.Key,
			Value:     e.Value,
			Meta:      e.Meta,
			ExpiresAt: e.ExpiresAt,
		})
		require.NoError(t, kv.Set(e))
	}
	kv.RunValueLogGC(0.9)
	for _, e := range kvList {
		item, err := kv.Get(e.Key)
		require.NoError(t, err)
		val := getItemValue(t, item)
		require.NotNil(t, val)
		require.True(t, bytes.Equal(item.Key, e.Key), "key not equal: e:%s, v:%s", e.Key, e.Key)
		require.True(t, bytes.Equal(item.Value, e.Value), "value not equal: e:%s, v:%s", e.Value, e.Value)
	}
}

func newRandEntry(sz int) *utils.Entry {
	v := make([]byte, sz)
	rand.Read(v[:rand.Intn(sz)])
	e := utils.BuildEntry()
	e.Value = v
	return e
}
func getItemValue(t *testing.T, item *utils.Entry) (val []byte) {
	t.Helper()
	if item == nil {
		return nil
	}
	var v []byte
	v = append(v, item.Value...)
	if v == nil {
		return nil
	}
	return v
}
