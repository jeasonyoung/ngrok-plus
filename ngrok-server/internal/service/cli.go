package service

import "flag"

// Options 参数选项
type Options struct {
	httpAddr   string
	httpsAddr  string
	tunnelAddr string
	domain     string
	tlsCrt     string
	tlsKey     string
	logTo      string
	logLevel   string
}

// 解析参数
func parseArgs() *Options {
	httpAddr := flag.String("httpAddr", ":80", "Public address for HTTP Connections,empty string to disable")
	httpsAddr := flag.String("httpsAddr", ":443", "Public address listening for HTTPS connections,empty string to disable")
	tunnelAddr := flag.String("tunnelAddr", ":4443", "Public address listening for ngrok client")
	domain := flag.String("domain", "ngrok-plus.com", "Domain where the tunnels are hosted")
	tlsCrt := flag.String("tlsCrt", "", "Path to a TLS certificate file")
	tlsKey := flag.String("tlsKey", "", "Path to a TLS key file")
	logTo := flag.String("log", "stdout", "Write log message to this file. 'stdout' and 'none' have special meanings")
	logLevel := flag.String("log-level", "DEBUG", "The level of messages to log. One of: DEBUG,INFO,WARNING,ERROR")
	flag.Parse()
	return &Options{
		httpAddr:   *httpAddr,
		httpsAddr:  *httpsAddr,
		tunnelAddr: *tunnelAddr,
		domain:     *domain,
		tlsCrt:     *tlsCrt,
		tlsKey:     *tlsKey,
		logTo:      *logTo,
		logLevel:   *logLevel,
	}
}
