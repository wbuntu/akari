package server

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mikumaycry/akari/internal/config"
	"github.com/mikumaycry/akari/internal/pkg/https"
	"github.com/mikumaycry/akari/internal/pkg/socks5"
	"github.com/mikumaycry/akari/internal/pkg/tcp"
	"github.com/mikumaycry/akari/internal/utils"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/xtaci/smux"
)

// Server wraps hold tls.Listner and distribute request to pkg based on sni
type Server struct {
	wg           sync.WaitGroup
	tlsConfig    *tls.Config
	ln           net.Listener
	httpsPort    string
	httpRedirect bool
	closeChan    chan struct{}
	confs        map[string]config.ServerConf
}

// New method
func New(cfg *config.Config) (*Server, error) {
	confs, err := loadServerConf(cfg.Conf)
	if err != nil {
		return nil, errors.Wrap(err, "loadServerConf")
	}
	tlsConfig := &tls.Config{
		MinVersion:               tls.VersionTLS12,
		PreferServerCipherSuites: true,
	}
	if cfg.TLS.ForwardSecurity {
		tlsConfig.CipherSuites = utils.CipherSuites()
	}
	if len(cfg.TLS.Certs) == 0 {
		return nil, errors.New("empty TLS certs")
	}
	for _, v := range cfg.TLS.Certs {
		kp, err := tls.LoadX509KeyPair(v.Cert, v.Key)
		if err != nil {
			return nil, errors.Wrap(err, "tls.LoadX509KeyPair")
		}
		tlsConfig.Certificates = append(tlsConfig.Certificates, kp)
	}
	ln, err := tls.Listen("tcp", cfg.Addr, tlsConfig)
	if err != nil {
		return nil, errors.Wrap(err, "tls.Listen")
	}
	s := &Server{
		tlsConfig:    tlsConfig,
		ln:           ln,
		httpsPort:    strings.Split(cfg.Addr, ":")[1],
		httpRedirect: cfg.HTTPRedirect,
		closeChan:    make(chan struct{}, 1),
		confs:        confs,
	}
	return s, nil
}

// Serve method
func (s *Server) Serve() error {
	log.Infof("start listening %s", s.ln.Addr())
	if s.httpRedirect {
		log.Info("start listening [::]:80")
		go s.handleHTTPRedirect()
	}
	var tempDelay time.Duration
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				log.Errorf("server: Accept error: %v; retrying in %v", err, tempDelay)
				time.Sleep(tempDelay)
				continue
			}
			log.Fatalf("server: Accept error: %s", err)
		}
		tempDelay = 0
		tlsConn, ok := conn.(*tls.Conn)
		if !ok {
			conn.Close()
			continue
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleConn(tlsConn)
		}()
	}
}

// Close method
func (s *Server) Close() error {
	s.wg.Wait()
	s.closeChan <- struct{}{}
	return s.ln.Close()
}

func (s *Server) handleConn(tlsConn *tls.Conn) {
	logger := log.WithField("Remote", tlsConn.RemoteAddr())
	if err := tlsConn.Handshake(); err != nil {
		logger.Error("tlsConn.Handshake: ", err)
		tlsConn.Close()
		return
	}
	sni := tlsConn.ConnectionState().ServerName
	if len(sni) == 0 {
		sni = "empty"
	}
	cfg, ok := s.confs[sni]
	if !ok {
		logger.Errorf("invalid SNI: %s", sni)
		tlsConn.Close()
		return
	}
	logger = logger.WithFields(log.Fields{
		"Mode": cfg.ConnMode(),
		"SNI":  sni,
		"TLS":  utils.TLSFormatString(tlsConn),
	})
	logger.Info("Open Conn")
	defer logger.Info("Close Conn")
	if cfg.Mux {
		handleMuxConn(tlsConn, &cfg, logger)
	} else {
		handleSingleConn(tlsConn, &cfg, logger)
	}
}

func handleMuxConn(srcConn net.Conn, cfg *config.ServerConf, logger *log.Entry) {
	defer srcConn.Close()
	muxCfg := smux.DefaultConfig()
	session, err := smux.Server(srcConn, muxCfg)
	if err != nil {
		logger.Errorf("smux.Server: %s", err)
		return
	}
	defer session.Close()
	for {
		stream, err := session.AcceptStream()
		if err != nil {
			logger.Errorf("session.AcceptStream: %s", err)
			return
		}
		go handleSingleConn(stream, cfg, logger)
	}
}

type bufferdConn struct {
	net.Conn
	br *bufio.Reader
}

func (c *bufferdConn) Read(b []byte) (int, error) {
	return c.br.Read(b)
}

func handleSingleConn(srcConn net.Conn, cfg *config.ServerConf, logger *log.Entry) {
	defer srcConn.Close()
	switch cfg.Mode {
	case "tcp":
		logger = logger.WithField("DST", cfg.Addr)
		tcp.HandleConn(srcConn, cfg, logger)
	case "socks5":
		socks5.HandleConn(srcConn, cfg, logger)
	case "https":
		https.HandleConn(srcConn, cfg, logger)
	case "auto":
		br := bufio.NewReader(srcConn)
		b, err := br.Peek(1)
		if err != nil {
			logger.Errorf("br.Peek: %s", err)
			return
		}
		bfConn := &bufferdConn{Conn: srcConn, br: br}
		switch b[0] {
		case 0x05:
			// socks5
			socks5.HandleConn(bfConn, cfg, logger)
		default:
			// http
			https.HandleConn(bfConn, cfg, logger)
		}
	default:
		logger.Errorf("invalid mode: %s", cfg.Mode)
	}
}

func (s *Server) handleHTTPRedirect() {
	httpSNI := make(map[string]struct{})
	for _, v := range s.confs {
		if v.Mode == "https" {
			httpSNI[v.SNI] = struct{}{}
		}
	}
	if len(httpSNI) == 0 {
		return
	}
	redirect := func(w http.ResponseWriter, req *http.Request) {
		logger := log.WithFields(log.Fields{"Mode": "http", "Remote": req.RemoteAddr})
		_, ok := httpSNI[req.Host]
		if !ok {
			logger.Infof("not found: %s", req.Host)
			http.NotFound(w, req)
			return
		}
		target := fmt.Sprintf("https://%s%s", req.Host, req.URL.Path)
		if s.httpsPort != "443" {
			target = fmt.Sprintf("https://%s:%s%s", req.Host, s.httpsPort, req.URL.Path)
		}
		if len(req.URL.RawQuery) > 0 {
			target += "?" + req.URL.RawQuery
		}
		logger.Infof("redirect: %s", target)
		http.Redirect(w, req, target, http.StatusMovedPermanently)
	}
	srv := &http.Server{
		Addr:         ":80",
		WriteTimeout: 60 * time.Second,
		ReadTimeout:  30 * time.Second,
		Handler:      http.HandlerFunc(redirect),
	}
	go srv.ListenAndServe()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()
		srv.Shutdown(ctx)
	}()
	<-s.closeChan
}
