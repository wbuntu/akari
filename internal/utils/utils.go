package utils

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"reflect"
	"runtime"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/sys/cpu"
)

// NewTLSConfig Import trusted certificates to build tls.Config
func NewTLSConfig(caCertFile, tlsCertFile, tlsKeyFile string) (*tls.Config, error) {
	if caCertFile == "" && tlsCertFile == "" && tlsKeyFile == "" {
		return &tls.Config{}, nil
	}

	tlsConfig := &tls.Config{}

	// Import trusted certificates from CAfile.pem.
	if caCertFile != "" {
		cacert, err := ioutil.ReadFile(caCertFile)
		if err != nil {
			return nil, errors.Wrap(err, "ioutil.ReadFile")
		}
		certpool := x509.NewCertPool()
		certpool.AppendCertsFromPEM(cacert)

		tlsConfig.RootCAs = certpool // RootCAs = certs used to verify server cert.
	}

	// Import certificate and the key
	if tlsCertFile != "" && tlsKeyFile != "" {
		kp, err := tls.LoadX509KeyPair(tlsCertFile, tlsKeyFile)
		if err != nil {
			return nil, errors.Wrap(err, "tls.LoadX509KeyPair")
		}
		tlsConfig.Certificates = []tls.Certificate{kp}
	}

	return tlsConfig, nil
}

// GetFunctionName return name for func
func GetFunctionName(i interface{}) string {
	name := runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
	data := strings.Split(name, "/")
	return data[len(data)-1]
}

var (
	hasGCMAsmAMD64 = cpu.X86.HasAES && cpu.X86.HasPCLMULQDQ
	hasGCMAsmARM64 = cpu.ARM64.HasAES && cpu.ARM64.HasPMULL
	// Keep in sync with crypto/aes/cipher_s390x.go.
	hasGCMAsmS390X = cpu.S390X.HasAES && cpu.S390X.HasAESCBC && cpu.S390X.HasAESCTR && (cpu.S390X.HasGHASH || cpu.S390X.HasAESGCM)

	hasGCMAsm = hasGCMAsmAMD64 || hasGCMAsmARM64 || hasGCMAsmS390X

	aesCipher = []uint16{
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	}
	chachaCipher = []uint16{
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
	}
)

func CipherSuites() []uint16 {
	var cipherSuites []uint16
	if hasGCMAsm {
		cipherSuites = append(aesCipher, chachaCipher...)
	} else {
		cipherSuites = append(chachaCipher, aesCipher...)
	}
	return cipherSuites
}

var ciphers map[uint16]string

func init() {
	ciphers = map[uint16]string{}
	for _, v := range tls.CipherSuites() {
		ciphers[v.ID] = v.Name
	}
	for _, v := range tls.InsecureCipherSuites() {
		ciphers[v.ID] = v.Name
	}
}

var tlsVersions = map[uint16]string{
	tls.VersionTLS10: "TLSv1.0",
	tls.VersionTLS11: "TLSv1.1",
	tls.VersionTLS12: "TLSv1.2",
	tls.VersionTLS13: "TLSv1.3",
}

func TLSFormatString(srcConn *tls.Conn) string {
	state := srcConn.ConnectionState()
	tlsVer := tlsVersions[state.Version]
	cipher := ciphers[state.CipherSuite]
	return tlsVer + "-" + cipher
}
