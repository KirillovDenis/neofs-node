package v2

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	objectV2 "github.com/nspcc-dev/neofs-api-go/v2/object"
	"github.com/nspcc-dev/neofs-api-go/v2/session"
	cidtest "github.com/nspcc-dev/neofs-sdk-go/container/id/test"
	eaclSDK "github.com/nspcc-dev/neofs-sdk-go/eacl"
	"github.com/nspcc-dev/neofs-sdk-go/object"
	objectSDKAddress "github.com/nspcc-dev/neofs-sdk-go/object/address"
	objectSDKID "github.com/nspcc-dev/neofs-sdk-go/object/id"
	"github.com/stretchr/testify/require"
)

type testLocalStorage struct {
	t *testing.T

	expAddr *objectSDKAddress.Address

	obj *object.Object
}

func (s *testLocalStorage) Head(addr *objectSDKAddress.Address) (*object.Object, error) {
	require.True(s.t, addr.ContainerID().Equal(addr.ContainerID()) && addr.ObjectID().Equal(addr.ObjectID()))

	return s.obj, nil
}

func testID(t *testing.T) *objectSDKID.ID {
	cs := [sha256.Size]byte{}

	_, err := rand.Read(cs[:])
	require.NoError(t, err)

	id := objectSDKID.NewID()
	id.SetSHA256(cs)

	return id
}

func testAddress(t *testing.T) *objectSDKAddress.Address {
	addr := objectSDKAddress.NewAddress()
	addr.SetObjectID(testID(t))
	addr.SetContainerID(cidtest.ID())

	return addr
}

func testXHeaders(strs ...string) []session.XHeader {
	res := make([]session.XHeader, len(strs)/2)

	for i := 0; i < len(strs); i += 2 {
		res[i/2].SetKey(strs[i])
		res[i/2].SetValue(strs[i+1])
	}

	return res
}

func TestHeadRequest(t *testing.T) {
	req := new(objectV2.HeadRequest)

	meta := new(session.RequestMetaHeader)
	req.SetMetaHeader(meta)

	body := new(objectV2.HeadRequestBody)
	req.SetBody(body)

	addr := testAddress(t)
	body.SetAddress(addr.ToV2())

	xKey := "x-key"
	xVal := "x-val"
	xHdrs := testXHeaders(
		xKey, xVal,
	)

	meta.SetXHeaders(xHdrs)

	obj := object.New()

	attrKey := "attr_key"
	attrVal := "attr_val"
	var attr object.Attribute
	attr.SetKey(attrKey)
	attr.SetValue(attrVal)
	obj.SetAttributes(attr)

	table := new(eaclSDK.Table)

	priv, err := keys.NewPrivateKey()
	require.NoError(t, err)
	senderKey := priv.PublicKey()

	r := eaclSDK.NewRecord()
	r.SetOperation(eaclSDK.OperationHead)
	r.SetAction(eaclSDK.ActionDeny)
	r.AddFilter(eaclSDK.HeaderFromObject, eaclSDK.MatchStringEqual, attrKey, attrVal)
	r.AddFilter(eaclSDK.HeaderFromRequest, eaclSDK.MatchStringEqual, xKey, xVal)
	eaclSDK.AddFormedTarget(r, eaclSDK.RoleUnknown, (ecdsa.PublicKey)(*senderKey))

	table.AddRecord(r)

	lStorage := &testLocalStorage{
		t:       t,
		expAddr: addr,
		obj:     obj,
	}

	cid := addr.ContainerID()
	unit := new(eaclSDK.ValidationUnit).
		WithContainerID(cid).
		WithOperation(eaclSDK.OperationHead).
		WithSenderKey(senderKey.Bytes()).
		WithHeaderSource(
			NewMessageHeaderSource(
				WithObjectStorage(lStorage),
				WithServiceRequest(req),
			),
		).
		WithEACLTable(table)

	validator := eaclSDK.NewValidator()

	require.Equal(t, eaclSDK.ActionDeny, validator.CalculateAction(unit))

	meta.SetXHeaders(nil)

	require.Equal(t, eaclSDK.ActionAllow, validator.CalculateAction(unit))

	meta.SetXHeaders(xHdrs)

	obj.SetAttributes()

	require.Equal(t, eaclSDK.ActionAllow, validator.CalculateAction(unit))
}
