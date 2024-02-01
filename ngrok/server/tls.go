package server

import (
	"crypto/tls"
	"ngrok-plus/ngrok/server/assets"
	"os"
)

func LoadTLSConfig(crtPath string, keyPath string) (tlsConfig *tls.Config, err error) {
	fileOrAsset := func(path string, defaultPath string) ([]byte, error) {
		loadFn := os.ReadFile
		if path == "" {
			loadFn = assets.Asset
			path = defaultPath
		}

		return loadFn(path)
	}

	var (
		crt  []byte
		key  []byte
		cert tls.Certificate
	)

	if crt, err = fileOrAsset(crtPath, "assets/server/tls/snakeoil.crt"); err != nil {
		return
	}

	if key, err = fileOrAsset(keyPath, "assets/server/tls/snakeoil.key"); err != nil {
		return
	}

	if cert, err = tls.X509KeyPair(crt, key); err != nil {
		return
	}

	tlsConfig = &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	return
}
