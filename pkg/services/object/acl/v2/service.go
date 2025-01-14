package v2

import (
	"context"
	"errors"
	"fmt"

	objectV2 "github.com/nspcc-dev/neofs-api-go/v2/object"
	"github.com/nspcc-dev/neofs-node/pkg/core/container"
	netmapClient "github.com/nspcc-dev/neofs-node/pkg/morph/client/netmap"
	"github.com/nspcc-dev/neofs-node/pkg/services/object"
	cidSDK "github.com/nspcc-dev/neofs-sdk-go/container/id"
	eaclSDK "github.com/nspcc-dev/neofs-sdk-go/eacl"
	sessionSDK "github.com/nspcc-dev/neofs-sdk-go/session"
	"go.uber.org/zap"
)

// Service checks basic ACL rules.
type Service struct {
	*cfg

	c senderClassifier
}

type putStreamBasicChecker struct {
	source *Service
	next   object.PutObjectStream
}

type getStreamBasicChecker struct {
	checker ACLChecker

	object.GetObjectStream

	info RequestInfo
}

type rangeStreamBasicChecker struct {
	checker ACLChecker

	object.GetObjectRangeStream

	info RequestInfo
}

type searchStreamBasicChecker struct {
	checker ACLChecker

	object.SearchStream

	info RequestInfo
}

// Option represents Service constructor option.
type Option func(*cfg)

type cfg struct {
	log *zap.Logger

	containers container.Source

	checker ACLChecker

	irFetcher InnerRingFetcher

	nm *netmapClient.Client

	next object.ServiceServer
}

func defaultCfg() *cfg {
	return &cfg{
		log: zap.L(),
	}
}

// New is a constructor for object ACL checking service.
func New(opts ...Option) Service {
	cfg := defaultCfg()

	for i := range opts {
		opts[i](cfg)
	}

	panicOnNil := func(v interface{}, name string) {
		if v == nil {
			panic(fmt.Sprintf("ACL service: %s is nil", name))
		}
	}

	panicOnNil(cfg.next, "next Service")
	panicOnNil(cfg.nm, "netmap client")
	panicOnNil(cfg.irFetcher, "inner Ring fetcher")
	panicOnNil(cfg.checker, "acl checker")
	panicOnNil(cfg.containers, "container source")

	return Service{
		cfg: cfg,
		c: senderClassifier{
			log:       cfg.log,
			innerRing: cfg.irFetcher,
			netmap:    cfg.nm,
		},
	}
}

// Get implements ServiceServer interface, makes ACL checks and calls
// next Get method in the ServiceServer pipeline.
func (b Service) Get(request *objectV2.GetRequest, stream object.GetObjectStream) error {
	cid, err := getContainerIDFromRequest(request)
	if err != nil {
		return err
	}

	sTok := originalSessionToken(request.GetMetaHeader())

	req := MetaWithToken{
		vheader: request.GetVerificationHeader(),
		token:   sTok,
		bearer:  originalBearerToken(request.GetMetaHeader()),
		src:     request,
	}

	reqInfo, err := b.findRequestInfo(req, cid, eaclSDK.OperationGet)
	if err != nil {
		return err
	}

	reqInfo.oid = getObjectIDFromRequestBody(request.GetBody())
	useObjectIDFromSession(&reqInfo, sTok)

	if !b.checker.CheckBasicACL(reqInfo) {
		return basicACLErr(reqInfo)
	} else if err := b.checker.CheckEACL(request, reqInfo); err != nil {
		return eACLErr(reqInfo, err)
	}

	return b.next.Get(request, &getStreamBasicChecker{
		GetObjectStream: stream,
		info:            reqInfo,
		checker:         b.checker,
	})
}

func (b Service) Put(ctx context.Context) (object.PutObjectStream, error) {
	streamer, err := b.next.Put(ctx)

	return putStreamBasicChecker{
		source: &b,
		next:   streamer,
	}, err
}

func (b Service) Head(
	ctx context.Context,
	request *objectV2.HeadRequest) (*objectV2.HeadResponse, error) {
	cid, err := getContainerIDFromRequest(request)
	if err != nil {
		return nil, err
	}

	sTok := originalSessionToken(request.GetMetaHeader())

	req := MetaWithToken{
		vheader: request.GetVerificationHeader(),
		token:   sTok,
		bearer:  originalBearerToken(request.GetMetaHeader()),
		src:     request,
	}

	reqInfo, err := b.findRequestInfo(req, cid, eaclSDK.OperationHead)
	if err != nil {
		return nil, err
	}

	reqInfo.oid = getObjectIDFromRequestBody(request.GetBody())
	useObjectIDFromSession(&reqInfo, sTok)

	if !b.checker.CheckBasicACL(reqInfo) {
		return nil, basicACLErr(reqInfo)
	} else if err := b.checker.CheckEACL(request, reqInfo); err != nil {
		return nil, eACLErr(reqInfo, err)
	}

	resp, err := b.next.Head(ctx, request)
	if err == nil {
		if err = b.checker.CheckEACL(resp, reqInfo); err != nil {
			err = eACLErr(reqInfo, err)
		}
	}

	return resp, err
}

func (b Service) Search(request *objectV2.SearchRequest, stream object.SearchStream) error {
	var id *cidSDK.ID

	id, err := getContainerIDFromRequest(request)
	if err != nil {
		return err
	}

	req := MetaWithToken{
		vheader: request.GetVerificationHeader(),
		token:   originalSessionToken(request.GetMetaHeader()),
		bearer:  originalBearerToken(request.GetMetaHeader()),
		src:     request,
	}

	reqInfo, err := b.findRequestInfo(req, id, eaclSDK.OperationSearch)
	if err != nil {
		return err
	}

	reqInfo.oid = getObjectIDFromRequestBody(request.GetBody())

	if !b.checker.CheckBasicACL(reqInfo) {
		return basicACLErr(reqInfo)
	} else if err := b.checker.CheckEACL(request, reqInfo); err != nil {
		return eACLErr(reqInfo, err)
	}

	return b.next.Search(request, &searchStreamBasicChecker{
		checker:      b.checker,
		SearchStream: stream,
		info:         reqInfo,
	})
}

func (b Service) Delete(
	ctx context.Context,
	request *objectV2.DeleteRequest) (*objectV2.DeleteResponse, error) {
	cid, err := getContainerIDFromRequest(request)
	if err != nil {
		return nil, err
	}

	sTok := originalSessionToken(request.GetMetaHeader())

	req := MetaWithToken{
		vheader: request.GetVerificationHeader(),
		token:   sTok,
		bearer:  originalBearerToken(request.GetMetaHeader()),
		src:     request,
	}

	reqInfo, err := b.findRequestInfo(req, cid, eaclSDK.OperationDelete)
	if err != nil {
		return nil, err
	}

	reqInfo.oid = getObjectIDFromRequestBody(request.GetBody())
	useObjectIDFromSession(&reqInfo, sTok)

	if !b.checker.CheckBasicACL(reqInfo) {
		return nil, basicACLErr(reqInfo)
	} else if err := b.checker.CheckEACL(request, reqInfo); err != nil {
		return nil, eACLErr(reqInfo, err)
	}

	return b.next.Delete(ctx, request)
}

func (b Service) GetRange(request *objectV2.GetRangeRequest, stream object.GetObjectRangeStream) error {
	cid, err := getContainerIDFromRequest(request)
	if err != nil {
		return err
	}

	sTok := originalSessionToken(request.GetMetaHeader())

	req := MetaWithToken{
		vheader: request.GetVerificationHeader(),
		token:   sTok,
		bearer:  originalBearerToken(request.GetMetaHeader()),
		src:     request,
	}

	reqInfo, err := b.findRequestInfo(req, cid, eaclSDK.OperationRange)
	if err != nil {
		return err
	}

	reqInfo.oid = getObjectIDFromRequestBody(request.GetBody())
	useObjectIDFromSession(&reqInfo, sTok)

	if !b.checker.CheckBasicACL(reqInfo) {
		return basicACLErr(reqInfo)
	} else if err := b.checker.CheckEACL(request, reqInfo); err != nil {
		return eACLErr(reqInfo, err)
	}

	return b.next.GetRange(request, &rangeStreamBasicChecker{
		checker:              b.checker,
		GetObjectRangeStream: stream,
		info:                 reqInfo,
	})
}

func (b Service) GetRangeHash(
	ctx context.Context,
	request *objectV2.GetRangeHashRequest) (*objectV2.GetRangeHashResponse, error) {
	cid, err := getContainerIDFromRequest(request)
	if err != nil {
		return nil, err
	}

	sTok := originalSessionToken(request.GetMetaHeader())

	req := MetaWithToken{
		vheader: request.GetVerificationHeader(),
		token:   sTok,
		bearer:  originalBearerToken(request.GetMetaHeader()),
		src:     request,
	}

	reqInfo, err := b.findRequestInfo(req, cid, eaclSDK.OperationRangeHash)
	if err != nil {
		return nil, err
	}

	reqInfo.oid = getObjectIDFromRequestBody(request.GetBody())
	useObjectIDFromSession(&reqInfo, sTok)

	if !b.checker.CheckBasicACL(reqInfo) {
		return nil, basicACLErr(reqInfo)
	} else if err := b.checker.CheckEACL(request, reqInfo); err != nil {
		return nil, eACLErr(reqInfo, err)
	}

	return b.next.GetRangeHash(ctx, request)
}

func (p putStreamBasicChecker) Send(request *objectV2.PutRequest) error {
	body := request.GetBody()
	if body == nil {
		return ErrMalformedRequest
	}

	part := body.GetObjectPart()
	if part, ok := part.(*objectV2.PutObjectPartInit); ok {
		cid, err := getContainerIDFromRequest(request)
		if err != nil {
			return err
		}

		ownerID, err := getObjectOwnerFromMessage(request)
		if err != nil {
			return err
		}

		sTok := sessionSDK.NewTokenFromV2(request.GetMetaHeader().GetSessionToken())

		req := MetaWithToken{
			vheader: request.GetVerificationHeader(),
			token:   sTok,
			bearer:  originalBearerToken(request.GetMetaHeader()),
			src:     request,
		}

		reqInfo, err := p.source.findRequestInfo(req, cid, eaclSDK.OperationPut)
		if err != nil {
			return err
		}

		reqInfo.oid = getObjectIDFromRequestBody(part)
		useObjectIDFromSession(&reqInfo, sTok)

		if !p.source.checker.CheckBasicACL(reqInfo) || !p.source.checker.StickyBitCheck(reqInfo, ownerID) {
			return basicACLErr(reqInfo)
		} else if err := p.source.checker.CheckEACL(request, reqInfo); err != nil {
			return eACLErr(reqInfo, err)
		}
	}

	return p.next.Send(request)
}

func (p putStreamBasicChecker) CloseAndRecv() (*objectV2.PutResponse, error) {
	return p.next.CloseAndRecv()
}

func (g *getStreamBasicChecker) Send(resp *objectV2.GetResponse) error {
	if _, ok := resp.GetBody().GetObjectPart().(*objectV2.GetObjectPartInit); ok {
		if err := g.checker.CheckEACL(resp, g.info); err != nil {
			return eACLErr(g.info, err)
		}
	}

	return g.GetObjectStream.Send(resp)
}

func (g *rangeStreamBasicChecker) Send(resp *objectV2.GetRangeResponse) error {
	if err := g.checker.CheckEACL(resp, g.info); err != nil {
		return eACLErr(g.info, err)
	}

	return g.GetObjectRangeStream.Send(resp)
}

func (g *searchStreamBasicChecker) Send(resp *objectV2.SearchResponse) error {
	if err := g.checker.CheckEACL(resp, g.info); err != nil {
		return eACLErr(g.info, err)
	}

	return g.SearchStream.Send(resp)
}

func (b Service) findRequestInfo(
	req MetaWithToken,
	cid *cidSDK.ID,
	op eaclSDK.Operation) (info RequestInfo, err error) {
	cnr, err := b.containers.Get(cid) // fetch actual container
	if err != nil {
		return info, err
	} else if cnr.OwnerID() == nil {
		return info, errors.New("missing owner in container descriptor")
	}

	// find request role and key
	res, err := b.c.classify(req, cid, cnr)
	if err != nil {
		return info, err
	}

	if res.role == eaclSDK.RoleUnknown {
		return info, ErrUnknownRole
	}

	// find verb from token if it is present
	verb, isUnknown := sourceVerbOfRequest(req.token, op)
	if !isUnknown && verb != op && !isVerbCompatible(verb, op) {
		return info, ErrInvalidVerb
	}

	info.basicACL = cnr.BasicACL()
	info.requestRole = res.role
	info.isInnerRing = res.isIR
	info.operation = verb
	info.cnrOwner = cnr.OwnerID()
	info.idCnr = cid

	// it is assumed that at the moment the key will be valid,
	// otherwise the request would not pass validation
	info.senderKey = res.key

	// add bearer token if it is present in request
	info.bearer = req.bearer

	info.srcRequest = req.src

	return info, nil
}
