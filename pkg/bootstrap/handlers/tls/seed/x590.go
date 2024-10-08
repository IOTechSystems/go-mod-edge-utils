//
// Copyright (C) 2024 IOTech Ltd
//

package seed

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	envPrefix   = "$"
	hostDefault = "localhost"
)

// X509 JSON config file main structure
type X509 struct {
	CreateNewRootCA string    `json:"create_new_rootca"`
	WorkingDir      string    `json:"working_dir"`
	PKISetupDir     string    `json:"pki_setup_dir"`
	DumpConfig      string    `json:"dump_config"`
	KeyScheme       KeyScheme `json:"key_scheme"`
	RootCA          RootCA    `json:"x509_root_ca_parameters"`
	TLSServer       TLSServer `json:"x509_tls_server_parameters"`
}

func NewX509(configFilePtr string) (X509, error) {

	var jsonX509Config X509

	// Open JSON config file
	bytes, err := os.ReadFile(configFilePtr)
	if err != nil {
		return jsonX509Config, err
	}

	// Initialize the final X509 Configuration array
	// Unmarshal byteArray with the jsonFile's content into jsonX509Config
	err = json.Unmarshal(bytes, &jsonX509Config)
	if err != nil {
		return jsonX509Config, err
	}

	tlsHost := jsonX509Config.TLSServer.TLSHost
	if strings.HasPrefix(envPrefix, tlsHost) {
		host := os.Getenv(strings.TrimPrefix(tlsHost, envPrefix))
		if host == "" {
			host = hostDefault
		}
		jsonX509Config.TLSServer.TLSHost = host
	}

	return jsonX509Config, nil
}

// PkiCADir returns the pkisetup root CA dir
func (cfg X509) PkiCADir() (string, error) {
	dir, err := filepath.Abs(cfg.WorkingDir)
	if err != nil {
		// Looking at the implementation of filepath.Abs it does NOT verify the existence of the path
		return "", fmt.Errorf("unable to determine absolute path -- %s", err.Error())
	}
	// pkiCaDir: Concatenate working dir absolute path with PKI setup dir, using separator "/"
	return strings.Join([]string{dir, cfg.PKISetupDir, cfg.RootCA.CAName}, "/"), nil
}

// GetCAPemFileName returns the file name of CA certificate
func (cfg X509) GetCAPemFileName() string {
	return cfg.RootCA.CAName + "." + certFileExt
}

// GetCAPrivateKeyFileName returns the file name of CA private key
func (cfg X509) GetCAPrivateKeyFileName() string {
	return cfg.RootCA.CAName + "." + skFileExt
}

// GetTLSPemFileName returns the file name of TLS certificate
func (cfg X509) GetTLSPemFileName() string {
	return cfg.TLSServer.TLSHost + "." + certFileExt
}

// GetTLSPrivateKeyFileName returns the file name of TLS private key
func (cfg X509) GetTLSPrivateKeyFileName() string {
	return cfg.TLSServer.TLSHost + "." + skFileExt
}

// KeyScheme parameters (RSA vs EC)
// RSA: 1024, 2048, 4096
// EC: 224, 256, 384, 521
type KeyScheme struct {
	DumpKeys   string `json:"dump_keys"`
	RSA        string `json:"rsa"`
	RSAKeySize string `json:"rsa_key_size"`
	EC         string `json:"ec"`
	ECCurve    string `json:"ec_curve"`
}

// RootCA parameters from JSON: x509_root_ca_parameters
type RootCA struct {
	CAName     string `json:"ca_name"`
	CACountry  string `json:"ca_c"`
	CAState    string `json:"ca_st"`
	CALocality string `json:"ca_l"`
	CAOrg      string `json:"ca_o"`
}

// TLSServer parameters from JSON config: x509_tls_server_parameters
type TLSServer struct {
	TLSHost     string `json:"tls_host"`
	TLSDomain   string `json:"tls_domain"`
	TLSCountry  string `json:"tls_c"`
	TLSSate     string `json:"tls_st"`
	TLSLocality string `json:"tls_l"`
	TLSOrg      string `json:"tls_o"`
}
