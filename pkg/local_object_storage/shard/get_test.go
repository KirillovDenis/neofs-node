package shard_test

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/nspcc-dev/neofs-node/pkg/core/object"
	"github.com/nspcc-dev/neofs-node/pkg/local_object_storage/shard"
	cidtest "github.com/nspcc-dev/neofs-sdk-go/container/id/test"
	objectSDK "github.com/nspcc-dev/neofs-sdk-go/object"
	"github.com/stretchr/testify/require"
)

func TestShard_Get(t *testing.T) {
	t.Run("without write cache", func(t *testing.T) {
		testShardGet(t, false)
	})

	t.Run("with write cache", func(t *testing.T) {
		testShardGet(t, true)
	})
}

func testShardGet(t *testing.T, hasWriteCache bool) {
	sh := newShard(t, hasWriteCache)
	defer releaseShard(sh, t)

	putPrm := new(shard.PutPrm)
	getPrm := new(shard.GetPrm)

	t.Run("small object", func(t *testing.T) {
		obj := generateObject(t)
		addAttribute(obj, "foo", "bar")
		addPayload(obj, 1<<5)

		putPrm.WithObject(obj)

		_, err := sh.Put(putPrm)
		require.NoError(t, err)

		getPrm.WithAddress(object.AddressOf(obj))

		res, err := testGet(t, sh, getPrm, hasWriteCache)
		require.NoError(t, err)
		require.Equal(t, obj, res.Object())
	})

	t.Run("big object", func(t *testing.T) {
		obj := generateObject(t)
		addAttribute(obj, "foo", "bar")
		obj.SetID(generateOID())
		addPayload(obj, 1<<20) // big obj

		putPrm.WithObject(obj)

		_, err := sh.Put(putPrm)
		require.NoError(t, err)

		getPrm.WithAddress(object.AddressOf(obj))

		res, err := testGet(t, sh, getPrm, hasWriteCache)
		require.NoError(t, err)
		require.Equal(t, obj, res.Object())
	})

	t.Run("parent object", func(t *testing.T) {
		obj := generateObject(t)
		addAttribute(obj, "foo", "bar")
		cid := cidtest.ID()
		splitID := objectSDK.NewSplitID()

		parent := generateObjectWithCID(t, cid)
		addAttribute(parent, "parent", "attribute")

		child := generateObjectWithCID(t, cid)
		child.SetParent(parent)
		child.SetParentID(parent.ID())
		child.SetSplitID(splitID)
		addPayload(child, 1<<5)

		putPrm.WithObject(child)

		_, err := sh.Put(putPrm)
		require.NoError(t, err)

		getPrm.WithAddress(object.AddressOf(child))

		res, err := testGet(t, sh, getPrm, hasWriteCache)
		require.NoError(t, err)
		require.True(t, binaryEqual(child, res.Object()))

		getPrm.WithAddress(object.AddressOf(parent))

		_, err = testGet(t, sh, getPrm, hasWriteCache)

		var expectedErr *objectSDK.SplitInfoError
		require.True(t, errors.As(err, &expectedErr))

		si, ok := err.(*objectSDK.SplitInfoError)
		require.True(t, ok)
		require.Nil(t, si.SplitInfo().Link())
		require.Equal(t, child.ID(), si.SplitInfo().LastPart())
		require.Equal(t, splitID, si.SplitInfo().SplitID())
	})
}

func testGet(t *testing.T, sh *shard.Shard, getPrm *shard.GetPrm, hasWriteCache bool) (*shard.GetRes, error) {
	res, err := sh.Get(getPrm)
	if hasWriteCache {
		require.Eventually(t, func() bool {
			if shard.IsErrNotFound(err) {
				res, err = sh.Get(getPrm)
			}
			return !shard.IsErrNotFound(err)
		}, time.Second, time.Millisecond*100)
	}
	return res, err
}

// binary equal is used when object contains empty lists in the structure and
// requre.Equal fails on comparing <nil> and []{} lists.
func binaryEqual(a, b *objectSDK.Object) bool {
	binaryA, err := a.Marshal()
	if err != nil {
		return false
	}

	binaryB, err := b.Marshal()
	if err != nil {
		return false
	}

	return bytes.Equal(binaryA, binaryB)
}
