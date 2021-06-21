package mux

import (
	"io"
	"math/rand"
	"sync"

	"github.com/pkg/errors"
	"github.com/xtaci/smux"
)

// Conn wraps mux session
type Conn struct {
	mu      sync.Mutex
	session *smux.Session
	dialFn  func() (io.ReadWriteCloser, error)
}

// NewConn method
func NewConn(dialFn func() (io.ReadWriteCloser, error)) *Conn {
	c := &Conn{
		dialFn: dialFn,
	}
	return c
}

func (conn *Conn) openSession() error {
	dstConn, err := conn.dialFn()
	if err != nil {
		return errors.Wrap(err, "conn.dialFn")
	}
	muxCfg := smux.DefaultConfig()
	session, err := smux.Client(dstConn, muxCfg)
	if err != nil {
		return errors.Wrap(err, "smux.Client")
	}
	conn.session = session
	return nil
}

func (conn *Conn) NumStreams() int {
	conn.mu.Lock()
	defer conn.mu.Unlock()
	if conn.session != nil {
		return conn.session.NumStreams()
	}
	return 0
}

// OpenStream warps session's openStream with retry
func (conn *Conn) OpenStream() (*smux.Stream, error) {
	conn.mu.Lock()
	defer conn.mu.Unlock()
	if conn.session == nil {
		if err := conn.openSession(); err != nil {
			return nil, errors.Wrap(err, "openSession")
		}
	}
	stream, err := conn.session.OpenStream()
	if err != nil {
		if err := conn.openSession(); err != nil {
			return nil, errors.Wrap(err, "reopenSession")
		}
		stream, err = conn.session.OpenStream()
	}
	if err != nil {
		return nil, errors.Wrap(err, "session.OpenStream")
	}
	return stream, nil
}

// Pool implement mux conn pool
type Pool struct {
	MaxIdle int
	MaxMux  int
	conns   []*Conn
}

// NewPool method
func NewPool(maxIdle, maxMux int, dialFn func() (io.ReadWriteCloser, error)) *Pool {
	p := &Pool{
		MaxIdle: maxIdle,
		MaxMux:  maxMux,
		conns:   make([]*Conn, maxIdle),
	}
	for i := 0; i < maxIdle; i++ {
		p.conns[i] = &Conn{
			dialFn: dialFn,
		}
	}
	return p
}

// GetStream fetch a stream from conn pool
func (p *Pool) GetStream() (*smux.Stream, error) {
	startIndex := rand.Int() % p.MaxIdle
	endIndex := startIndex + p.MaxIdle
	for i := startIndex; i < endIndex; i++ {
		conn := p.conns[i%p.MaxIdle]
		if conn.NumStreams() < p.MaxMux {
			return conn.OpenStream()
		}
	}
	return nil, errors.New("mux conns run out")
}
