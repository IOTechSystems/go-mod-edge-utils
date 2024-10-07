//
// Copyright (C) 2024 IOTech Ltd
//

package certificates

import (
	"fmt"

	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/handlers/tls/seed"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/log"
)

type CertificateType int

const (
	RootCertificate CertificateType = 1
	TLSCertificate  CertificateType = 2
)

type CertificateGenerator interface {
	Generate() error
}

func NewCertificateGenerator(t CertificateType, certificateSeed seed.CertificateSeed, w FileWriter, logger log.Logger) (CertificateGenerator, error) {
	switch t {
	case RootCertificate:
		return rootCertGenerator{certificateSeed: certificateSeed, writer: w, logger: logger}, nil
	case TLSCertificate:
		return tlsCertGenerator{certificateSeed: certificateSeed, writer: w, logger: logger}, nil
	default:
		return nil, fmt.Errorf("unknown CertificateType %v", t)
	}
}
