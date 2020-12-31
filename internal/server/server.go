package server

import (
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
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleConn(conn)
		}()
	}
}

// Close method
func (s *Server) Close() error {
	s.wg.Wait()
	s.closeChan <- struct{}{}
	return s.ln.Close()
}

func (s *Server) handleConn(conn net.Conn) {
	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		conn.Close()
		return
	}
	if err := tlsConn.Handshake(); err != nil {
		log.WithField("Remote", conn.RemoteAddr()).Error("tlsCon.Handshake: ", err)
		tlsConn.Close()
		return
	}
	sni := tlsConn.ConnectionState().ServerName
	if len(sni) == 0 {
		sni = "empty"
	}
	dst, ok := s.confs[sni]
	if !ok {
		log.WithField("Remote", tlsConn.RemoteAddr()).Errorf("invalid SNI: %s", sni)
		tlsConn.Close()
		return
	}
	funcMap[dst.Mode](tlsConn, &dst)
}

var funcMap = map[string]func(conn *tls.Conn, cfg *config.ServerConf){
	"tcp":    tcp.HandleConn,
	"socks5": socks5.HandleConn,
	"https":  https.HandleConn,
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
