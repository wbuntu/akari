package mux

import (
	"crypto/tls"
	"math/rand"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/xtaci/smux"
)

// Conn wraps mux session
type Conn struct {
	mu      sync.Mutex
	session *smux.Session
	remote  string
	sni     string
	v2      bool
}

// NewConn method
func NewConn(remote, sni string, v2 bool) *Conn {
	c := &Conn{
		remote: remote,
		sni:    sni,
		v2:     v2,
	}
	return c
}

func (conn *Conn) openSession() error {
	dstConn, err := tls.Dial("tcp", conn.remote, &tls.Config{
		ServerName: conn.sni,
		MinVersion: tls.VersionTLS12,
	})
	if err != nil {
		return errors.Wrap(err, "tls.Dial")
	}
	muxCfg := smux.DefaultConfig()
	if conn.v2 {
		muxCfg.Version = 2
	}
	session, err := smux.Client(dstConn, muxCfg)
	if err != nil {
		return errors.Wrap(err, "smux.Client")
	}
	conn.session = session
	return nil
}

func (conn *Conn) numStreams() int {
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
			return nil, errors.Wrap(err, "conn.openSession")
		}
	}
	stream, err := conn.session.OpenStream()
	if err != nil {
		if err := conn.openSession(); err != nil {
			return nil, errors.Wrap(err, "conn.reopenSession")
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
	MaxIdle  int
	MaxMux   int
	sessions []*Conn
}

// NewPool method
func NewPool(maxIdle, maxMux int, remote, sni string, v2 bool) *Pool {
	p := &Pool{
		MaxIdle:  maxIdle,
		MaxMux:   maxMux,
		sessions: make([]*Conn, maxIdle),
	}
	for i := 0; i < maxIdle; i++ {
		p.sessions[i] = &Conn{
			remote: remote,
			sni:    sni,
			v2:     v2,
		}
	}
	rand.Seed(time.Now().UnixNano())
	return p
}

// GetStream fetch a stream from conn pool
func (p *Pool) GetStream() (*smux.Stream, error) {
	index := rand.Int() % p.MaxIdle
start:
	conn := p.sessions[index]
	if conn.numStreams() > p.MaxMux {
		index++
		index = index % p.MaxIdle
		goto start
	}
	return conn.OpenStream()
}
