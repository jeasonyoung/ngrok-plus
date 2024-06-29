package metrics

import (
	"ngrok-server/internal/consts"
	"ngrok-server/internal/service"
	"os"
	"time"
)

// 初始化
func init() {
	keepApiKey := os.Getenv(consts.KeenApiKey)
	if keepApiKey != "" {
		service.RegisterMetrics(newKeenIoMetrics(60 * time.Second))
	} else {
		service.RegisterMetrics(newLocalMetrics(30 * time.Second))
	}
}
