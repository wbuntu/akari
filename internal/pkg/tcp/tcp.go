package tcp

import (
	"crypto/tls"
	"net"

	"github.com/mikumaycry/akari/internal/config"
	"github.com/mikumaycry/akari/internal/pkg/transport"
	"github.com/mikumaycry/akari/internal/utils"
	log "github.com/sirupsen/logrus"
	"github.com/xtaci/smux"
)

// HandleConn handle TCP and Mux-TCP
func HandleConn(srcConn *tls.Conn, cfg *config.ServerConf) {
	logEntry := log.WithFields(log.Fields{
		"Mode":   cfg.ConnMode(),
		"SNI":    cfg.SNI,
		"TLS":    utils.TLSFormatString(srcConn),
		"Remote": srcConn.RemoteAddr().String(),
		"DST":    cfg.Addr,
	})
	defer logEntry.Info("Close Conn")
	logEntry.Info("Open Conn")
	if cfg.Mux {
		handleMuxConn(srcConn, cfg, logEntry)
	} else {
		handleSingleConn(srcConn, cfg, logEntry)
	}
}

func handleMuxConn(srcConn net.Conn, cfg *config.ServerConf, logEntry *log.Entry) {
	defer srcConn.Close()
	muxCfg := smux.DefaultConfig()
	if cfg.MuxV2 {
		muxCfg.Version = 2
	}
	session, err := smux.Server(srcConn, muxCfg)
	if err != nil {
		logEntry.Errorf("smux.Server: %s", err)
		return
	}
	defer session.Close()
	for {
		stream, err := session.AcceptStream()
		if err != nil {
			logEntry.Errorf("session.AcceptStream: %s", err)
			return
		}
		go handleSingleConn(stream, cfg, logEntry)
	}
}

func handleSingleConn(srcConn net.Conn, cfg *config.ServerConf, logEntry *log.Entry) {
	defer srcConn.Close()
	dstConn, err := net.Dial("tcp", cfg.Addr)
	if err != nil {
		logEntry.Errorf("net.Dial: %s", err)
		return
	}
	defer dstConn.Close()
	transport.Transport(srcConn, dstConn)
}
