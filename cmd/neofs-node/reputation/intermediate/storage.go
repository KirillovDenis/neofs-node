package intermediate

import (
	"encoding/hex"
	"fmt"

	"github.com/nspcc-dev/neofs-node/pkg/services/reputation"
	eigentrustcalc "github.com/nspcc-dev/neofs-node/pkg/services/reputation/eigentrust/calculator"
	consumerstorage "github.com/nspcc-dev/neofs-node/pkg/services/reputation/eigentrust/storage/consumers"
	"github.com/nspcc-dev/neofs-node/pkg/services/reputation/eigentrust/storage/daughters"
)

// DaughterTrustIteratorProvider is implementation of the
// reputation/eigentrust/calculator's DaughterTrustIteratorProvider interface.
type DaughterTrustIteratorProvider struct {
	DaughterStorage *daughters.Storage
	ConsumerStorage *consumerstorage.Storage
}

// InitDaughterIterator returns iterator over received
// local trusts for ctx.Epoch() epoch from daughter p.
func (ip *DaughterTrustIteratorProvider) InitDaughterIterator(ctx eigentrustcalc.Context,
	p reputation.PeerID) (eigentrustcalc.TrustIterator, error) {
	epoch := ctx.Epoch()

	daughterIterator, ok := ip.DaughterStorage.DaughterTrusts(epoch, p)
	if !ok {
		return nil, fmt.Errorf("no data in %d epoch for daughter: %s",
			epoch,
			hex.EncodeToString(p.Bytes()),
		)
	}

	return daughterIterator, nil
}

// InitAllDaughtersIterator returns iterator over all
// daughters of the current node(manager) and all local
// trusts received from them for ctx.Epoch() epoch.
func (ip *DaughterTrustIteratorProvider) InitAllDaughtersIterator(
	ctx eigentrustcalc.Context) (eigentrustcalc.PeerTrustsIterator, error) {
	epoch := ctx.Epoch()

	iter, ok := ip.DaughterStorage.AllDaughterTrusts(epoch)
	if !ok {
		return nil, fmt.Errorf("no data in %d epoch for daughters", epoch)
	}

	return iter, nil
}

// InitConsumersIterator returns iterator over all daughters
// of the current node(manager) and all their consumers' local
// trusts for ctx.Epoch() epoch and ctx.I() iteration.
func (ip *DaughterTrustIteratorProvider) InitConsumersIterator(
	ctx eigentrustcalc.Context) (eigentrustcalc.PeerTrustsIterator, error) {
	epoch, iter := ctx.Epoch(), ctx.I()

	consumerIterator, ok := ip.ConsumerStorage.Consumers(epoch, iter)
	if !ok {
		return nil, fmt.Errorf("no data for %d iteration in %d epoch for consumers's trusts",
			iter,
			epoch,
		)
	}

	return consumerIterator, nil
}
