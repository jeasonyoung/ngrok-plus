package service

import (
	"crypto/tls"
	"github.com/gogf/gf/v2/os/gres"
	"os"
)

// LoadTLSConfig 加载TLS配置
func LoadTLSConfig(crt, keyPath string) (tlsConfig *tls.Config, err error) {
	fileOrAssert := func(path, defPath string) ([]byte, error) {
		loadFn := os.ReadFile
		if path == "" {
			loadFn = func(path string) ([]byte, error) {
				data := gres.GetContent(path)
				return data, nil
			}
			path = defPath
		}
		return loadFn(path)
	}

	var (
		crtBytes []byte
		keyBytes []byte
		cert     tls.Certificate
	)
	if crtBytes, err = fileOrAssert(crt, "assets/server/tls/snakeoil.crt"); err != nil {
		return
	}
	if keyBytes, err = fileOrAssert(keyPath, "assets/server/tls/snakeoil.key"); err != nil {
		return
	}
	if cert, err = tls.X509KeyPair(crtBytes, keyBytes); err != nil {
		return
	}
	tlsConfig = &tls.Config{
		Certificates: []tls.Certificate{cert},
	}
	return
}
