//
// Copyright (C) 2024 IOTech Ltd
//

package certificates

import (
	"crypto"
	"crypto/rand"
	"crypto/sha512"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/bootstrap/handlers/tls/seed"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/log"
)

type rootCertGenerator struct {
	logger          log.Logger
	certificateSeed seed.CertificateSeed
	writer          FileWriter
}

func (gen rootCertGenerator) Generate() (err error) {
	if gen.certificateSeed.NewCA {
		gen.logger.Debug("<Phase 1> Generating CA PKI materials")
		gen.logger.Debug("Generating Root CA key pair (sk,pk)")

		private, err := generatePrivateKey(gen.certificateSeed, gen.logger)
		if err != nil {
			return err
		}

		// Extract PK from RSA or EC generated SK
		public := private.(crypto.Signer).Public()
		if gen.certificateSeed.DumpKeys {
			dumpKeyPair(private, gen.logger)
			dumpKeyPair(public, gen.logger)
		}

		serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
		serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
		if err != nil {
			return fmt.Errorf("failed to generate serial number: %s", err.Error())
		}

		spkiASN1, err := x509.MarshalPKIXPublicKey(public)
		if err != nil {
			return fmt.Errorf("failed to encode public key: %s", err.Error())
		}

		var spki struct {
			Algorithm        pkix.AlgorithmIdentifier
			SubjectPublicKey asn1.BitString
		}
		_, err = asn1.Unmarshal(spkiASN1, &spki)
		if err != nil {
			return fmt.Errorf("failed to decode public key: %s", err.Error())
		}

		skid := sha512.Sum512(spki.SubjectPublicKey.Bytes)
		caCertTemplate := &x509.Certificate{
			SerialNumber: serialNumber,
			Subject: pkix.Name{
				CommonName:         gen.certificateSeed.CAName,
				Organization:       []string{gen.certificateSeed.CAName},
				OrganizationalUnit: []string{gen.certificateSeed.CAOrg},
				Locality:           []string{gen.certificateSeed.CALocality},
				Province:           []string{gen.certificateSeed.CAState},
				Country:            []string{gen.certificateSeed.CACountry},
			},
			EmailAddresses:        []string{gen.certificateSeed.CAName + "@" + gen.certificateSeed.TLSDomain},
			SubjectKeyId:          skid[:],
			NotAfter:              time.Now().AddDate(10, 0, 0),
			NotBefore:             time.Now(),
			KeyUsage:              x509.KeyUsageCertSign,
			BasicConstraintsValid: true,
			IsCA:                  true,
			MaxPathLenZero:        true,
		}
		gen.logger.Debug("Generating Root CA certificate")
		caDER, err := x509.CreateCertificate(rand.Reader, caCertTemplate, caCertTemplate, public, private)
		if err != nil {
			return fmt.Errorf("failed to generate CA certificate (DER): %s", err.Error())
		}

		_, err = x509.ParseCertificate(caDER)
		if err != nil {
			return fmt.Errorf("failed to parse Root CA certificate: %s", err.Error())
		}

		gen.logger.Debug(fmt.Sprintf("Saving Root CA private key to PEM file: %s", gen.certificateSeed.CAKeyFile))
		skPKCS8, err := x509.MarshalPKCS8PrivateKey(private)
		if err != nil {
			return fmt.Errorf("failed to encode CA private key: %s", err.Error())
		}

		err = gen.writer.Write(gen.certificateSeed.CAKeyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: skPKCS8}), 0400)
		if err != nil {
			return fmt.Errorf("failed to save CA private key: %s", err.Error())
		}

		gen.logger.Debug(fmt.Sprintf("Saving Root CA certificate to PEM file: %s", gen.certificateSeed.CACertFile))
		err = gen.writer.Write(gen.certificateSeed.CACertFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), 0644)
		if err != nil {
			return fmt.Errorf("failed to save CA certificate: %s", err.Error())
		}

		gen.logger.Debug("New local Root CA successfully created!")
	}
	return
}
