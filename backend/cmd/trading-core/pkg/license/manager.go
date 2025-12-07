package license

import (
	"fmt"
	"time"
)

// Manager validates tokens against the current machine id.
type Manager struct {
	Secret string
}

func NewManager(secret string) *Manager {
	return &Manager{Secret: secret}
}

func (m *Manager) Validate(token string) error {
	mid, err := MachineID()
	if err != nil {
		return fmt.Errorf("machine id: %w", err)
	}
	claims, err := ParseToken(m.Secret, token)
	if err != nil {
		return fmt.Errorf("parse token: %w", err)
	}
	if claims.Machine != mid {
		return fmt.Errorf("license machine mismatch")
	}
	if claims.ExpiresAt != nil && claims.ExpiresAt.Before(time.Now()) {
		return fmt.Errorf("license expired")
	}
	return nil
}
