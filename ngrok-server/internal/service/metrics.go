package service

import (
	"ngrok-common/conn"
	"ngrok-common/log"
	"time"
)

// MetricsService 度量指标_服务接口
type MetricsService interface {
	log.Logger
	OpenConnection(t *Tunnel, c conn.Conn)
	CloseConnection(t *Tunnel, c conn.Conn, start time.Time, bytesIn, bytesOut int64)
	OpenTunnel(t *Tunnel)
	CloseTunnel(t *Tunnel)
}

var localMetricsService MetricsService

// Metrics 度量指标服务
func Metrics() MetricsService {
	if localMetricsService == nil {
		panic("未注册服务=>MetricsService")
	}
	return localMetricsService
}

// RegisterMetrics 注册度量指标服务
func RegisterMetrics(svc MetricsService) {
	svc.Info("注册服务:MetricsService=> %p", svc)
	localMetricsService = svc
}
