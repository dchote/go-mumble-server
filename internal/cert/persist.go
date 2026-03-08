package cert

import (
	"github.com/dchote/go-mumble-server/internal/database/models"
	"github.com/dchote/go-mumble-server/internal/transport"
	"gorm.io/gorm"
)

// GetOrCreateCertForVirtualServer returns the TLS certificate and key for the
// given virtual server. If the server has no persisted cert, a new self-signed
// certificate is generated, saved to the database, and returned. The virtual
// server must exist; the caller must ensure it before calling.
func GetOrCreateCertForVirtualServer(db *gorm.DB, vserverID uint) (certPEM, keyPEM []byte, err error) {
	var vs models.VirtualServer
	if err := db.First(&vs, vserverID).Error; err != nil {
		return nil, nil, err
	}
	if vs.CertPEM != "" && vs.KeyPEM != "" {
		return []byte(vs.CertPEM), []byte(vs.KeyPEM), nil
	}
	certPEM, keyPEM, err = transport.GenerateSelfSigned()
	if err != nil {
		return nil, nil, err
	}
	if err := db.Model(&vs).Updates(map[string]interface{}{
		"cert_pem": string(certPEM),
		"key_pem":  string(keyPEM),
	}).Error; err != nil {
		return nil, nil, err
	}
	return certPEM, keyPEM, nil
}
