package cmd

import (
	"fmt"

	internalclient "github.com/nspcc-dev/neofs-node/cmd/neofs-cli/internal/client"
	objectcore "github.com/nspcc-dev/neofs-node/pkg/core/object"
	cid "github.com/nspcc-dev/neofs-sdk-go/container/id"
	"github.com/nspcc-dev/neofs-sdk-go/object"
	oid "github.com/nspcc-dev/neofs-sdk-go/object/id"
	"github.com/spf13/cobra"
)

// object lock command.
var cmdObjectLock = &cobra.Command{
	Use:   "lock CONTAINER OBJECT...",
	Short: "Lock object in container",
	Long:  "Lock object in container",
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		var cnr cid.ID

		err := cnr.Parse(args[0])
		exitOnErr(cmd, errf("Incorrect container arg: %v", err))

		argsList := args[1:]

		lockList := make([]oid.ID, len(argsList))

		for i := range argsList {
			err = lockList[i].Parse(argsList[i])
			exitOnErr(cmd, errf(fmt.Sprintf("Incorrect object arg #%d: %%v", i+1), err))
		}

		key, err := getKey()
		exitOnErr(cmd, errf("can't fetch private key: %w", err))

		idOwner, err := getOwnerID(key)
		exitOnErr(cmd, err)

		var lock object.Lock
		lock.WriteMembers(lockList)

		obj := object.New()
		obj.SetContainerID(&cnr)
		obj.SetOwnerID(idOwner)
		obj.SetType(object.TypeLock)
		obj.SetPayload(lock.Marshal())

		var prm internalclient.PutObjectPrm

		prepareSessionPrmWithOwner(cmd, objectcore.AddressOf(obj), key, idOwner, &prm)
		prepareObjectPrm(cmd, &prm)
		prm.SetHeader(obj)

		_, err = internalclient.PutObject(prm)
		exitOnErr(cmd, errf("Store lock object in NeoFS: %w", err))

		cmd.Println("Objects successfully locked.")
	},
}

func initCommandObjectLock() {
	initCommonFlags(cmdObjectLock)
}
