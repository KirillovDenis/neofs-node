package container

import (
	"fmt"

	core "github.com/nspcc-dev/neofs-node/pkg/core/container"
	"github.com/nspcc-dev/neofs-node/pkg/morph/client"
)

// Delete marshals container ID, and passes it to Wrapper's Delete method
// along with signature and session token.
//
// Returns error if container ID is nil.
func Delete(c *Client, witness core.RemovalWitness) error {
	id := witness.ContainerID()
	if id == nil {
		return errNilArgument
	}

	binToken, err := witness.SessionToken().Marshal()
	if err != nil {
		return fmt.Errorf("could not marshal session token: %w", err)
	}

	return c.Delete(
		DeletePrm{
			cid:       id.ToV2().GetValue(),
			signature: witness.Signature(),
			token:     binToken,
		})
}

// DeletePrm groups parameters of Delete client operation.
type DeletePrm struct {
	cid       []byte
	signature []byte
	token     []byte

	client.InvokePrmOptional
}

// SetCID sets container ID.
func (d *DeletePrm) SetCID(cid []byte) {
	d.cid = cid
}

// SetSignature sets signature.
func (d *DeletePrm) SetSignature(signature []byte) {
	d.signature = signature
}

// SetToken sets session token.
func (d *DeletePrm) SetToken(token []byte) {
	d.token = token
}

// Delete removes the container from NeoFS system
// through Container contract call.
//
// Returns any error encountered that caused
// the removal to interrupt.
//
// If TryNotary is provided, calls notary contract.
func (c *Client) Delete(p DeletePrm) error {
	if len(p.signature) == 0 {
		return errNilArgument
	}

	prm := client.InvokePrm{}
	prm.SetMethod(deleteMethod)
	prm.SetArgs(p.cid, p.signature, p.token)
	prm.InvokePrmOptional = p.InvokePrmOptional

	err := c.client.Invoke(prm)
	if err != nil {
		return fmt.Errorf("could not invoke method (%s): %w", deleteMethod, err)
	}
	return nil
}
