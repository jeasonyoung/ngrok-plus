package metrics

import (
	"encoding/json"
	"ngrok-common/conn"
	"ngrok-common/log"
	"ngrok-server/internal/service"
	"time"

	gometrics "github.com/rcrowley/go-metrics"
)

type localMetrics struct {
	log.Logger
	reportInterval time.Duration
	windowsCounter gometrics.Counter
	linuxCounter   gometrics.Counter
	osxCounter     gometrics.Counter
	otherCounter   gometrics.Counter

	tunnelMeter        gometrics.Meter
	tcpTunnelMeter     gometrics.Meter
	httpTunnelMeter    gometrics.Meter
	connMeter          gometrics.Meter
	lostHeartbeatMeter gometrics.Meter

	connTimer gometrics.Timer

	bytesInCount  gometrics.Counter
	bytesOutCount gometrics.Counter
}

func newLocalMetrics(reportInterval time.Duration) service.MetricsService {
	metrics := &localMetrics{
		Logger:             log.NewPrefixLogger("metrics"),
		reportInterval:     reportInterval,
		windowsCounter:     gometrics.NewCounter(),
		linuxCounter:       gometrics.NewCounter(),
		osxCounter:         gometrics.NewCounter(),
		otherCounter:       gometrics.NewCounter(),
		tunnelMeter:        gometrics.NewMeter(),
		tcpTunnelMeter:     gometrics.NewMeter(),
		httpTunnelMeter:    gometrics.NewMeter(),
		connMeter:          gometrics.NewMeter(),
		lostHeartbeatMeter: gometrics.NewMeter(),
		connTimer:          gometrics.NewTimer(),
		bytesInCount:       gometrics.NewCounter(),
		bytesOutCount:      gometrics.NewCounter(),
	}
	go metrics.Report()
	return metrics
}

func (m *localMetrics) OpenConnection(t *service.Tunnel, c conn.Conn) {
	m.connMeter.Mark(1)
}

func (m *localMetrics) CloseConnection(t *service.Tunnel, c conn.Conn, start time.Time, bytesIn, bytesOut int64) {
	m.bytesInCount.Inc(bytesIn)
	m.bytesOutCount.Inc(bytesOut)
}

func (m *localMetrics) OpenTunnel(t *service.Tunnel) {
	m.tunnelMeter.Mark(1)

	switch t.GetCtlAuthOS() {
	case "windows":
		m.windowsCounter.Inc(1)
	case "linux":
		m.linuxCounter.Inc(1)
	case "darwin":
		m.osxCounter.Inc(1)
	default:
		m.otherCounter.Inc(1)
	}

	switch t.GetReqProcotol() {
	case "tcp":
		m.tcpTunnelMeter.Mark(1)
	case "http":
		m.httpTunnelMeter.Mark(1)
	}
}

func (m *localMetrics) CloseTunnel(t *service.Tunnel) {

}

func (m *localMetrics) Report() {
	m.Info("Reporting every %d seconds", int(m.reportInterval.Seconds()))

	for {
		time.Sleep(m.reportInterval)
		buf, err := json.Marshal(map[string]interface{}{
			"windows":               m.windowsCounter.Count(),
			"linux":                 m.linuxCounter.Count(),
			"osx":                   m.osxCounter.Count(),
			"other":                 m.otherCounter.Count(),
			"httpTunnelMeter.count": m.httpTunnelMeter.Count(),
			"tcpTunnelMeter.count":  m.tcpTunnelMeter.Count(),
			"tunnelMeter.count":     m.tunnelMeter.Count(),
			"tunnelMeter.m1":        m.tunnelMeter.Rate1(),
			"bytesIn.count":         m.bytesInCount.Count(),
			"bytesOut.count":        m.bytesOutCount.Count(),
		})

		if err != nil {
			_ = m.Error("Failed to serialize metrics: %v", err)
			continue
		}

		m.Info("Reporting: %s", buf)
	}
}
