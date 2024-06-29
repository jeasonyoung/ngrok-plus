package service

import (
	"ngrok-common/log"
	"time"
)

// ControlRegistryService 控制注册服务接口
type ControlRegistryService interface {
	log.Logger
	// Get 获取
	Get(clientId string) *Control
	// Add 添加
	Add(clientId string, ctl *Control) (oldCtl *Control)
	// Del 删除
	Del(clientId string) error
}

var localControlRegistryService ControlRegistryService

// ControlRegistry 控制注册服务
func ControlRegistry() ControlRegistryService {
	if localControlRegistryService == nil {
		panic("未注册服务=>ControlRegistryService")
	}
	return localControlRegistryService
}

// RegisterControlRegistry 注册控制注册服务
func RegisterControlRegistry(svc ControlRegistryService) {
	svc.Info("注册服务:ControlRegistryService=> %p", svc)
	localControlRegistryService = svc
}

// TunnelRegistryService 通道注册服务接口
type TunnelRegistryService interface {
	log.Logger
	// SaveCacheThread 保存缓存到文件
	SaveCacheThread(path string, interval time.Duration)
	// Register 注册通道
	Register(url string, t *Tunnel) error
	// GetCachedRegistration 获取注册缓存
	GetCachedRegistration(t *Tunnel) (url string)
	// RegisterAndCache 注册并缓存
	RegisterAndCache(url string, t *Tunnel) (err error)
	// RegisterRepeat 注册
	RegisterRepeat(urlFn func() string, t *Tunnel) (string, error)
	// Del 删除
	Del(url string)
	// Get 获取
	Get(url string) *Tunnel
}

var localTunnelRegistryService TunnelRegistryService

// TunnelRegistry 通道注册服务实例
func TunnelRegistry() TunnelRegistryService {
	if localTunnelRegistryService == nil {
		panic("未注册服务=>TunnelRegistryService")
	}
	return localTunnelRegistryService
}

// RegisterTunnelRegistry 注册通道注册服务
func RegisterTunnelRegistry(svc TunnelRegistryService) {
	svc.Info("注册服务:TunnelRegistryService=> %p", svc)
	localTunnelRegistryService = svc
}
