package shard

import (
	"errors"

	apistatus "github.com/nspcc-dev/neofs-sdk-go/client/status"
)

// IsErrNotFound checks if error returned by Shard Get/Head/GetRange method
// corresponds to missing object.
func IsErrNotFound(err error) bool {
	return errors.As(err, new(apistatus.ObjectNotFound))
}

// IsErrRemoved checks if error returned by Shard Exists/Get/Head/GetRange method
// corresponds to removed object.
func IsErrRemoved(err error) bool {
	return errors.As(err, new(apistatus.ObjectAlreadyRemoved))
}
