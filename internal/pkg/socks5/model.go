package socks5

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"strconv"

	"github.com/pkg/errors"
)

const (
	verSocks5 = 0x05
	rsvSocks5 = 0x00
)

const (
	socks5AuthMethodNone       = 0x00
	socks5AuthMethodGSSAPI     = 0x01
	socks5AuthMethodUserPasswd = 0x02
	// X'03' to X'7F' IANA ASSIGNED
	// X'80' to X'FE' RESERVED FOR PRIVATE METHODS
	socks5AuthMethodNoAcceptable = 0xff
)

const (
	// The VER field contains the current version of the subnegotiation, which is X'01'.
	verAuthMethodUserPasswd = 0x01
	// A STATUS field of X'00' indicates success. If the server returns a
	// `failure' (STATUS value other than X'00') status
	socks5AuthMethodUserPasswdSuccess = 0x00
	socks5AuthMethodUserPasswdFailed  = 0x01
)

const (
	socks5CmdConnect = 0x01
	socks5CmdBind    = 0x02
	socks5CmdUDP     = 0x03
)

const (
	socks5AddrTypeIPv4   = 0x01
	socks5AddrTypeDomain = 0x03
	socks5AddrTypeIPv6   = 0x04
)

const (
	socks5RepSuccesss            = 0x00
	socks5RepServerFailure       = 0x01
	socks5RepNotAllowed          = 0x02
	socks5RepNetworkUnreachable  = 0x03
	socks5RepHostUnreachable     = 0x04
	socks5RepConnRefused         = 0x05
	socks5RepTLLExpired          = 0x06
	socks5RepCmdUnsupported      = 0x07
	socks5RepAddrTypeUnsupported = 0x08
	// X'09' to X'FF' unassigned
)

type methodReq struct {
	ver     byte
	nm      byte
	methods []byte
}

func (r *methodReq) read(srcConn net.Conn) error {
	if err := binary.Read(srcConn, binary.BigEndian, &r.ver); err != nil {
		return errors.Wrap(err, "read version")
	}
	if r.ver != verSocks5 {
		return errors.Errorf("unsupported protocol version: %0x", r.ver)
	}
	if err := binary.Read(srcConn, binary.BigEndian, &r.nm); err != nil {
		return errors.Wrap(err, "read num of methods")
	}
	r.methods = make([]byte, r.nm)
	if _, err := io.ReadFull(srcConn, r.methods); err != nil {
		return errors.Wrap(err, "read methods")
	}
	if bytes.IndexByte(r.methods, socks5AuthMethodNone) == -1 && bytes.IndexByte(r.methods, socks5AuthMethodUserPasswd) == -1 {
		srcConn.Write([]byte{verSocks5, socks5AuthMethodNoAcceptable})
		return errors.Errorf("invalid method: %v", r.methods)
	}
	return nil
}

type authReq struct {
	ver    byte
	nu     byte
	np     byte
	uname  []byte
	passwd []byte
}

func (r *authReq) read(srcConn net.Conn, auth string) error {
	if len(auth) == 0 {
		if _, err := srcConn.Write([]byte{verSocks5, socks5AuthMethodNone}); err != nil {
			return errors.Wrap(err, "write noauth status")
		}
		return nil
	}
	if _, err := srcConn.Write([]byte{verSocks5, socks5AuthMethodUserPasswd}); err != nil {
		return errors.Wrap(err, "write username/password auth")
	}
	if err := binary.Read(srcConn, binary.BigEndian, &r.ver); err != nil {
		return errors.Wrap(err, "read auth version")
	}
	if r.ver != verAuthMethodUserPasswd {
		return errors.Errorf("invalid auth version: %0x", r.ver)
	}
	if err := binary.Read(srcConn, binary.BigEndian, &r.nu); err != nil {
		return errors.Wrap(err, "read num of username")
	}
	r.uname = make([]byte, r.nu)
	if _, err := io.ReadFull(srcConn, r.uname); err != nil {
		return errors.Wrap(err, "read usename")
	}
	if err := binary.Read(srcConn, binary.BigEndian, &r.np); err != nil {
		return errors.Wrap(err, "read num of password")
	}
	r.passwd = make([]byte, r.np)
	if _, err := io.ReadFull(srcConn, r.passwd); err != nil {
		return errors.Wrap(err, "read password")
	}
	authStr := string(r.uname) + ":" + string(r.passwd)
	if auth != authStr {
		srcConn.Write([]byte{verAuthMethodUserPasswd, socks5AuthMethodUserPasswdFailed})
		return errors.Errorf("invalid auth: %s", authStr)
	}
	if _, err := srcConn.Write([]byte{verAuthMethodUserPasswd, socks5AuthMethodUserPasswdSuccess}); err != nil {
		return errors.Wrap(err, "write username/password auth status")
	}
	return nil
}

type cmdReq struct {
	ver     byte
	cmd     byte
	rsv     byte
	atype   byte
	dstAddr []byte
	dstPort uint16
	dst     string
}

func (r *cmdReq) read(srcConn net.Conn) error {
	if err := binary.Read(srcConn, binary.BigEndian, &r.ver); err != nil {
		return errors.Wrap(err, "read cmd version")
	}

	if r.ver != verSocks5 {
		return errors.Errorf("unsupported protocol version: %0x", r.ver)
	}

	if err := binary.Read(srcConn, binary.BigEndian, &r.cmd); err != nil {
		return errors.Wrap(err, "read cmd")
	}

	if r.cmd != socks5CmdConnect {
		rep := newCmdRep()
		rep.rep = socks5RepCmdUnsupported
		rep.write(srcConn)
		return errors.Errorf("unsupported cmd: %0x", r.cmd)
	}

	if err := binary.Read(srcConn, binary.BigEndian, &r.rsv); err != nil {
		return errors.Wrap(err, "read rsv")
	}

	if err := binary.Read(srcConn, binary.BigEndian, &r.atype); err != nil {
		return errors.Wrap(err, "read address type")
	}

	var na byte
	switch r.atype {
	case socks5AddrTypeIPv4:
		na = net.IPv4len
	case socks5AddrTypeDomain:
		if err := binary.Read(srcConn, binary.BigEndian, &na); err != nil {
			return errors.Wrap(err, "read num of domain address")
		}
	case socks5AddrTypeIPv6:
		na = net.IPv6len
	default:
		return errors.Errorf("unsupported address type: %0x", na)
	}

	r.dstAddr = make([]byte, na)
	if _, err := io.ReadFull(srcConn, r.dstAddr); err != nil {
		return errors.Wrap(err, "read dst address")
	}

	if err := binary.Read(srcConn, binary.BigEndian, &r.dstPort); err != nil {
		return errors.Wrap(err, "read dst port")
	}

	switch r.atype {
	case socks5AddrTypeIPv4:
		r.dst = net.IPv4(r.dstAddr[0], r.dstAddr[1], r.dstAddr[2], r.dstAddr[3]).String()
	case socks5AddrTypeDomain:
		r.dst = string(r.dstAddr)
	case socks5AddrTypeIPv6:
		r.dst = net.IP(r.dstAddr).String()
	}
	r.dst += ":" + strconv.Itoa(int(r.dstPort))
	return nil
}

type cmdRep struct {
	ver     byte
	rep     byte
	rsv     byte
	atyp    byte
	bndAddr []byte
	bndPort uint16
}

func newCmdRep() *cmdRep {
	return &cmdRep{
		ver:     verSocks5,
		rep:     socks5RepSuccesss,
		rsv:     rsvSocks5,
		atyp:    socks5AddrTypeIPv4,
		bndAddr: []byte{0, 0, 0, 0},
		bndPort: 0,
	}
}

func (p *cmdRep) write(srcConn net.Conn) error {
	var buf []byte
	if p.rep != socks5RepSuccesss {
		buf = []byte{
			p.ver,
			p.rep,
			p.rsv,
			socks5AddrTypeIPv4,
			0, 0, 0, 0,
			0, 0,
		}
	} else {
		buf = make([]byte, 0, net.IPv6len+8)
		buf = append(buf, p.ver, p.rep, p.rsv, p.atyp)
		switch p.atyp {
		case socks5AddrTypeIPv4:
			if len(p.bndAddr) < net.IPv4len {
				return errors.New("bndAddr too short")
			}
			buf = append(buf, p.bndAddr[:net.IPv4len]...)
		case socks5AddrTypeDomain:
			if len(p.bndAddr) > 255 {
				return errors.New("bndAddr too large")
			}
			buf = append(buf, byte(len(p.bndAddr)))
			buf = append(buf, p.bndAddr...)
		case socks5AddrTypeIPv6:
			if len(p.bndAddr) < net.IPv6len {
				return errors.New("bndAddr too short")
			}
			buf = append(buf, p.bndAddr[:net.IPv6len]...)
		}
		buf = append(buf, 0, 0)
		binary.BigEndian.PutUint16(buf[len(buf)-2:], p.bndPort)
	}
	if _, err := srcConn.Write(buf); err != nil {
		return errors.Wrap(err, "write")
	}
	return nil
}
