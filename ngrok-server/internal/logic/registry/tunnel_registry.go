package registry

import (
	"encoding/gob"
	"fmt"
	"ngrok-common/cache"
	"ngrok-common/log"
	"ngrok-server/internal/consts"
	"ngrok-server/internal/service"
	"os"
	"sync"
	"time"
)

const (
	registryCacheSize uint64        = 1024 * 1024 // 1 MB
	cacheSaveInterval time.Duration = 10 * time.Minute
)

type cacheUrl string

func (url cacheUrl) Size() int {
	return len(url)
}

type localTunnelRegistry struct {
	tunnels  map[string]*service.Tunnel
	affinity cache.Cache
	log.Logger
	sync.RWMutex
}

func NewTunnelRegistry(cacheSize uint64, cacheFile string) service.TunnelRegistryService {
	registry := &localTunnelRegistry{
		tunnels:  make(map[string]*service.Tunnel),
		affinity: cache.NewLRUCache(cacheSize),
		Logger:   log.NewPrefixLogger("registry", "tunnel"),
	}
	// LRUCache uses Gob encoding. Unfortunately, Gob is fickle and will fail
	// to encode or decode any non-primitive types that haven't been "registered"
	// with it. Since we store cacheUrl objects, we need to register them here first
	// for the encoding/decoding to work
	var urlObj cacheUrl
	gob.Register(urlObj)

	// try to load and then periodically save the affinity cache to file, if specified
	if cacheFile != "" {
		if err := registry.affinity.LoadItemsFromFile(cacheFile); err != nil {
			_ = registry.Error("Failed to load affinity cache %s:%v", cacheFile, err)
		}

		registry.SaveCacheThread(cacheFile, cacheSaveInterval)
	} else {
		registry.Info("No affinity cache specified")
	}

	return registry
}

// 初始化
func init() {
	registryCacheFile := os.Getenv(consts.RegistryCacheFile)
	service.RegisterTunnelRegistry(NewTunnelRegistry(registryCacheSize, registryCacheFile))
}

// SaveCacheThread spawns a goroutine the periodically saves the cache to a file
func (r *localTunnelRegistry) SaveCacheThread(path string, interval time.Duration) {
	go func() {
		r.Info("Saving affinity cache to %s every %s", path, interval.String())
		for {
			time.Sleep(interval)
			r.Debug("Saving affinity cache")
			if err := r.affinity.SaveItemsToFile(path); err != nil {
				_ = r.Error("Failed to save affinity cache: %v", err)
			} else {
				r.Info("Saved affinity cache")
			}
		}
	}()
}

// Register a tunnel with a specific url,returns an error if a tunnel is already registered at that url
func (r *localTunnelRegistry) Register(url string, t *service.Tunnel) error {
	r.Lock()
	defer r.Unlock()

	if r.tunnels[url] != nil {
		return fmt.Errorf("the tunnel %s is already registered", url)
	}
	r.tunnels[url] = t

	return nil
}

func (r *localTunnelRegistry) cacheKeys(t *service.Tunnel) (ipKey, idKey string) {
	clientIp := t.GetCtlIPAddr()
	clientId := t.GetCtlId()

	procotol := t.GetReqProcotol()
	ipKey = fmt.Sprintf("client-ip-%s:%s", procotol, clientIp)
	idKey = fmt.Sprintf("client-id-%s:%s", procotol, clientId)
	return
}

func (r *localTunnelRegistry) GetCachedRegistration(t *service.Tunnel) (url string) {
	ipCacheKey, idCacheKey := r.cacheKeys(t)
	//check cache for ID first,because we prefer that over IP which might not be specific to
	// a user because of NATs
	if v, ok := r.affinity.Get(idCacheKey); ok {
		url = string(v.(cacheUrl))
		t.Debug("Found registry affinity %s for %s", url, idCacheKey)
		return
	}
	if v, ok := r.affinity.Get(ipCacheKey); ok {
		url = string(v.(cacheUrl))
		t.Debug("Found registry affinity %s for %s", url, ipCacheKey)
	}
	return
}

func (r *localTunnelRegistry) RegisterAndCache(url string, t *service.Tunnel) (err error) {
	if err = r.Register(url, t); err == nil {
		// we successfully assigned a url,cache it
		ipCacheKey, idCacheKey := r.cacheKeys(t)
		r.affinity.Set(ipCacheKey, cacheUrl(url))
		r.affinity.Set(idCacheKey, cacheUrl(url))
	}
	return
}

// RegisterRepeat a tunnel with the following process:
// Consult the affinity cache to try to assign a previously used tunnel url if possible
// Generate new urls repeatedly with the urlFn and register until one is available.
func (r *localTunnelRegistry) RegisterRepeat(urlFn func() string, t *service.Tunnel) (string, error) {
	url := r.GetCachedRegistration(t)
	if url == "" {
		url = urlFn()
	}
	maxAttempts := 5
	for i := 0; i < maxAttempts; i++ {
		if err := r.RegisterAndCache(url, t); err != nil {
			//pick a new url and try again
			url = urlFn()
		} else {
			//we successfully assigned a url, we're done
			return url, nil
		}
	}

	return "", fmt.Errorf("failed to assign a URL after %d attempts", maxAttempts)
}

func (r *localTunnelRegistry) Del(url string) {
	r.Lock()
	defer r.Unlock()
	delete(r.tunnels, url)
}

func (r *localTunnelRegistry) Get(url string) *service.Tunnel {
	r.RLock()
	defer r.RUnlock()
	return r.tunnels[url]
}
