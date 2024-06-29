package metrics

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"ngrok-common/conn"
	"ngrok-common/log"
	"ngrok-server/internal/consts"
	"ngrok-server/internal/service"
	"os"
	"time"
)

type keenIoMetric struct {
	Collection string
	Event      interface{}
}

type keenIoMetrics struct {
	log.Logger
	ApiKey       string
	ProjectToken string
	HttpClient   http.Client
	Metrics      chan *keenIoMetric
}

func newKeenIoMetrics(batchInterval time.Duration) service.MetricsService {
	metrics := &keenIoMetrics{
		Logger:       log.NewPrefixLogger("metrics"),
		ApiKey:       os.Getenv(consts.KeenApiKey),
		ProjectToken: os.Getenv(consts.KeenProjectToken),
		Metrics:      make(chan *keenIoMetric, 1000),
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				_ = metrics.Error("KeenIoMetrics failed:%v", r)
			}
		}()

		batch := make(map[string][]interface{})
		batchTimer := time.Tick(batchInterval)

		for {
			select {
			case m := <-metrics.Metrics:
				list, ok := batch[m.Collection]
				if !ok {
					list = make([]interface{}, 0)
				}
				batch[m.Collection] = append(list, m.Event)

			case <-batchTimer:
				//no metrics to report
				if len(batch) == 0 {
					continue
				}

				if payload, err := json.Marshal(batch); err != nil {
					_ = metrics.Error("Failed to serialize metrics payload: %v, %v", batch, err)
				} else {
					for key, val := range batch {
						metrics.Debug("Reporting %d metrics for %s", len(val), key)
					}
					_, _ = metrics.AuthedRequest("POST", "/events", bytes.NewReader(payload))
				}
				batch = make(map[string][]interface{})
			}
		}
	}()

	return metrics
}

func (m *keenIoMetrics) OpenConnection(t *service.Tunnel, c conn.Conn) {

}

type KeenStruct struct {
	Timestamp string `json:"timestamp"`
}

func (m *keenIoMetrics) CloseConnection(t *service.Tunnel, c conn.Conn, start time.Time, bytesIn, bytesOut int64) {
	event := struct {
		Keen               KeenStruct `json:"keen"`
		OS                 string
		ClientId           string
		Protocol           string
		Url                string
		User               string
		Version            string
		Reason             string
		Duration           float64
		HttpAuth           bool
		Subdomain          bool
		TunnelDuration     float64
		ConnectionDuration float64
		BytesIn            int64
		BytesOut           int64
	}{
		Keen: KeenStruct{
			Timestamp: start.UTC().Format("2006-01-02T15:04:05.000Z"),
		},
		OS:                 t.GetCtlAuthOS(),
		ClientId:           t.GetCtlId(),
		Protocol:           t.GetReqProcotol(),
		Url:                t.GetUrl(),
		User:               t.GetCtlAuthUser(),
		Version:            t.GetCtlAuthVersion(),
		HttpAuth:           t.GetReqHttpAuth() != "",
		Subdomain:          t.GetReqSubdomain() != "",
		TunnelDuration:     time.Since(t.GetStart()).Seconds(),
		ConnectionDuration: time.Since(start).Seconds(),
		BytesIn:            bytesIn,
		BytesOut:           bytesOut,
	}
	m.Metrics <- &keenIoMetric{Collection: "CloseConnection", Event: event}
}

func (m *keenIoMetrics) OpenTunnel(t *service.Tunnel) {

}

func (m *keenIoMetrics) CloseTunnel(t *service.Tunnel) {
	event := struct {
		Keen      KeenStruct `json:"keen"`
		OS        string
		ClientId  string
		Protocol  string
		Url       string
		User      string
		Version   string
		Reason    string
		Duration  float64
		HttpAuth  bool
		Subdomain bool
	}{
		Keen: KeenStruct{
			Timestamp: t.GetStart().UTC().Format("2006-01-02T15:04:05.000Z"),
		},
		OS:       t.GetCtlAuthOS(),
		ClientId: t.GetCtlId(),
		Protocol: t.GetReqProcotol(),
		Url:      t.GetUrl(),
		User:     t.GetCtlAuthUser(),
		Version:  t.GetCtlAuthVersion(),
		//Reason:
		Duration:  time.Since(t.GetStart()).Seconds(),
		HttpAuth:  t.GetReqHttpAuth() != "",
		Subdomain: t.GetReqSubdomain() != "",
	}

	m.Metrics <- &keenIoMetric{Collection: "CloseTunnel", Event: event}
}

func (m *keenIoMetrics) AuthedRequest(method, path string, body *bytes.Reader) (res *http.Response, err error) {
	path = fmt.Sprintf("https://api.keen.io/3.0/projects/%s%s", m.ProjectToken, path)
	var req *http.Request
	if req, err = http.NewRequest(method, path, body); err != nil {
		return
	}

	req.Header.Add("Authorization", m.ApiKey)
	if body != nil {
		req.Header.Add("Content-Type", "application/json")
		req.ContentLength = int64(body.Len())
	}

	requestStartAt := time.Now()

	if res, err = m.HttpClient.Do(req); err != nil {
		_ = m.Error("Failed to send metric event to keen.io %v", err)
	} else {
		m.Info("keen.io processed requet in %f sec", time.Since(requestStartAt).Seconds())
		defer func(body io.ReadCloser) {
			_ = body.Close()
		}(res.Body)
		if res.StatusCode != 200 {
			_bytes, _ := io.ReadAll(res.Body)
			_ = m.Error("Got %v response from keen.io: %s", res.StatusCode, _bytes)
		}
	}
	return
}
