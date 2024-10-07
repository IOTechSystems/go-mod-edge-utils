//
// Copyright (C) 2024 IOTech Ltd
//

package seed

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/IOTechSystems/go-mod-edge-utils/pkg/log"
)

type RSAKeySize int

const (
	skFileExt   = "priv.key"
	certFileExt = "pem"

	RSA_1024 RSAKeySize = 1024
	RSA_2048 RSAKeySize = 2048
	RSA_4096 RSAKeySize = 4096
)

func validateRSAKeySize(value int) bool {
	sizes := map[int]RSAKeySize{
		1024: RSA_1024,
		2048: RSA_2048,
		4096: RSA_4096,
	}
	_, ok := sizes[value]
	return ok
}

type EllipticCurve int

const (
	EC_224 EllipticCurve = 224
	EC_256 EllipticCurve = 256
	EC_384 EllipticCurve = 384
	EC_521 EllipticCurve = 521
)

func validateEllipticCurve(value int) bool {
	curves := map[int]EllipticCurve{
		224: EC_224,
		256: EC_256,
		384: EC_384,
		521: EC_521,
	}
	_, ok := curves[value]
	return ok
}

// CertificateSeed is responsible for parsing the X509 configuration into values that can be readily used to generate
// Root CA and TLS-related certificates. It will also validate the configuration provided to it upon instantiation.
type CertificateSeed struct {
	CACertFile  string
	CACountry   string
	CAKeyFile   string
	CALocality  string
	CAName      string
	CAOrg       string
	CAState     string
	DumpKeys    bool
	ECCurve     EllipticCurve
	ECScheme    bool
	NewCA       bool
	RSAKeySize  RSAKeySize
	RSAScheme   bool
	TLSAltFqdn  string
	TLSCertFile string
	TLSCountry  string
	TLSDomain   string
	TLSFqdn     string
	TLSHost     string
	TLSKeyFile  string
	TLSLocality string
	TLSOrg      string
	TLSState    string
}

func NewCertificateSeed(cfg X509, lc log.Logger) (seed CertificateSeed, err error) {
	directoryHandler := NewDirectoryHandler(lc)

	// Convert create_new_ca JSON string "true|false" to boolean
	seed.NewCA, err = strconv.ParseBool(cfg.CreateNewRootCA)
	if err != nil {
		return
	}

	// Convert dump_keys JSON string "true|flase| to boolean
	seed.DumpKeys, err = strconv.ParseBool(cfg.KeyScheme.DumpKeys)
	if err != nil {
		return
	}

	// Convert rsa JSON string "true|false" to boolean
	seed.RSAScheme, err = strconv.ParseBool(cfg.KeyScheme.RSA)
	if err != nil {
		return
	}

	// Convert rsa_key_size JSON string to integer
	check, err := strconv.Atoi(cfg.KeyScheme.RSAKeySize)
	if err != nil {
		return
	}

	if validateRSAKeySize(check) {
		seed.RSAKeySize = RSAKeySize(check)
	} else {
		return seed, fmt.Errorf("RSAKeySize value is invalid %v", check)
	}

	// Convert ec JSON string "true|false" to boolean
	seed.ECScheme, err = strconv.ParseBool(cfg.KeyScheme.EC)
	if err != nil {
		return
	}

	// Convert EC chosen curve JSON string to integer
	check, err = strconv.Atoi(cfg.KeyScheme.ECCurve)
	if err != nil {
		return
	}

	if validateEllipticCurve(check) {
		seed.ECCurve = EllipticCurve(check)
	} else {
		return seed, fmt.Errorf("ECCurve value is invalid %v", check)
	}

	// Init: CA name and PEM key/cert filenames
	pkiCaDir, err := cfg.PkiCADir()
	if err != nil {
		return
	}
	seed.CAName = cfg.RootCA.CAName
	seed.CAKeyFile = filepath.Join(pkiCaDir, seed.CAName+"."+skFileExt)
	seed.CACertFile = filepath.Join(pkiCaDir, seed.CAName+"."+certFileExt)

	// Init: TLS host.domain and PEM key/cert filenames
	seed.TLSHost = cfg.TLSServer.TLSHost
	seed.TLSDomain = cfg.TLSServer.TLSDomain
	if seed.TLSDomain == "local" {
		seed.TLSFqdn = seed.TLSHost
		seed.TLSAltFqdn = seed.TLSHost + "." + seed.TLSDomain
	} else {
		seed.TLSFqdn = seed.TLSHost + "." + seed.TLSDomain
	}
	seed.TLSKeyFile = filepath.Join(pkiCaDir, seed.TLSHost+"."+skFileExt)
	seed.TLSCertFile = filepath.Join(pkiCaDir, seed.TLSHost+"."+certFileExt)
	// CA subjects
	seed.CACountry = cfg.RootCA.CACountry
	seed.CAState = cfg.RootCA.CAState
	seed.CALocality = cfg.RootCA.CALocality
	seed.CAOrg = cfg.RootCA.CAOrg
	// TLS subjects
	seed.TLSCountry = cfg.TLSServer.TLSCountry
	seed.TLSState = cfg.TLSServer.TLSSate
	seed.TLSLocality = cfg.TLSServer.TLSLocality
	seed.TLSOrg = cfg.TLSServer.TLSOrg

	dumpConfig, err := strconv.ParseBool(cfg.DumpConfig)
	if err != nil {
		return
	}
	if dumpConfig {
		json, err := json.MarshalIndent(seed, "", "    ")
		if err != nil {
			return seed, err
		}
		directoryHandler.loggingClient.Debug(string(json))
	}

	if seed.NewCA {
		return seed, directoryHandler.Create(pkiCaDir)
	}

	return seed, directoryHandler.Verify(pkiCaDir)
}
