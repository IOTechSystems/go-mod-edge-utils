//
// Copyright (C) 2024 IOTech Ltd
//

package certificates

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"fmt"

	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/handlers/tls/seed"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/log"
)

// generatePrivateKey creates a new RSA or EC based private key (sk)
// ----------------------------------------------------------
func generatePrivateKey(certificateSeed seed.CertificateSeed, logger log.Logger) (crypto.PrivateKey, error) {
	if certificateSeed.RSAScheme {
		logger.Debug(fmt.Sprintf("Generating private key with RSA scheme %v", certificateSeed.RSAKeySize))
		return rsa.GenerateKey(rand.Reader, int(certificateSeed.RSAKeySize))
	}

	if certificateSeed.ECScheme {
		logger.Debug(fmt.Sprintf("Generating private key with EC scheme %v", certificateSeed.ECCurve))
		switch certificateSeed.ECCurve {
		case seed.EC_224: // secp224r1 NIST P-224
			return ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
		case seed.EC_256: // secp256v1 NIST P-256
			return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		case seed.EC_384: // secp384r1 NIST P-384
			return ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
		case seed.EC_521: // secp521r1 NIST P-521
			return ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
		}
	}

	return nil, fmt.Errorf("Unknown key scheme: RSA[%t] EC[%t]", certificateSeed.RSAScheme, certificateSeed.ECScheme)
}

// dumpKeyPair output sk,pk keypair (RSA or EC) to console
// !!! Debug only for obvious security reasons...
// ----------------------------------------------------------
func dumpKeyPair(key any, logger log.Logger) {
	switch key.(type) {
	case *rsa.PrivateKey:
		logger.Debug(fmt.Sprintf(">> RSA SK: %q", key))
	case *ecdsa.PrivateKey:
		logger.Debug(fmt.Sprintf(">> ECDSA SK: %q", key))
	case *rsa.PublicKey:
		logger.Debug(fmt.Sprintf(">> RSA PK: %q", key))
	case *ecdsa.PublicKey:
		logger.Debug(fmt.Sprintf(">> ECDSA PK: %q", key))
	default:
		logger.Error("Unsupported Key Type")
	}
}
