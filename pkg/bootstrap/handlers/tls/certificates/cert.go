//
// Copyright (C) 2024 IOTech Ltd
//

package certificates

import (
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/bootstrap/handlers/tls/seed"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/log"
)

type tlsCertGenerator struct {
	logger          log.Logger
	certificateSeed seed.CertificateSeed
	writer          FileWriter
}

func (gen tlsCertGenerator) Generate() (err error) {

	gen.logger.Debug("<Phase 2> Generating TLS server PKI materials")

	// Root CA certificate fetch --------------------------------------------------------
	gen.logger.Debug(fmt.Sprintf("Loading Root CA certificate: %s", gen.certificateSeed.CACertFile))
	certPEMBlock, err := os.ReadFile(gen.certificateSeed.CACertFile) // Load Root CA certificate
	if err != nil {
		return fmt.Errorf("failed to read the Root CA certificate: %s", err.Error())
	}

	gen.logger.Debug("Decoding the Root CA certificate")
	certDERBlock, _ := pem.Decode(certPEMBlock) // Decode Root CA certificate
	if certDERBlock == nil || certDERBlock.Type != "CERTIFICATE" {
		return fmt.Errorf("failed to read the Root CA certificate: unexpected content")
	}

	gen.logger.Debug("- Parsing the Root CA certificate")
	caCert, err := x509.ParseCertificate(certDERBlock.Bytes) // Parse Root CA certificate
	if err != nil {
		return fmt.Errorf("failed to parse the Root CA certificate: %s", err.Error())
	}

	// Root CA private key fetch --------------------------------------------------------
	gen.logger.Debug(fmt.Sprintf("Loading the Root CA private key: %s", gen.certificateSeed.CAKeyFile))
	keyPEMBlock, err := os.ReadFile(gen.certificateSeed.CAKeyFile)
	if err != nil {
		return fmt.Errorf("failed to read the Root CA private key: %s", err.Error())
	}

	gen.logger.Debug("- Decoding the Root CA private key")
	keyDERBlock, _ := pem.Decode(keyPEMBlock) // Decode Root CA private key
	if keyDERBlock == nil || keyDERBlock.Type != "PRIVATE KEY" {
		return fmt.Errorf("failed to read the Root CA key: unexpected content")
	}

	gen.logger.Debug("- Parsing the Root CA private key")
	privateCA, err := x509.ParsePKCS8PrivateKey(keyDERBlock.Bytes) // Parse Root CA private key
	if err != nil {
		return fmt.Errorf("failed to parse the Root CA key: %s", err.Error())
	}

	// TLS server certificate preparation -----------------------------------------------
	gen.logger.Debug("Generating TLS server key pair (sk,pk)")

	// Generate RSA or EC based SK
	privateTLS, err := generatePrivateKey(gen.certificateSeed, gen.logger)
	if err != nil {
		return err
	}
	// Extract PK from RSA or EC generated SK
	publicTLS := privateTLS.(crypto.Signer).Public()
	// Debug the key pair generation/extraction
	if gen.certificateSeed.DumpKeys {
		dumpKeyPair(privateTLS, gen.logger)
		dumpKeyPair(publicTLS, gen.logger)
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return fmt.Errorf("failed to generate serial number: %s", err.Error())
	}

	tlsCertTemplate := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:         gen.certificateSeed.TLSFqdn,
			Organization:       []string{gen.certificateSeed.TLSHost},
			OrganizationalUnit: []string{gen.certificateSeed.TLSOrg},
			Locality:           []string{gen.certificateSeed.TLSLocality},
			Province:           []string{gen.certificateSeed.TLSState},
			Country:            []string{gen.certificateSeed.TLSCountry},
		},
		EmailAddresses:        []string{"admin@" + gen.certificateSeed.TLSDomain},
		DNSNames:              []string{gen.certificateSeed.TLSFqdn, gen.certificateSeed.TLSAltFqdn}, // Alternative Names
		NotAfter:              time.Now().AddDate(10, 0, 0),
		NotBefore:             time.Now(),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	gen.logger.Debug("Generating TLS server certificate (Self-signed with our local Root CA)")
	tlsDER, err := x509.CreateCertificate(rand.Reader, tlsCertTemplate, caCert, publicTLS, privateCA)
	if err != nil {
		return fmt.Errorf("failed to generate TLS server certificate - DER: %s", err.Error())
	}

	_, err = x509.ParseCertificate(tlsDER)
	if err != nil {
		return fmt.Errorf("failed to parse TLS server certificate - DER: %s", err.Error())
	}

	gen.logger.Debug(fmt.Sprintf("Saving TLS server private key to PEM file: %s", gen.certificateSeed.TLSKeyFile))
	skPKCS8, err := x509.MarshalPKCS8PrivateKey(privateTLS)
	if err != nil {
		return fmt.Errorf("failed to encode TLS server key: %s", err.Error())
	}

	err = gen.writer.Write(gen.certificateSeed.TLSKeyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: skPKCS8}), 0600)
	if err != nil {
		return fmt.Errorf("failed to save TLS server private key: %s", err.Error())
	}

	gen.logger.Debug(fmt.Sprintf("Saving TLS server certificate to PEM file: %s", gen.certificateSeed.TLSCertFile))
	err = gen.writer.Write(gen.certificateSeed.TLSCertFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: tlsDER}), 0644)
	if err != nil {
		return fmt.Errorf("failed to save TLS server certificate: %s", err.Error())
	}

	gen.logger.Debug("New TLS server certificate/key successfully created!")

	return
}
