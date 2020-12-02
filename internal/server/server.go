package server

import (
	"crypto/tls"
	"net"
	"sync"
	"time"

	"github.com/mikumaycry/akari/internal/config"
	"github.com/mikumaycry/akari/internal/pkg/http"
	"github.com/mikumaycry/akari/internal/pkg/socks5"
	"github.com/mikumaycry/akari/internal/pkg/tcp"
	"github.com/mikumaycry/akari/internal/utils"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// Server wraps hold tls.Listner and distribute request to pkg based on sni
type Server struct {
	wg        sync.WaitGroup
	tlsConfig *tls.Config
	ln        net.Listener
	confs     map[string]config.ServerConf
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
		tlsConfig: tlsConfig,
		ln:        ln,
		confs:     confs,
	}
	return s, nil
}

// Serve method
func (s *Server) Serve() error {
	log.Infof("start listening %s", s.ln.Addr())
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
	"https":  http.HandleConn,
}
