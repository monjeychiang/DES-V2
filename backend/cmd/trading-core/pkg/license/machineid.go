package license

import (
	"github.com/denisbrodbeck/machineid"
)

// MachineID fetches a stable identifier for licensing.
func MachineID() (string, error) {
	return machineid.ID()
}
