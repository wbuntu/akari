package agent

import (
	"crypto/tls"
	"io"
	"net"
	"sync"
	"time"

	"github.com/mikumaycry/akari/internal/config"
	"github.com/mikumaycry/akari/internal/pkg/mux"
	"github.com/mikumaycry/akari/internal/pkg/transport"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

var (
	defaultIdle = 8
	defaultMux  = 8
)

// Agent hold a slice of Listener
type Agent struct {
	lns []*Listener
}

// New method
func New(cfg *config.Config) (*Agent, error) {
	confs, err := loadAgentConf(cfg.Conf)
	if err != nil {
		return nil, errors.Wrap(err, "loadAgentConf")
	}
	s := &Agent{}
	for _, v := range confs {
		ln, err := net.Listen("tcp", v.Local)
		if err != nil {
			return nil, errors.Wrapf(err, "net.Listen: %v", v)
		}
		dialFn := func() (io.ReadWriteCloser, error) {
			return tls.Dial("tcp", v.Remote, &tls.Config{
				ServerName: v.SNI,
				MinVersion: tls.VersionTLS12,
			})
		}
		listener := &Listener{
			ln:     ln,
			cfg:    v,
			dialFn: dialFn,
		}
		if v.Mux {
			if v.Pool {
				maxIdle, maxMux := defaultIdle, defaultMux
				if v.MaxIdle != 0 {
					maxIdle = v.MaxIdle
				}
				if v.MaxMux != 0 {
					maxMux = v.MaxMux
				}
				listener.pool = mux.NewPool(maxIdle, maxMux, dialFn)
			} else {
				listener.conn = mux.NewConn(dialFn)
			}
		}
		s.lns = append(s.lns, listener)
	}
	return s, nil
}

// Serve method
func (a *Agent) Serve() error {
	for i := range a.lns {
		go func(i int) {
			a.lns[i].serve()
		}(i)
	}
	return nil
}

// Close method
func (a *Agent) Close() error {
	for i := range a.lns {
		a.lns[i].close()
	}
	return nil
}

// Listener provide tcp, mux-tcp and mux-pool conn
type Listener struct {
	ln     net.Listener
	cfg    config.AgentConf
	pool   *mux.Pool
	conn   *mux.Conn
	wg     sync.WaitGroup
	dialFn func() (io.ReadWriteCloser, error)
}

func (l *Listener) serve() error {
	log.Infof("start listening %s", l.ln.Addr())
	var tempDelay time.Duration
	for {
		conn, err := l.ln.Accept()
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
				log.Errorf("agent: Accept error: %v; retrying in %v", err, tempDelay)
				time.Sleep(tempDelay)
				continue
			}
			log.Fatalf("agent: Accept error: %s", err)
		}
		tempDelay = 0
		l.wg.Add(1)
		go func() {
			defer l.wg.Done()
			l.handleConn(conn)
		}()
	}
}

func (l *Listener) close() error {
	l.wg.Wait()
	return l.ln.Close()
}

func (l *Listener) handleConn(srcConn net.Conn) {
	logEntry := log.WithFields(log.Fields{
		"Mode":   l.cfg.ConnMode(),
		"SNI":    l.cfg.SNI,
		"Remote": srcConn.RemoteAddr().String(),
	})
	defer func() {
		logEntry.Info("Close Conn")
		srcConn.Close()
	}()
	logEntry.Info("Open Conn")
	if l.cfg.Mux {
		if l.cfg.Pool {
			l.handlePoolConn(srcConn, logEntry)
		} else {
			l.handleMuxTCPConn(srcConn, logEntry)
		}
	} else {
		l.handleSingleTCPConn(srcConn, logEntry)
	}
}

func (l *Listener) handleSingleTCPConn(srcConn net.Conn, logEntry *log.Entry) {
	dstConn, err := l.dialFn()
	if err != nil {
		logEntry.Errorf("dialFn: %s", err)
		return
	}
	defer dstConn.Close()
	transport.Transport(srcConn, dstConn)
}

func (l *Listener) handleMuxTCPConn(srcConn net.Conn, logEntry *log.Entry) {
	stream, err := l.conn.OpenStream()
	if err != nil {
		logEntry.Errorf("conn.OpenStream: %s", err)
		return
	}
	defer stream.Close()
	transport.Transport(srcConn, stream)
}

func (l *Listener) handlePoolConn(srcConn net.Conn, logEntry *log.Entry) {
	dstConn, err := l.pool.GetStream()
	if err != nil {
		logEntry.Errorf("pool.GetStream: %s", err)
		return
	}
	defer dstConn.Close()
	transport.Transport(srcConn, dstConn)
}
