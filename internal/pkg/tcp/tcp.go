package tcp

import (
	"net"

	"github.com/mikumaycry/akari/internal/config"
	"github.com/mikumaycry/akari/internal/pkg/transport"
	log "github.com/sirupsen/logrus"
)

// HandleConn handle TCP
func HandleConn(srcConn net.Conn, cfg *config.ServerConf, logEntry *log.Entry) {
	dstConn, err := net.Dial("tcp", cfg.Addr)
	if err != nil {
		logEntry.Errorf("net.Dial: %s", err)
		return
	}
	defer dstConn.Close()
	transport.Transport(srcConn, dstConn)
}
