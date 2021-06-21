package socks5

import (
	"net"
	"strconv"

	"github.com/mikumaycry/akari/internal/config"
	"github.com/mikumaycry/akari/internal/pkg/transport"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// HandleConn handle socks5
func HandleConn(srcConn net.Conn, cfg *config.ServerConf, origLogEntry *log.Entry) {
	if err := handleMethod(srcConn); err != nil {
		origLogEntry.Errorf("handleMethod: %s", err)
		return
	}
	if err := handleAuth(srcConn, cfg.Auth); err != nil {
		origLogEntry.Errorf("handleAuth: %s", err)
		return
	}
	cmd, dstAddr, err := handleCmd(srcConn)
	if err != nil {
		origLogEntry.Errorf("handleCmd: %s", err)
		return
	}
	logEntry := origLogEntry.WithField("DST", dstAddr)
	defer logEntry.Info("Close DST")
	logEntry.Info("Open DST")
	switch cmd {
	case socks5CmdConnect:
		handleConnect(dstAddr, logEntry, srcConn)
	case socks5CmdBind:
		handleBind(dstAddr, logEntry, srcConn)
	case socks5CmdUDP:
		handleUDP(dstAddr, logEntry, srcConn)
	}
}

func handleMethod(srcConn net.Conn) error {
	var req methodReq
	if err := req.read(srcConn); err != nil {
		return errors.Wrap(err, "req.read")
	}
	return nil
}

func handleAuth(srcConn net.Conn, auth string) error {
	var req authReq
	if err := req.read(srcConn, auth); err != nil {
		return errors.Wrap(err, "req.read")
	}
	return nil
}

func handleCmd(srcConn net.Conn) (cmd byte, dst string, err error) {
	var req cmdReq
	if err := req.read(srcConn); err != nil {
		return 0, "", errors.Wrap(err, "req.read")
	}
	return req.cmd, req.dst, nil
}

func handleConnect(dstAddr string, logEntry *log.Entry, srcConn net.Conn) {
	dstConn, err := handleConnectDial(dstAddr, srcConn)
	if err != nil {
		logEntry.Errorf("handleConnectDial: %s", err)
		return
	}
	defer dstConn.Close()
	transport.Transport(srcConn, dstConn)
}

func handleBind(dstAddr string, logEntry *log.Entry, srcConn net.Conn) {

}

func handleUDP(dstAddr string, logEntry *log.Entry, srcConn net.Conn) {

}

func handleConnectDial(dstAddr string, srcConn net.Conn) (net.Conn, error) {
	rep := newCmdRep()
	defer func() {
		rep.write(srcConn)
	}()
	dstConn, err := net.Dial("tcp", dstAddr)
	if err != nil {
		switch e := err.(type) {
		case *net.OpError:
			switch e.Err.(type) {
			case *net.DNSError:
				rep.rep = socks5RepHostUnreachable
			}
			rep.rep = socks5RepServerFailure
		default:
			rep.rep = socks5RepServerFailure
		}
		return nil, errors.Wrap(err, "net.Dial")
	}
	host, port, err := net.SplitHostPort(dstConn.LocalAddr().String())
	if err != nil {
		rep.rep = socks5RepServerFailure
		return nil, errors.Wrap(err, "net.SplitHostPort")
	}
	ip := net.ParseIP(host)
	if ipv4 := ip.To4(); ipv4 != nil {
		rep.atyp = socks5AddrTypeIPv4
		rep.bndAddr = ipv4[:net.IPv4len]
	} else if ipv6 := ip.To16(); ipv6 != nil {
		rep.atyp = socks5AddrTypeIPv6
		rep.bndAddr = ipv6[:net.IPv6len]
	}
	prt, err := strconv.Atoi(port)
	if err != nil {
		rep.rep = socks5RepServerFailure
		return nil, errors.Wrap(err, "strconv.Atoi port")
	}
	rep.bndPort = uint16(prt)
	return dstConn, nil
}
