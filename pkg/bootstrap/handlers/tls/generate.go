//
// Copyright (C) 2024 IOTech Ltd
//

package secrets

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/bootstrap/handlers/tls/certificates"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/bootstrap/handlers/tls/seed"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/errors"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/log"
)

const (
	tlsCertPathEnvName = "TLS_CERT_PATH"
	tlsKeyPathEnvName  = "TLS_KEY_PATH"

	tlsCertFileName   = "cert.pem"
	caCertFileName    = "ca.pem"
	tlsSecretFileName = "key.pem"
	CaServiceName     = "ca"
)

func GetTlsCertAndKeyPath(lc log.Logger, certConfig, certOutputDir string) (tlsCertPath string, tlsKeyPath string, err error) {
	tlsCertPath = os.Getenv(tlsCertPathEnvName)
	tlsKeyPath = os.Getenv(tlsKeyPathEnvName)
	if tlsCertPath != "" && tlsKeyPath != "" {
		return tlsCertPath, tlsKeyPath, nil
	}

	tlsCertPath, tlsKeyPath, err = generateCertAndKey(lc, certConfig, certOutputDir)
	if err != nil {
		return "", "", err
	}
	return tlsCertPath, tlsKeyPath, nil
}

// generateCertAndKey generates cert and key if not exist
func generateCertAndKey(lc log.Logger, certConfig, certOutputDir string) (tlsCertPath string, tlsKeyPath string, err errors.Error) {
	lc.Warn("The TLS is enabled but the cert and key are undefined, try to generate self-signed TLS cert and key...")
	caFilePath := tlsCaFilePath(certOutputDir)
	certFilePth := tlsCertFilePath(certOutputDir)
	keyFilePth := tlsKeyFilePath(certOutputDir)

	if checkIfFileExists(caFilePath) &&
		checkIfFileExists(certFilePth) &&
		checkIfFileExists(keyFilePth) {
		lc.Info("The self-signed TLS cert and key exist, skip generating.")
		return certFilePth, keyFilePth, nil
	}

	if !checkIfFileExists(certConfig) {
		return "", "", errors.NewBaseError(errors.KindEntityDoesNotExist, fmt.Sprintf("config file %s does not exist", certConfig), nil)
	}

	if err := genTLSAssets(lc, certConfig, certOutputDir); err != nil {
		return "", "", errors.BaseErrorWrapper(err)
	}

	lc.Info("The self-signed TLS cert and key generated.")
	return certFilePth, keyFilePth, nil
}

// genTLSAssets generates the TLS assets based on the JSON configuration file
func genTLSAssets(lc log.Logger, jsonConfig, certOutputDir string) errors.Error {
	// Read the Json x509 file and unmarshall content into struct type X509
	x509Config, err := seed.NewX509(jsonConfig)
	if err != nil {
		return errors.NewBaseError(errors.KindServerError, "fail to read x590 json config", err)
	}

	seed, err := seed.NewCertificateSeed(x509Config, lc)
	if err != nil {
		return errors.NewBaseError(errors.KindServerError, "fail to create certificate seed", err)
	}

	rootCA, err := certificates.NewCertificateGenerator(certificates.RootCertificate, seed, certificates.NewFileWriter(), lc)
	if err != nil {
		return errors.NewBaseError(errors.KindServerError, "fail to create root certificate generator", err)
	}

	err = rootCA.Generate()
	if err != nil {
		return errors.NewBaseError(errors.KindServerError, "fail to create root certificate", err)
	}

	tlsCert, err := certificates.NewCertificateGenerator(certificates.TLSCertificate, seed, certificates.NewFileWriter(), lc)
	if err != nil {
		return errors.NewBaseError(errors.KindServerError, "fail to create tls certificate generator", err)
	}

	err = tlsCert.Generate()
	if err != nil {
		return errors.NewBaseError(errors.KindServerError, "fail to create tls certificate", err)
	}

	err = copyToTlsFolder(x509Config, certOutputDir)
	if err != nil {
		return errors.NewBaseError(errors.KindServerError, "fail to copy generated certificate to tls folder", err)
	}
	return nil
}

func copyToTlsFolder(x509Config seed.X509, certOutputDir string) error {
	pkiDir, err := x509Config.PkiCADir()
	if err != nil {
		return err
	}

	err = createDirectoryIfNotExists(certOutputDir)
	if err != nil {
		return err
	}

	// Copy the ca.pem file
	pkiCaFilePath := filepath.Join(pkiDir, x509Config.GetCAPemFileName())
	destCaFilePath := tlsCaFilePath(certOutputDir)
	if _, err := copyFile(pkiCaFilePath, destCaFilePath); err != nil {
		return err
	}

	// Copy the cert.pem and key.pem file
	pkiKeyFilePth := filepath.Join(pkiDir, x509Config.GetTLSPrivateKeyFileName())
	destKeyFileName := tlsKeyFilePath(certOutputDir)
	pkiCertFileName := filepath.Join(pkiDir, x509Config.GetTLSPemFileName())
	destCertFileName := tlsCertFilePath(certOutputDir)
	if filepath.Base(x509Config.WorkingDir) == CaServiceName {
		caKeyFilePth := filepath.Join(pkiDir, x509Config.GetCAPrivateKeyFileName())
		if _, err := copyFile(caKeyFilePth, destKeyFileName); err != nil {
			return err
		}
	} else {
		if _, err := copyFile(pkiKeyFilePth, destKeyFileName); err != nil {
			return err
		}
		// if not CA, then also copy the TLS cert as well
		if _, err := copyFile(pkiCertFileName, destCertFileName); err != nil {
			return err
		}
	}

	// read-only to the owner
	return os.Chmod(destKeyFileName, 0400)
}

func tlsCertFilePath(configDir string) string {
	return filepath.Join(configDir, tlsCertFileName)
}

func tlsKeyFilePath(configDir string) string {
	return filepath.Join(configDir, tlsSecretFileName)
}

func tlsCaFilePath(configDir string) string {
	return filepath.Join(configDir, caCertFileName)
}

func createDirectoryIfNotExists(dirName string) (err error) {
	if _, err := os.Stat(dirName); os.IsNotExist(err) {
		err = os.MkdirAll(dirName, os.ModePerm)
		if err != nil {
			return err
		}
	}
	return err
}

func copyFile(fileSrc, fileDest string) (int64, error) {
	var zeroByte int64
	sourceFileSt, err := os.Stat(fileSrc)
	if err != nil {
		return zeroByte, err
	}

	// only regular file allows to be copied
	if !sourceFileSt.Mode().IsRegular() {
		return zeroByte, fmt.Errorf("[%s] is not a regular file to be copied", fileSrc)
	}

	// now open the source file
	source, openErr := os.Open(fileSrc)
	if openErr != nil {
		return zeroByte, openErr
	}
	defer source.Close()

	if _, err := os.Stat(fileDest); err == nil {
		// if fileDest alrady exists, remove it first before create a new one
		os.Remove(fileDest)
	}

	// now create a new file
	dest, createErr := os.Create(fileDest)
	if createErr != nil {
		return zeroByte, createErr
	}
	defer dest.Close()

	bytesWritten, copyErr := io.Copy(dest, source)
	if copyErr != nil {
		return zeroByte, copyErr
	}
	// make dest has the same file mode as the source
	_ = os.Chmod(fileDest, sourceFileSt.Mode())
	return bytesWritten, nil
}

func checkIfFileExists(fileName string) bool {
	fileInfo, statErr := os.Stat(fileName)
	if os.IsNotExist(statErr) {
		return false
	}
	return !fileInfo.IsDir()
}
