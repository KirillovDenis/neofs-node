package meta

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/nspcc-dev/neofs-node/pkg/core/object"
	storagelog "github.com/nspcc-dev/neofs-node/pkg/local_object_storage/internal/log"
	apistatus "github.com/nspcc-dev/neofs-sdk-go/client/status"
	objectSDK "github.com/nspcc-dev/neofs-sdk-go/object"
	addressSDK "github.com/nspcc-dev/neofs-sdk-go/object/address"
	"go.etcd.io/bbolt"
)

// DeletePrm groups the parameters of Delete operation.
type DeletePrm struct {
	addrs []*addressSDK.Address
}

// DeleteRes groups resulting values of Delete operation.
type DeleteRes struct{}

// WithAddresses is a Delete option to set the addresses of the objects to delete.
//
// Option is required.
func (p *DeletePrm) WithAddresses(addrs ...*addressSDK.Address) *DeletePrm {
	if p != nil {
		p.addrs = addrs
	}

	return p
}

// Delete removes objects from DB.
func Delete(db *DB, addrs ...*addressSDK.Address) error {
	_, err := db.Delete(new(DeletePrm).WithAddresses(addrs...))
	return err
}

type referenceNumber struct {
	all, cur int

	addr *addressSDK.Address

	obj *objectSDK.Object
}

type referenceCounter map[string]*referenceNumber

// Delete removed object records from metabase indexes.
func (db *DB) Delete(prm *DeletePrm) (*DeleteRes, error) {
	err := db.boltDB.Update(func(tx *bbolt.Tx) error {
		return db.deleteGroup(tx, prm.addrs)
	})
	if err == nil {
		for i := range prm.addrs {
			storagelog.Write(db.log,
				storagelog.AddressField(prm.addrs[i]),
				storagelog.OpField("metabase DELETE"))
		}
	}
	return new(DeleteRes), err
}

func (db *DB) deleteGroup(tx *bbolt.Tx, addrs []*addressSDK.Address) error {
	refCounter := make(referenceCounter, len(addrs))

	for i := range addrs {
		err := db.delete(tx, addrs[i], refCounter)
		if err != nil {
			return err // maybe log and continue?
		}
	}

	for _, refNum := range refCounter {
		if refNum.cur == refNum.all {
			err := db.deleteObject(tx, refNum.obj, true)
			if err != nil {
				return err // maybe log and continue?
			}
		}
	}

	return nil
}

func (db *DB) delete(tx *bbolt.Tx, addr *addressSDK.Address, refCounter referenceCounter) error {
	// remove record from graveyard
	graveyard := tx.Bucket(graveyardBucketName)
	if graveyard != nil {
		err := graveyard.Delete(addressKey(addr))
		if err != nil {
			return fmt.Errorf("could not remove from graveyard: %w", err)
		}
	}

	// unmarshal object, work only with physically stored (raw == true) objects
	obj, err := db.get(tx, addr, false, true)
	if err != nil {
		if errors.As(err, new(apistatus.ObjectNotFound)) {
			return nil
		}

		return err
	}

	// if object is an only link to a parent, then remove parent
	if parent := obj.Parent(); parent != nil {
		parAddr := object.AddressOf(parent)
		sParAddr := parAddr.String()

		nRef, ok := refCounter[sParAddr]
		if !ok {
			nRef = &referenceNumber{
				all:  parentLength(tx, parAddr),
				addr: parAddr,
				obj:  parent,
			}

			refCounter[sParAddr] = nRef
		}

		nRef.cur++
	}

	// remove object
	return db.deleteObject(tx, obj, false)
}

func (db *DB) deleteObject(
	tx *bbolt.Tx,
	obj *objectSDK.Object,
	isParent bool,
) error {
	err := delUniqueIndexes(tx, obj, isParent)
	if err != nil {
		return fmt.Errorf("can't remove unique indexes")
	}

	err = updateListIndexes(tx, obj, delListIndexItem)
	if err != nil {
		return fmt.Errorf("can't remove list indexes: %w", err)
	}

	err = updateFKBTIndexes(tx, obj, delFKBTIndexItem)
	if err != nil {
		return fmt.Errorf("can't remove fake bucket tree indexes: %w", err)
	}

	return nil
}

// parentLength returns amount of available children from parentid index.
func parentLength(tx *bbolt.Tx, addr *addressSDK.Address) int {
	bkt := tx.Bucket(parentBucketName(addr.ContainerID()))
	if bkt == nil {
		return 0
	}

	lst, err := decodeList(bkt.Get(objectKey(addr.ObjectID())))
	if err != nil {
		return 0
	}

	return len(lst)
}

func delUniqueIndexItem(tx *bbolt.Tx, item namedBucketItem) {
	bkt := tx.Bucket(item.name)
	if bkt != nil {
		_ = bkt.Delete(item.key) // ignore error, best effort there
	}
}

func delFKBTIndexItem(tx *bbolt.Tx, item namedBucketItem) error {
	bkt := tx.Bucket(item.name)
	if bkt == nil {
		return nil
	}

	fkbtRoot := bkt.Bucket(item.key)
	if fkbtRoot == nil {
		return nil
	}

	_ = fkbtRoot.Delete(item.val) // ignore error, best effort there
	return nil
}

func delListIndexItem(tx *bbolt.Tx, item namedBucketItem) error {
	bkt := tx.Bucket(item.name)
	if bkt == nil {
		return nil
	}

	lst, err := decodeList(bkt.Get(item.key))
	if err != nil || len(lst) == 0 {
		return nil
	}

	// remove element from the list
	for i := range lst {
		if bytes.Equal(item.val, lst[i]) {
			copy(lst[i:], lst[i+1:])
			lst = lst[:len(lst)-1]
			break
		}
	}

	// if list empty, remove the key from <list> bucket
	if len(lst) == 0 {
		_ = bkt.Delete(item.key) // ignore error, best effort there

		return nil
	}

	// if list is not empty, then update it
	encodedLst, err := encodeList(lst)
	if err != nil {
		return nil // ignore error, best effort there
	}

	_ = bkt.Put(item.key, encodedLst) // ignore error, best effort there
	return nil
}

func delUniqueIndexes(tx *bbolt.Tx, obj *objectSDK.Object, isParent bool) error {
	addr := object.AddressOf(obj)
	objKey := objectKey(addr.ObjectID())
	addrKey := addressKey(addr)

	// add value to primary unique bucket
	if !isParent {
		var bucketName []byte

		switch obj.Type() {
		case objectSDK.TypeRegular:
			bucketName = primaryBucketName(addr.ContainerID())
		case objectSDK.TypeTombstone:
			bucketName = tombstoneBucketName(addr.ContainerID())
		case objectSDK.TypeStorageGroup:
			bucketName = storageGroupBucketName(addr.ContainerID())
		case objectSDK.TypeLock:
			bucketName = bucketNameLockers(*addr.ContainerID())
		default:
			return ErrUnknownObjectType
		}

		delUniqueIndexItem(tx, namedBucketItem{
			name: bucketName,
			key:  objKey,
		})
	} else {
		delUniqueIndexItem(tx, namedBucketItem{
			name: parentBucketName(obj.ContainerID()),
			key:  objKey,
		})
	}

	delUniqueIndexItem(tx, namedBucketItem{ // remove from small blobovnicza id index
		name: smallBucketName(addr.ContainerID()),
		key:  objKey,
	})
	delUniqueIndexItem(tx, namedBucketItem{ // remove from root index
		name: rootBucketName(addr.ContainerID()),
		key:  objKey,
	})
	delUniqueIndexItem(tx, namedBucketItem{ // remove from ToMoveIt index
		name: toMoveItBucketName,
		key:  addrKey,
	})

	return nil
}
