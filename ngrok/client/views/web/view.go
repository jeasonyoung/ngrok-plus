package web

import (
	"github.com/gorilla/websocket"
	"net/http"
	"ngrok-plus/ngrok/client/assets"
	"ngrok-plus/ngrok/client/mvc"
	"ngrok-plus/ngrok/log"
	"ngrok-plus/ngrok/proto"
	"ngrok-plus/ngrok/util"
	"path"
)

// WebView webView
type WebView struct {
	log.Logger
	ctl mvc.Controller
	// messages sent over this broadcast are sent to all websocket connections
	wsMessages *util.Broadcast
}

func NewWebView(ctl mvc.Controller, addr string) *WebView {
	wv := &WebView{
		Logger:     log.NewPrefixLogger("view", "web"),
		wsMessages: util.NewBroadcast(),
		ctl:        ctl,
	}
	// for now, always redirect to the http view
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/http/in", 302)
	})

	// handle web socket connections
	http.HandleFunc("/_ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Upgrade(w, r, nil, 1024, 1024)

		if err != nil {
			http.Error(w, "Failed websocket upgrade", 400)
			_ = wv.Warn("Failed websocket upgrade: %v", err)
			return
		}

		msgs := wv.wsMessages.Reg()
		defer wv.wsMessages.UnReg(msgs)
		for m := range msgs {
			err := conn.WriteMessage(websocket.TextMessage, m.([]byte))
			if err != nil {
				// connection is closed
				break
			}
		}
	})

	// serve static assets
	http.HandleFunc("/static/", func(w http.ResponseWriter, r *http.Request) {
		buf, err := assets.Asset(path.Join("assets", "client", r.URL.Path[1:]))
		if err != nil {
			_ = wv.Warn("Error serving static file: %s", err.Error())
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(buf)
	})

	wv.Info("Serving web interface on %s", addr)
	wv.ctl.Go(func() { _ = http.ListenAndServe(addr, nil) })
	return wv
}

func (wv *WebView) NewHttpView(proto *proto.Http) *WebHttpView {
	return newWebHttpView(wv.ctl, wv, proto)
}

func (wv *WebView) Shutdown() {
}