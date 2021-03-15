package https

import (
	"bufio"
	"encoding/base64"
	"net"
	"net/http"
	"strings"

	"github.com/mikumaycry/akari/internal/config"
	"github.com/mikumaycry/akari/internal/pkg/transport"
	log "github.com/sirupsen/logrus"
)

// HandleConn handle http connect
func HandleConn(srcConn net.Conn, cfg *config.ServerConf, origLogEntry *log.Entry) {
	defer srcConn.Close()
	req, err := http.ReadRequest(bufio.NewReader(srcConn))
	if err != nil {
		origLogEntry.Errorf("http.ReadRequest: %s", err)
		return
	}
	defer req.Body.Close()
	resp := &http.Response{
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     http.Header{},
	}
	dstAddr := req.URL.Host
	if len(dstAddr) == 0 {
		dstAddr = req.Host
	}
	if len(dstAddr) == 0 {
		origLogEntry.Error("empty dstAddr")
		resp.StatusCode = http.StatusBadRequest
		resp.Write(srcConn)
		return
	}
	httpSNI := stripPort(dstAddr)
	if dstAddr == httpSNI {
		dstAddr = net.JoinHostPort(dstAddr, "80")
	}
	logEntry := origLogEntry.WithField("DST", httpSNI)
	if httpSNI == cfg.SNI {
		reverseProxyDst, ok := cfg.ReverseProxy[req.URL.Path]
		if !ok {
			if len(cfg.Addr) == 0 {
				logEntry.Errorf("empty path: %s", req.URL.Path)
				resp.StatusCode = http.StatusBadRequest
				resp.Write(srcConn)
				return
			}
			reverseProxyDst = cfg.Addr
		}
		remoteIP := stripPort(srcConn.RemoteAddr().String())
		localIP := stripPort(srcConn.LocalAddr().String())
		req.Header.Set("X-Real-IP", remoteIP)
		req.Header.Set("X-Forwarded-For", remoteIP)
		req.Header.Add("X-Forwarded-For", localIP)
		dstConn, err := net.Dial("tcp", reverseProxyDst)
		if err != nil {
			logEntry.Errorf("net.Dial: %s", err)
			resp.StatusCode = http.StatusServiceUnavailable
			resp.Write(srcConn)
			return
		}
		defer dstConn.Close()
		if err = req.Write(dstConn); err != nil {
			logEntry.Errorf("req.Write: %s", err)
			return
		}
		transport.Transport(srcConn, dstConn)
	}
	if cfg.DisableForwardProxy {
		return
	}
	if ok := authenticate(srcConn, cfg, req, resp); !ok {
		origLogEntry.Error("invalid auth")
		return
	}
	defer logEntry.Info("Close DST")
	logEntry.Info("Open DST")
	req.Header.Del("Proxy-Authorization")
	dstConn, err := net.Dial("tcp", dstAddr)
	if err != nil {
		logEntry.Errorf("net.Dial: %s", err)
		resp.StatusCode = http.StatusServiceUnavailable
		resp.Write(srcConn)
		return
	}
	defer dstConn.Close()
	if req.Method == http.MethodConnect {
		b := []byte("HTTP/1.1 200 Connection established\r\n" +
			"Proxy-Agent: Akari" + "\r\n\r\n")
		srcConn.Write(b)
	} else {
		req.Header.Del("Proxy-Connection")
		if err = req.Write(dstConn); err != nil {
			logEntry.Errorf("req.Write: %s", err)
			return
		}
	}
	transport.Transport(srcConn, dstConn)
}

func authenticate(conn net.Conn, cfg *config.ServerConf, req *http.Request, resp *http.Response) bool {
	if cfg.Auth == "" {
		return true
	}
	ok := basicProxyAuth(req.Header.Get("Proxy-Authorization"), cfg)
	if ok {
		return true
	}
	resp.StatusCode = http.StatusProxyAuthRequired
	resp.Header.Add("Proxy-Authenticate", "Basic realm=\"Akari\"")
	resp.Write(conn)
	return false
}

func basicProxyAuth(proxyAuth string, cfg *config.ServerConf) bool {
	if proxyAuth == "" {
		return false
	}
	if !strings.HasPrefix(proxyAuth, "Basic ") {
		return false
	}
	c, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(proxyAuth, "Basic "))
	if err != nil {
		return false
	}
	cs := string(c)
	return cs == cfg.Auth
}

// borrowed from `proxy` plugin
func stripPort(address string) string {
	// Keep in mind that the address might be a IPv6 address
	// and thus contain a colon, but not have a port.
	portIdx := strings.LastIndex(address, ":")
	ipv6Idx := strings.LastIndex(address, "]")
	if portIdx > ipv6Idx {
		address = address[:portIdx]
	}
	return address
}
