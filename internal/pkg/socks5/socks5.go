package socks5

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"strconv"

	"github.com/mikumaycry/akari/internal/config"
	"github.com/mikumaycry/akari/internal/pkg/transport"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// HandleConn handle socks5
func HandleConn(srcConn net.Conn, cfg *config.ServerConf, origLogEntry *log.Entry) {
	defer srcConn.Close()
	if err := handshake(srcConn, cfg); err != nil {
		origLogEntry.Errorf("handshake: %s", err)
		return
	}
	dstAddr, err := handleCmd(srcConn)
	if err != nil {
		origLogEntry.Errorf("handleCmd: %s", err)
		return
	}
	logEntry := origLogEntry.WithField("DST", dstAddr)
	defer logEntry.Info("Close DST")
	logEntry.Info("Open DST")
	dstConn, err := handleDial(dstAddr, srcConn)
	if err != nil {
		logEntry.Errorf("handleDial: %s", err)
		return
	}
	defer dstConn.Close()
	if err := handleResp(srcConn, dstConn); err != nil {
		logEntry.Errorf("handleResp: %s", err)
		return
	}
	transport.Transport(srcConn, dstConn)
}

func handshake(srcConn net.Conn, cfg *config.ServerConf) error {
	var (
		ver     byte
		nm      byte
		methods []byte
		uname   []byte
		passwd  []byte
		nu      byte
		np      byte
	)
	if err := binary.Read(srcConn, binary.BigEndian, &ver); err != nil {
		return errors.Wrap(err, "read version")
	}
	if ver != verSocks5 {
		return errors.Errorf("unsupported protocol version: %0x", ver)
	}
	if err := binary.Read(srcConn, binary.BigEndian, &nm); err != nil {
		return errors.Wrap(err, "read num of bytes of methods")
	}
	methods = make([]byte, nm)
	if _, err := io.ReadFull(srcConn, methods); err != nil {
		return errors.Wrap(err, "read methods")
	}
	if len(cfg.Auth) == 0 {
		if bytes.IndexByte(methods, authNoAuthenticationRequired) == -1 {
			srcConn.Write([]byte{verSocks5, authNoAcceptableMethods})
			return errors.Errorf("invalid method: %v", methods)
		}
		if _, err := srcConn.Write([]byte{verSocks5, authNoAuthenticationRequired}); err != nil {
			return errors.Wrap(err, "write noauth success")
		}
		return nil
	}
	if bytes.IndexByte(methods, authUsernamePassword) == -1 {
		srcConn.Write([]byte{verSocks5, authNoAcceptableMethods})
		return errors.Errorf("invalid method: %v", methods)
	}
	if _, err := srcConn.Write([]byte{verSocks5, authUsernamePassword}); err != nil {
		return errors.Wrap(err, "write auth username/password")
	}
	if err := binary.Read(srcConn, binary.BigEndian, &ver); err != nil {
		return errors.Wrap(err, "read auth ver")
	}
	if ver != authUsernamePasswordVersion {
		return errors.New("invalid auth protocol version")
	}
	if err := binary.Read(srcConn, binary.BigEndian, &nu); err != nil {
		return errors.Wrap(err, "read num of bytes of username")
	}
	uname = make([]byte, nu)
	if _, err := io.ReadFull(srcConn, uname); err != nil {
		return errors.Wrap(err, "read usename")
	}
	if err := binary.Read(srcConn, binary.BigEndian, &np); err != nil {
		return errors.Wrap(err, "read num of bytes of password")
	}
	passwd = make([]byte, np)
	if _, err := io.ReadFull(srcConn, passwd); err != nil {
		return errors.Wrap(err, "read password")
	}
	authStr := string(uname) + ":" + string(passwd)
	if authStr != cfg.Auth {
		srcConn.Write([]byte{authUsernamePasswordVersion, authUsernamePasswordStatusFailure})
		return errors.Errorf("invalid auth: %s", authStr)
	}
	if _, err := srcConn.Write([]byte{authUsernamePasswordVersion, authUsernamePasswordStatusSuccess}); err != nil {
		return errors.Wrap(err, "write auth success")
	}
	return nil
}

func handleCmd(srcConn net.Conn) (dstAddr string, err error) {
	var (
		ver          byte
		cmd          byte
		rsv          byte
		atyp         byte
		dstAddrBytes []byte
		dstPort      uint16
	)
	if err = binary.Read(srcConn, binary.BigEndian, &ver); err != nil {
		err = errors.Wrap(err, "binary.Read ver")
		return
	}

	if ver != verSocks5 {
		err = errors.Errorf("unsupported protocol version: %0x", ver)
		return
	}

	if err = binary.Read(srcConn, binary.BigEndian, &cmd); err != nil {
		err = errors.Wrap(err, "binary.Read cmd")
		return
	}

	if cmd != cmdConnect {
		writeCmdError(srcConn, repComandNotSupported)
		err = errors.Errorf("unsupported cmd: %0x", cmd)
		return
	}

	if err = binary.Read(srcConn, binary.BigEndian, &rsv); err != nil {
		err = errors.Wrap(err, "binary.Read rsv")
		return
	}

	if err = binary.Read(srcConn, binary.BigEndian, &atyp); err != nil {
		err = errors.Wrap(err, "binary.Read address type")
		return
	}

	var ln byte
	switch atyp {
	case atypIPv4Address:
		ln = net.IPv4len
	case atypDomainName:
		if err = binary.Read(srcConn, binary.BigEndian, &ln); err != nil {
			err = errors.Wrap(err, "binary.Read length of domain address")
			return
		}
	case atypIPv6Address:
		ln = net.IPv6len
	default:
		err = errors.New("unsupported address type")
		return
	}
	dstAddrBytes = make([]byte, ln)
	if _, err = io.ReadFull(srcConn, dstAddrBytes); err != nil {
		err = errors.Wrap(err, "io.ReadFull dst address")
		return
	}

	if err = binary.Read(srcConn, binary.BigEndian, &dstPort); err != nil {
		err = errors.Wrap(err, "binary.Read dst port")
		return
	}

	switch atyp {
	case atypIPv4Address:
		dstAddr = net.IPv4(dstAddrBytes[0], dstAddrBytes[1], dstAddrBytes[2], dstAddrBytes[3]).String()
	case atypDomainName:
		dstAddr = string(dstAddrBytes)
	case atypIPv6Address:
		dstAddr = net.IP(dstAddrBytes).String()
	}
	dstAddr += ":" + strconv.Itoa(int(dstPort))
	return
}

func handleDial(dstAddr string, srcConn net.Conn) (net.Conn, error) {
	dstConn, err := net.Dial("tcp", dstAddr)
	if err != nil {
		switch e := err.(type) {
		case *net.OpError:
			switch e.Err.(type) {
			case *net.DNSError:
				writeCmdError(srcConn, repHostUnreachable)
			}
			writeCmdError(srcConn, repGeneralSocksServerFailure)

		default:
			writeCmdError(srcConn, repGeneralSocksServerFailure)
		}
		return nil, errors.Wrap(err, "dial dstAddr")
	}
	return dstConn, nil
}

func handleResp(srcConn net.Conn, dstConn net.Conn) error {
	var (
		atyp         byte
		bndAddrBytes []byte
		bndPort      uint16
	)

	host, port, err := net.SplitHostPort(dstConn.LocalAddr().String())
	if err != nil {
		writeCmdError(srcConn, repGeneralSocksServerFailure)
		return errors.Wrap(err, "net.SplitHostPort")
	}

	ip := net.ParseIP(host)
	if ipv4 := ip.To4(); ipv4 != nil {
		atyp = atypIPv4Address
		bndAddrBytes = ipv4[:net.IPv4len]
	} else {
		atyp = atypIPv6Address
		bndAddrBytes = ip[:net.IPv6len]
	}

	prt, err := strconv.Atoi(port)
	if err != nil {
		writeCmdError(srcConn, repGeneralSocksServerFailure)
		return errors.Wrap(err, "strconv.Atoi port")
	}
	bndPort = uint16(prt)

	if err := writeCmdResp(srcConn, atyp, bndAddrBytes, bndPort); err != nil {
		writeCmdError(srcConn, repGeneralSocksServerFailure)
		return errors.Wrap(err, "writeCmdResp")
	}
	return nil
}

func writeCmdError(srcConn net.Conn, rep byte) error {
	if _, err := srcConn.Write([]byte{
		verSocks5,
		rep,
		rsvReserved,
		atypIPv4Address,
		0, 0, 0, 0,
		0, 0,
	}); err != nil {
		return errors.Wrap(err, "write")
	}
	return nil
}

func writeCmdResp(srcConn net.Conn, atyp byte, bndAddrBytes []byte, bndPort uint16) error {
	buf := make([]byte, 0, net.IPv6len+8)
	buf = append(buf, verSocks5, repSucceeded, rsvReserved, atyp)
	switch atyp {
	case atypIPv4Address:
		if len(bndAddrBytes) < net.IPv4len {
			return errors.New("bndAddrBytes too short")
		}
		buf = append(buf, bndAddrBytes[:net.IPv4len]...)
	case atypDomainName:
		if len(bndAddrBytes) > 255 {
			return errors.New("bndAddrBytes too large")
		}
		buf = append(buf, byte(len(bndAddrBytes)))
		buf = append(buf, bndAddrBytes...)
	case atypIPv6Address:
		if len(bndAddrBytes) < net.IPv6len {
			return errors.New("bndAddrBytes too short")
		}
		buf = append(buf, bndAddrBytes[:net.IPv6len]...)
	}
	buf = append(buf, 0, 0)
	binary.BigEndian.PutUint16(buf[len(buf)-2:], bndPort)
	if _, err := srcConn.Write(buf); err != nil {
		return errors.Wrap(err, "write")
	}
	return nil
}
