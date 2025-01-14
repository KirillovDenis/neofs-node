package cmd

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cheggaaa/pb"
	objectV2 "github.com/nspcc-dev/neofs-api-go/v2/object"
	internalclient "github.com/nspcc-dev/neofs-node/cmd/neofs-cli/internal/client"
	"github.com/nspcc-dev/neofs-sdk-go/checksum"
	cid "github.com/nspcc-dev/neofs-sdk-go/container/id"
	"github.com/nspcc-dev/neofs-sdk-go/object"
	addressSDK "github.com/nspcc-dev/neofs-sdk-go/object/address"
	oidSDK "github.com/nspcc-dev/neofs-sdk-go/object/id"
	"github.com/nspcc-dev/neofs-sdk-go/owner"
	"github.com/nspcc-dev/neofs-sdk-go/session"
	"github.com/nspcc-dev/neofs-sdk-go/token"
	"github.com/spf13/cobra"
)

const (
	getRangeCmdUse = "range"

	getRangeCmdShortDesc = "Get payload range data of an object"

	getRangeCmdLongDesc = "Get payload range data of an object"
)

const (
	getRangeHashSaltFlag = "salt"
)

const bearerTokenFlag = "bearer"

const sessionTokenLifetime = 10 // in epochs

var (
	// objectCmd represents the object command
	objectCmd = &cobra.Command{
		Use:   "object",
		Short: "Operations with Objects",
		Long:  `Operations with Objects`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// bind exactly that cmd's flags to
			// the viper before execution
			bindCommonFlags(cmd)
			bindAPIFlags(cmd)
		},
	}

	objectPutCmd = &cobra.Command{
		Use:   "put",
		Short: "Put object to NeoFS",
		Long:  "Put object to NeoFS",
		Run:   putObject,
	}

	objectGetCmd = &cobra.Command{
		Use:   "get",
		Short: "Get object from NeoFS",
		Long:  "Get object from NeoFS",
		Run:   getObject,
	}

	objectDelCmd = &cobra.Command{
		Use:     "delete",
		Aliases: []string{"del"},
		Short:   "Delete object from NeoFS",
		Long:    "Delete object from NeoFS",
		Run:     deleteObject,
	}

	searchFilters []string

	objectSearchCmd = &cobra.Command{
		Use:   "search",
		Short: "Search object",
		Long:  "Search object",
		Run:   searchObject,
	}

	objectHeadCmd = &cobra.Command{
		Use:   "head",
		Short: "Get object header",
		Long:  "Get object header",
		Run:   getObjectHeader,
	}

	objectHashCmd = &cobra.Command{
		Use:   "hash",
		Short: "Get object hash",
		Long:  "Get object hash",
		Run:   getObjectHash,
	}

	objectRangeCmd = &cobra.Command{
		Use:   getRangeCmdUse,
		Short: getRangeCmdShortDesc,
		Long:  getRangeCmdLongDesc,
		Run:   getObjectRange,
	}
)

const notificationFlag = "notify"

const (
	hashSha256 = "sha256"
	hashTz     = "tz"
	rangeSep   = ":"
)

const searchOIDFlag = "oid"

const (
	rawFlag     = "raw"
	rawFlagDesc = "Set raw request option"
)

const putExpiresOnFlag = "expires-on"

const noProgressFlag = "no-progress"

var putExpiredOn uint64

func initObjectPutCmd() {
	initCommonFlags(objectPutCmd)

	flags := objectPutCmd.Flags()

	flags.String("file", "", "File with object payload")
	_ = objectPutCmd.MarkFlagFilename("file")
	_ = objectPutCmd.MarkFlagRequired("file")

	flags.String("cid", "", "Container ID")
	_ = objectPutCmd.MarkFlagRequired("cid")

	flags.String("attributes", "", "User attributes in form of Key1=Value1,Key2=Value2")
	flags.Bool("disable-filename", false, "Do not set well-known filename attribute")
	flags.Bool("disable-timestamp", false, "Do not set well-known timestamp attribute")
	flags.Uint64VarP(&putExpiredOn, putExpiresOnFlag, "e", 0, "Last epoch in the life of the object")
	flags.Bool(noProgressFlag, false, "Do not show progress bar")

	flags.String(notificationFlag, "", "Object notification in the form of *epoch*:*topic*; '-' topic means using default")
}

func initObjectDeleteCmd() {
	initCommonFlags(objectDelCmd)

	flags := objectDelCmd.Flags()

	flags.String("cid", "", "Container ID")
	_ = objectDelCmd.MarkFlagRequired("cid")

	flags.String("oid", "", "Object ID")
	_ = objectDelCmd.MarkFlagRequired("oid")
}

func initObjectGetCmd() {
	initCommonFlags(objectGetCmd)

	flags := objectGetCmd.Flags()

	flags.String("cid", "", "Container ID")
	_ = objectGetCmd.MarkFlagRequired("cid")

	flags.String("oid", "", "Object ID")
	_ = objectGetCmd.MarkFlagRequired("oid")

	flags.String("file", "", "File to write object payload to. Default: stdout.")
	flags.String("header", "", "File to write header to. Default: stdout.")
	flags.Bool(rawFlag, false, rawFlagDesc)
	flags.Bool(noProgressFlag, false, "Do not show progress bar")
}

func initObjectSearchCmd() {
	initCommonFlags(objectSearchCmd)

	flags := objectSearchCmd.Flags()

	flags.String("cid", "", "Container ID")
	_ = objectSearchCmd.MarkFlagRequired("cid")

	flags.StringSliceVarP(&searchFilters, "filters", "f", nil,
		"Repeated filter expressions or files with protobuf JSON")

	flags.Bool("root", false, "Search for user objects")
	flags.Bool("phy", false, "Search physically stored objects")
	flags.String(searchOIDFlag, "", "Search object by identifier")
}

func initObjectHeadCmd() {
	initCommonFlags(objectHeadCmd)

	flags := objectHeadCmd.Flags()

	flags.String("cid", "", "Container ID")
	_ = objectHeadCmd.MarkFlagRequired("cid")

	flags.String("oid", "", "Object ID")
	_ = objectHeadCmd.MarkFlagRequired("oid")

	flags.String("file", "", "File to write header to. Default: stdout.")
	flags.Bool("main-only", false, "Return only main fields")
	flags.Bool("json", false, "Marshal output in JSON")
	flags.Bool("proto", false, "Marshal output in Protobuf")
	flags.Bool(rawFlag, false, rawFlagDesc)
}

func initObjectHashCmd() {
	initCommonFlags(objectHashCmd)

	flags := objectHashCmd.Flags()

	flags.String("cid", "", "Container ID")
	_ = objectHashCmd.MarkFlagRequired("cid")

	flags.String("oid", "", "Object ID")
	_ = objectHashCmd.MarkFlagRequired("oid")

	flags.String("range", "", "Range to take hash from in the form offset1:length1,...")
	flags.String("type", hashSha256, "Hash type. Either 'sha256' or 'tz'")
	flags.String(getRangeHashSaltFlag, "", "Salt in hex format")
}

func initObjectRangeCmd() {
	initCommonFlags(objectRangeCmd)

	flags := objectRangeCmd.Flags()

	flags.String("cid", "", "Container ID")
	_ = objectRangeCmd.MarkFlagRequired("cid")

	flags.String("oid", "", "Object ID")
	_ = objectRangeCmd.MarkFlagRequired("oid")

	flags.String("range", "", "Range to take data from in the form offset:length")
	flags.String("file", "", "File to write object payload to. Default: stdout.")
	flags.Bool(rawFlag, false, rawFlagDesc)
}

func init() {
	objectChildCommands := []*cobra.Command{
		objectPutCmd,
		objectDelCmd,
		objectGetCmd,
		objectSearchCmd,
		objectHeadCmd,
		objectHashCmd,
		objectRangeCmd,
		cmdObjectLock,
	}

	rootCmd.AddCommand(objectCmd)
	objectCmd.AddCommand(objectChildCommands...)

	for _, objCommand := range objectChildCommands {
		flags := objCommand.Flags()

		flags.String(bearerTokenFlag, "", "File with signed JSON or binary encoded bearer token")
		flags.StringSliceVarP(&xHeaders, xHeadersKey, xHeadersShorthand, xHeadersDefault, xHeadersUsage)
		flags.Uint32P(ttl, ttlShorthand, ttlDefault, ttlUsage)
	}

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// objectCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// objectCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	initObjectPutCmd()
	initObjectDeleteCmd()
	initObjectGetCmd()
	initObjectSearchCmd()
	initObjectHeadCmd()
	initObjectHashCmd()
	initObjectRangeCmd()
	initCommandObjectLock()
}

type clientKeySession interface {
	clientWithKey
	SetSessionToken(*session.Token)
}

func prepareSessionPrm(cmd *cobra.Command, addr *addressSDK.Address, prms ...clientKeySession) {
	key, err := getKey()
	exitOnErr(cmd, errf("get private key: %w", err))

	prepareSessionPrmWithKey(cmd, addr, key, prms...)
}

func prepareSessionPrmWithKey(cmd *cobra.Command, addr *addressSDK.Address, key *ecdsa.PrivateKey, prms ...clientKeySession) {
	ownerID, err := getOwnerID(key)
	exitOnErr(cmd, errf("owner ID from key: %w", err))

	prepareSessionPrmWithOwner(cmd, addr, key, ownerID, prms...)
}

func prepareSessionPrmWithOwner(
	cmd *cobra.Command,
	addr *addressSDK.Address,
	key *ecdsa.PrivateKey,
	ownerID *owner.ID,
	prms ...clientKeySession,
) {
	var (
		sessionPrm internalclient.CreateSessionPrm
		netInfoPrm internalclient.NetworkInfoPrm
	)

	cws := make([]clientWithKey, 2, len(prms)+2)
	cws[0] = &sessionPrm
	cws[1] = &netInfoPrm

	for i := range prms {
		cws = append(cws, prms[i])
	}

	prepareAPIClientWithKey(cmd, key, cws...)

	ni, err := internalclient.NetworkInfo(netInfoPrm)
	exitOnErr(cmd, errf("read network info: %w", err))

	cur := ni.NetworkInfo().CurrentEpoch()
	exp := cur + sessionTokenLifetime
	sessionPrm.SetExp(exp)

	sessionRes, err := internalclient.CreateSession(sessionPrm)
	exitOnErr(cmd, errf("open session: %w", err))

	for i := range prms {
		objectContext := session.NewObjectContext()
		switch prms[i].(type) {
		case *internalclient.GetObjectPrm:
			objectContext.ForGet()
		case *internalclient.HeadObjectPrm:
			objectContext.ForHead()
		case *internalclient.PutObjectPrm:
			objectContext.ForPut()
		case *internalclient.DeleteObjectPrm:
			objectContext.ForDelete()
		case *internalclient.SearchObjectsPrm:
			objectContext.ForSearch()
		case *internalclient.PayloadRangePrm:
			objectContext.ForRange()
		case *internalclient.HashPayloadRangesPrm:
			objectContext.ForRangeHash()
		default:
			panic("invalid client parameter type")
		}
		objectContext.ApplyTo(addr)

		tok := session.NewToken()
		tok.SetID(sessionRes.ID())
		tok.SetSessionKey(sessionRes.SessionKey())
		tok.SetOwnerID(ownerID)
		tok.SetContext(objectContext)
		tok.SetExp(exp)
		tok.SetIat(cur)
		tok.SetNbf(cur)

		err = tok.Sign(key)
		exitOnErr(cmd, errf("session token signing: %w", err))

		prms[i].SetSessionToken(tok)
	}
}

type objectPrm interface {
	bearerPrm
	SetTTL(uint32)
	SetXHeaders([]*session.XHeader)
}

func prepareObjectPrm(cmd *cobra.Command, prms ...objectPrm) {
	for i := range prms {
		prepareBearerPrm(cmd, prms[i])

		prms[i].SetTTL(getTTL())
		prms[i].SetXHeaders(parseXHeaders())
	}
}

func prepareObjectPrmRaw(cmd *cobra.Command, prm interface {
	objectPrm
	SetRawFlag(bool)
}) {
	prepareObjectPrm(cmd, prm)

	raw, _ := cmd.Flags().GetBool(rawFlag)
	prm.SetRawFlag(raw)
}

func putObject(cmd *cobra.Command, _ []string) {
	key, err := getKey()
	exitOnErr(cmd, errf("can't fetch private key: %w", err))

	ownerID, err := getOwnerID(key)
	exitOnErr(cmd, err)
	cid, err := getCID(cmd)
	exitOnErr(cmd, err)

	filename := cmd.Flag("file").Value.String()
	f, err := os.OpenFile(filename, os.O_RDONLY, os.ModePerm)
	if err != nil {
		exitOnErr(cmd, fmt.Errorf("can't open file '%s': %w", filename, err))
	}

	attrs, err := parseObjectAttrs(cmd)
	exitOnErr(cmd, errf("can't parse object attributes: %w", err))

	expiresOn, _ := cmd.Flags().GetUint64(putExpiresOnFlag)
	if expiresOn > 0 {
		var expAttrFound bool
		expAttrValue := strconv.FormatUint(expiresOn, 10)

		for i := range attrs {
			if attrs[i].Key() == objectV2.SysAttributeExpEpoch {
				attrs[i].SetValue(expAttrValue)
				expAttrFound = true
				break
			}
		}

		if !expAttrFound {
			index := len(attrs)
			attrs = append(attrs, object.Attribute{})
			attrs[index].SetKey(objectV2.SysAttributeExpEpoch)
			attrs[index].SetValue(expAttrValue)
		}
	}

	obj := object.New()
	obj.SetContainerID(cid)
	obj.SetOwnerID(ownerID)
	obj.SetAttributes(attrs...)

	notificationInfo, err := parseObjectNotifications(cmd)
	exitOnErr(cmd, errf("can't parse object notification information: %w", err))

	if notificationInfo != nil {
		obj.SetNotification(*notificationInfo)
	}

	var prm internalclient.PutObjectPrm

	sessionObjectCtxAddress := addressSDK.NewAddress()
	sessionObjectCtxAddress.SetContainerID(cid)
	prepareSessionPrmWithOwner(cmd, sessionObjectCtxAddress, key, ownerID, &prm)
	prepareObjectPrm(cmd, &prm)
	prm.SetHeader(obj)

	var p *pb.ProgressBar

	noProgress, _ := cmd.Flags().GetBool(noProgressFlag)
	if noProgress {
		prm.SetPayloadReader(f)
	} else {
		fi, err := f.Stat()
		if err != nil {
			cmd.PrintErrf("Failed to get file size, progress bar is disabled: %v\n", err)
			prm.SetPayloadReader(f)
		} else {
			p = pb.New64(fi.Size())
			p.Output = cmd.OutOrStdout()
			prm.SetPayloadReader(p.NewProxyReader(f))
			p.Start()
		}
	}

	res, err := internalclient.PutObject(prm)
	exitOnErr(cmd, errf("rpc error: %w", err))

	if p != nil {
		p.Finish()
	}
	cmd.Printf("[%s] Object successfully stored\n", filename)
	cmd.Printf("  ID: %s\n  CID: %s\n", res.ID(), cid)
}

func deleteObject(cmd *cobra.Command, _ []string) {
	objAddr, err := getObjectAddress(cmd)
	exitOnErr(cmd, err)

	var prm internalclient.DeleteObjectPrm

	prepareSessionPrm(cmd, objAddr, &prm)
	prepareObjectPrm(cmd, &prm)
	prm.SetAddress(objAddr)

	res, err := internalclient.DeleteObject(prm)
	exitOnErr(cmd, errf("rpc error: %w", err))

	tombstoneAddr := res.TombstoneAddress()

	cmd.Println("Object removed successfully.")
	cmd.Printf("  ID: %s\n  CID: %s\n", tombstoneAddr.ObjectID(), tombstoneAddr.ContainerID())
}

func getObject(cmd *cobra.Command, _ []string) {
	objAddr, err := getObjectAddress(cmd)
	exitOnErr(cmd, err)

	var out io.Writer
	filename := cmd.Flag("file").Value.String()
	if filename == "" {
		out = os.Stdout
	} else {
		f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, os.ModePerm)
		if err != nil {
			exitOnErr(cmd, fmt.Errorf("can't open file '%s': %w", filename, err))
		}

		defer f.Close()

		out = f
	}

	var prm internalclient.GetObjectPrm

	prepareSessionPrm(cmd, objAddr, &prm)
	prepareObjectPrmRaw(cmd, &prm)
	prm.SetAddress(objAddr)

	var p *pb.ProgressBar
	noProgress, _ := cmd.Flags().GetBool(noProgressFlag)

	if filename == "" || noProgress {
		prm.SetPayloadWriter(out)
	} else {
		p = pb.New64(0)
		p.Output = cmd.OutOrStdout()
		prm.SetPayloadWriter(p.NewProxyWriter(out))
		prm.SetHeaderCallback(func(o *object.Object) {
			p.SetTotal64(int64(o.PayloadSize()))
			p.Start()
		})
	}

	res, err := internalclient.GetObject(prm)
	if err != nil {
		if ok := printSplitInfoErr(cmd, err); ok {
			return
		}

		exitOnErr(cmd, errf("rpc error: %w", err))
	}

	hdrFile := cmd.Flag("header").Value.String()
	if filename != "" {
		if p != nil {
			p.Finish()
		}
		if hdrFile != "" || !strictOutput(cmd) {
			cmd.Printf("[%s] Object successfully saved\n", filename)
		}
	}

	// Print header only if file is not streamed to stdout.
	if filename != "" || hdrFile != "" {
		err = saveAndPrintHeader(cmd, res.Header(), hdrFile)
		exitOnErr(cmd, err)
	}
}

func getObjectHeader(cmd *cobra.Command, _ []string) {
	objAddr, err := getObjectAddress(cmd)
	exitOnErr(cmd, err)

	mainOnly, _ := cmd.Flags().GetBool("main-only")

	var prm internalclient.HeadObjectPrm

	prepareSessionPrm(cmd, objAddr, &prm)
	prepareObjectPrmRaw(cmd, &prm)
	prm.SetAddress(objAddr)
	prm.SetMainOnlyFlag(mainOnly)

	res, err := internalclient.HeadObject(prm)
	if err != nil {
		if ok := printSplitInfoErr(cmd, err); ok {
			return
		}

		exitOnErr(cmd, errf("rpc error: %w", err))
	}

	err = saveAndPrintHeader(cmd, res.Header(), cmd.Flag("file").Value.String())
	exitOnErr(cmd, err)
}

func searchObject(cmd *cobra.Command, _ []string) {
	cid, err := getCID(cmd)
	exitOnErr(cmd, err)

	sf, err := parseSearchFilters(cmd)
	exitOnErr(cmd, err)

	var prm internalclient.SearchObjectsPrm

	sessionObjectCtxAddress := addressSDK.NewAddress()
	sessionObjectCtxAddress.SetContainerID(cid)
	prepareSessionPrm(cmd, sessionObjectCtxAddress, &prm)
	prepareObjectPrm(cmd, &prm)
	prm.SetContainerID(cid)
	prm.SetFilters(sf)

	res, err := internalclient.SearchObjects(prm)
	exitOnErr(cmd, errf("rpc error: %w", err))

	ids := res.IDList()

	cmd.Printf("Found %d objects.\n", len(ids))
	for _, id := range ids {
		cmd.Println(id)
	}
}

func getObjectHash(cmd *cobra.Command, _ []string) {
	objAddr, err := getObjectAddress(cmd)
	exitOnErr(cmd, err)
	ranges, err := getRangeList(cmd)
	exitOnErr(cmd, err)
	typ, err := getHashType(cmd)
	exitOnErr(cmd, err)

	strSalt := strings.TrimPrefix(cmd.Flag(getRangeHashSaltFlag).Value.String(), "0x")

	salt, err := hex.DecodeString(strSalt)
	exitOnErr(cmd, errf("could not decode salt: %w", err))

	var (
		hashPrm internalclient.HashPayloadRangesPrm
		headPrm internalclient.HeadObjectPrm

		sesPrms = []clientKeySession{&hashPrm}
		objPrms = []objectPrm{&hashPrm}
	)

	fullHash := len(ranges) == 0
	if fullHash {
		sesPrms = append(sesPrms, &headPrm)
		objPrms = append(objPrms, &headPrm)
	}

	prepareSessionPrm(cmd, objAddr, sesPrms...)
	prepareObjectPrm(cmd, objPrms...)

	tz := typ == hashTz

	if fullHash {
		headPrm.SetAddress(objAddr)

		// get hash of full payload through HEAD (may be user can do it through dedicated command?)
		res, err := internalclient.HeadObject(headPrm)
		exitOnErr(cmd, errf("rpc error: %w", err))

		var cs *checksum.Checksum

		if tz {
			cs = res.Header().PayloadHomomorphicHash()
		} else {
			cs = res.Header().PayloadChecksum()
		}

		cmd.Println(hex.EncodeToString(cs.Sum()))

		return
	}

	hashPrm.SetAddress(objAddr)
	hashPrm.SetSalt(salt)
	hashPrm.SetRanges(ranges)

	if tz {
		hashPrm.TZ()
	}

	res, err := internalclient.HashPayloadRanges(hashPrm)
	exitOnErr(cmd, errf("rpc error: %w", err))

	hs := res.HashList()

	for i := range hs {
		cmd.Printf("Offset=%d (Length=%d)\t: %s\n", ranges[i].GetOffset(), ranges[i].GetLength(),
			hex.EncodeToString(hs[i]))
	}
}

func getOwnerID(key *ecdsa.PrivateKey) (*owner.ID, error) {
	return owner.NewIDFromPublicKey(&key.PublicKey), nil
}

var searchUnaryOpVocabulary = map[string]object.SearchMatchType{
	"NOPRESENT": object.MatchNotPresent,
}

var searchBinaryOpVocabulary = map[string]object.SearchMatchType{
	"EQ":            object.MatchStringEqual,
	"NE":            object.MatchStringNotEqual,
	"COMMON_PREFIX": object.MatchCommonPrefix,
}

func parseSearchFilters(cmd *cobra.Command) (object.SearchFilters, error) {
	var fs object.SearchFilters

	for i := range searchFilters {
		words := strings.Fields(searchFilters[i])

		switch len(words) {
		default:
			return nil, fmt.Errorf("invalid field number: %d", len(words))
		case 1:
			data, err := os.ReadFile(words[0])
			if err != nil {
				return nil, fmt.Errorf("could not read attributes filter from file: %w", err)
			}

			subFs := object.NewSearchFilters()

			if err := subFs.UnmarshalJSON(data); err != nil {
				return nil, fmt.Errorf("could not unmarshal attributes filter from file: %w", err)
			}

			fs = append(fs, subFs...)
		case 2:
			m, ok := searchUnaryOpVocabulary[words[1]]
			if !ok {
				return nil, fmt.Errorf("unsupported unary op: %s", words[1])
			}

			fs.AddFilter(words[0], "", m)
		case 3:
			m, ok := searchBinaryOpVocabulary[words[1]]
			if !ok {
				return nil, fmt.Errorf("unsupported binary op: %s", words[1])
			}

			fs.AddFilter(words[0], words[2], m)
		}
	}

	root, _ := cmd.Flags().GetBool("root")
	if root {
		fs.AddRootFilter()
	}

	phy, _ := cmd.Flags().GetBool("phy")
	if phy {
		fs.AddPhyFilter()
	}

	oid, _ := cmd.Flags().GetString(searchOIDFlag)
	if oid != "" {
		id := oidSDK.NewID()
		if err := id.Parse(oid); err != nil {
			return nil, fmt.Errorf("could not parse object ID: %w", err)
		}

		fs.AddObjectIDFilter(object.MatchStringEqual, id)
	}

	return fs, nil
}

func parseObjectAttrs(cmd *cobra.Command) ([]object.Attribute, error) {
	var rawAttrs []string

	raw := cmd.Flag("attributes").Value.String()
	if len(raw) != 0 {
		rawAttrs = strings.Split(raw, ",")
	}

	attrs := make([]object.Attribute, len(rawAttrs), len(rawAttrs)+2) // name + timestamp attributes
	for i := range rawAttrs {
		kv := strings.SplitN(rawAttrs[i], "=", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid attribute format: %s", rawAttrs[i])
		}
		attrs[i].SetKey(kv[0])
		attrs[i].SetValue(kv[1])
	}

	disableFilename, _ := cmd.Flags().GetBool("disable-filename")
	if !disableFilename {
		filename := filepath.Base(cmd.Flag("file").Value.String())
		index := len(attrs)
		attrs = append(attrs, object.Attribute{})
		attrs[index].SetKey(object.AttributeFileName)
		attrs[index].SetValue(filename)
	}

	disableTime, _ := cmd.Flags().GetBool("disable-timestamp")
	if !disableTime {
		index := len(attrs)
		attrs = append(attrs, object.Attribute{})
		attrs[index].SetKey(object.AttributeTimestamp)
		attrs[index].SetValue(strconv.FormatInt(time.Now().Unix(), 10))
	}

	return attrs, nil
}

func parseObjectNotifications(cmd *cobra.Command) (*object.NotificationInfo, error) {
	const (
		separator       = ":"
		useDefaultTopic = "-"
	)

	raw := cmd.Flag(notificationFlag).Value.String()
	if raw == "" {
		return nil, nil
	}

	rawSlice := strings.SplitN(raw, separator, 2)
	if len(rawSlice) != 2 {
		return nil, fmt.Errorf("notification must be in the form of: *epoch*%s*topic*, got %s", separator, raw)
	}

	ni := new(object.NotificationInfo)

	epoch, err := strconv.ParseUint(rawSlice[0], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("could not parse notification epoch %s: %w", rawSlice[0], err)
	}

	ni.SetEpoch(epoch)

	if rawSlice[1] == "" {
		return nil, fmt.Errorf("incorrect empty topic: use %s to force using default topic", useDefaultTopic)
	}

	if rawSlice[1] != useDefaultTopic {
		ni.SetTopic(rawSlice[1])
	}

	return ni, nil
}

func getCID(cmd *cobra.Command) (*cid.ID, error) {
	id := cid.New()

	err := id.Parse(cmd.Flag("cid").Value.String())
	if err != nil {
		return nil, fmt.Errorf("could not parse container ID: %w", err)
	}

	return id, nil
}

func getOID(cmd *cobra.Command) (*oidSDK.ID, error) {
	oid := oidSDK.NewID()

	err := oid.Parse(cmd.Flag("oid").Value.String())
	if err != nil {
		return nil, fmt.Errorf("could not parse object ID: %w", err)
	}

	return oid, nil
}

func getObjectAddress(cmd *cobra.Command) (*addressSDK.Address, error) {
	cid, err := getCID(cmd)
	if err != nil {
		return nil, err
	}
	oid, err := getOID(cmd)
	if err != nil {
		return nil, err
	}

	objAddr := addressSDK.NewAddress()
	objAddr.SetContainerID(cid)
	objAddr.SetObjectID(oid)
	return objAddr, nil
}

func getRangeList(cmd *cobra.Command) ([]*object.Range, error) {
	v := cmd.Flag("range").Value.String()
	if len(v) == 0 {
		return nil, nil
	}
	vs := strings.Split(v, ",")
	rs := make([]*object.Range, len(vs))
	for i := range vs {
		r := strings.Split(vs[i], rangeSep)
		if len(r) != 2 {
			return nil, fmt.Errorf("invalid range specifier: %s", vs[i])
		}

		offset, err := strconv.ParseUint(r[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid range specifier: %s", vs[i])
		}
		length, err := strconv.ParseUint(r[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid range specifier: %s", vs[i])
		}
		rs[i] = object.NewRange()
		rs[i].SetOffset(offset)
		rs[i].SetLength(length)
	}
	return rs, nil
}

func getHashType(cmd *cobra.Command) (string, error) {
	rawType := cmd.Flag("type").Value.String()
	switch typ := strings.ToLower(rawType); typ {
	case hashSha256, hashTz:
		return typ, nil
	default:
		return "", fmt.Errorf("invalid hash type: %s", typ)
	}
}

func saveAndPrintHeader(cmd *cobra.Command, obj *object.Object, filename string) error {
	bs, err := marshalHeader(cmd, obj)
	if err != nil {
		return fmt.Errorf("could not marshal header: %w", err)
	}
	if len(bs) != 0 {
		if filename == "" {
			cmd.Println(string(bs))
			return nil
		}
		err = os.WriteFile(filename, bs, os.ModePerm)
		if err != nil {
			return fmt.Errorf("could not write header to file: %w", err)
		}
		cmd.Printf("[%s] Header successfully saved.", filename)
	}

	return printHeader(cmd, obj)
}

func printHeader(cmd *cobra.Command, obj *object.Object) error {
	cmd.Printf("ID: %s\n", obj.ID())
	cmd.Printf("CID: %s\n", obj.ContainerID())
	cmd.Printf("Owner: %s\n", obj.OwnerID())
	cmd.Printf("CreatedAt: %d\n", obj.CreationEpoch())
	cmd.Printf("Size: %d\n", obj.PayloadSize())
	cmd.Printf("HomoHash: %s\n", hex.EncodeToString(obj.PayloadHomomorphicHash().Sum()))
	cmd.Printf("Checksum: %s\n", hex.EncodeToString(obj.PayloadChecksum().Sum()))
	cmd.Printf("Type: %s\n", obj.Type())

	cmd.Println("Attributes:")
	for _, attr := range obj.Attributes() {
		if attr.Key() == object.AttributeTimestamp {
			cmd.Printf("  %s=%s (%s)\n",
				attr.Key(),
				attr.Value(),
				prettyPrintUnixTime(attr.Value()))
			continue
		}
		cmd.Printf("  %s=%s\n", attr.Key(), attr.Value())
	}

	return printSplitHeader(cmd, obj)
}

func printSplitHeader(cmd *cobra.Command, obj *object.Object) error {
	if splitID := obj.SplitID(); splitID != nil {
		cmd.Printf("Split ID: %s\n", splitID)
	}

	if oid := obj.ParentID(); oid != nil {
		cmd.Printf("Split ParentID: %s\n", oid)
	}

	if prev := obj.PreviousID(); prev != nil {
		cmd.Printf("Split PreviousID: %s\n", prev)
	}

	for _, child := range obj.Children() {
		cmd.Printf("Split ChildID: %s\n", child.String())
	}

	if signature := obj.Signature(); signature != nil {
		cmd.Print("Split Header Signature:\n")
		cmd.Printf("  public key: %s\n", hex.EncodeToString(signature.Key()))
		cmd.Printf("  signature: %s\n", hex.EncodeToString(signature.Sign()))
	}

	parent := obj.Parent()
	if parent != nil {
		cmd.Print("\nSplit Parent Header:\n")

		return printHeader(cmd, parent)
	}

	return nil
}

func strictOutput(cmd *cobra.Command) bool {
	toJSON, _ := cmd.Flags().GetBool("json")
	toProto, _ := cmd.Flags().GetBool("proto")
	return toJSON || toProto
}

func marshalHeader(cmd *cobra.Command, hdr *object.Object) ([]byte, error) {
	toJSON, _ := cmd.Flags().GetBool("json")
	toProto, _ := cmd.Flags().GetBool("proto")
	switch {
	case toJSON && toProto:
		return nil, errors.New("'--json' and '--proto' flags are mutually exclusive")
	case toJSON:
		return hdr.MarshalJSON()
	case toProto:
		return hdr.Marshal()
	default:
		return nil, nil
	}
}

func getBearerToken(cmd *cobra.Command, flagname string) (*token.BearerToken, error) {
	path, err := cmd.Flags().GetString(flagname)
	if err != nil || len(path) == 0 {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("can't read bearer token file: %w", err)
	}

	tok := token.NewBearerToken()
	if err := tok.UnmarshalJSON(data); err != nil {
		if err = tok.Unmarshal(data); err != nil {
			return nil, fmt.Errorf("can't decode bearer token: %w", err)
		}

		printVerbose("Using binary encoded bearer token")
	} else {
		printVerbose("Using JSON encoded bearer token")
	}

	return tok, nil
}

func getObjectRange(cmd *cobra.Command, _ []string) {
	objAddr, err := getObjectAddress(cmd)
	exitOnErr(cmd, err)

	ranges, err := getRangeList(cmd)
	exitOnErr(cmd, err)

	if len(ranges) != 1 {
		exitOnErr(cmd, fmt.Errorf("exactly one range must be specified, got: %d", len(ranges)))
	}

	var out io.Writer

	filename := cmd.Flag("file").Value.String()
	if filename == "" {
		out = os.Stdout
	} else {
		f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, os.ModePerm)
		if err != nil {
			exitOnErr(cmd, fmt.Errorf("can't open file '%s': %w", filename, err))
		}

		defer f.Close()

		out = f
	}

	var prm internalclient.PayloadRangePrm

	prepareSessionPrm(cmd, objAddr, &prm)
	prepareObjectPrmRaw(cmd, &prm)
	prm.SetAddress(objAddr)
	prm.SetRange(ranges[0])
	prm.SetPayloadWriter(out)

	_, err = internalclient.PayloadRange(prm)
	if err != nil {
		if ok := printSplitInfoErr(cmd, err); ok {
			return
		}

		exitOnErr(cmd, fmt.Errorf("can't get object payload range: %w", err))
	}

	if filename != "" {
		cmd.Printf("[%s] Payload successfully saved\n", filename)
	}
}

func printSplitInfoErr(cmd *cobra.Command, err error) bool {
	var errSplitInfo *object.SplitInfoError

	ok := errors.As(err, &errSplitInfo)

	if ok {
		cmd.PrintErrln("Object is complex, split information received.")
		printSplitInfo(cmd, errSplitInfo.SplitInfo())
	}

	return ok
}

func printSplitInfo(cmd *cobra.Command, info *object.SplitInfo) {
	bs, err := marshalSplitInfo(cmd, info)
	exitOnErr(cmd, errf("can't marshal split info: %w", err))

	cmd.Println(string(bs))
}

func marshalSplitInfo(cmd *cobra.Command, info *object.SplitInfo) ([]byte, error) {
	toJSON, _ := cmd.Flags().GetBool("json")
	toProto, _ := cmd.Flags().GetBool("proto")
	switch {
	case toJSON && toProto:
		return nil, errors.New("'--json' and '--proto' flags are mutually exclusive")
	case toJSON:
		return info.MarshalJSON()
	case toProto:
		return info.Marshal()
	default:
		b := bytes.NewBuffer(nil)
		if splitID := info.SplitID(); splitID != nil {
			b.WriteString("Split ID: " + splitID.String() + "\n")
		}
		if link := info.Link(); link != nil {
			b.WriteString("Linking object: " + link.String() + "\n")
		}
		if last := info.LastPart(); last != nil {
			b.WriteString("Last object: " + last.String() + "\n")
		}
		return b.Bytes(), nil
	}
}
