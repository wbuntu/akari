package transport

import (
	"io"
	"sync"
)

var (
	tinyBufferSize   = 512
	smallBufferSize  = 4 * 1024  // 2KB small buffer
	mediumBufferSize = 8 * 1024  // 8KB medium buffer
	largeBufferSize  = 16 * 1024 // 16KB large buffer
	xlargeBufferSize = 32 * 1024 // 32KB large buffer
)

var (
	sPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, smallBufferSize)
		},
	}
	mPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, mediumBufferSize)
		},
	}
	lPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, largeBufferSize)
		},
	}
	xlPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, xlargeBufferSize)
		},
	}
)

// MTransport method
func MTransport(rw1, rw2 io.ReadWriter) error {
	return transport(rw1, rw2, &mPool)
}

// LTransport method
func LTransport(rw1, rw2 io.ReadWriter) error {
	return transport(rw1, rw2, &lPool)
}

// Transport method
func Transport(rw1, rw2 io.ReadWriter) error {
	return transport(rw1, rw2, &xlPool)
}

func transport(rw1, rw2 io.ReadWriter, pool *sync.Pool) error {
	errc := make(chan error, 1)

	go func() {
		errc <- copyBuffer(rw1, rw2, pool)
	}()

	go func() {
		errc <- copyBuffer(rw2, rw1, pool)
	}()

	err := <-errc
	if err != nil && err == io.EOF {
		err = nil
	}
	return err
}

func copyBuffer(dst io.Writer, src io.Reader, pool *sync.Pool) error {
	buf := pool.Get().([]byte)
	defer pool.Put(buf)

	_, err := io.CopyBuffer(dst, src, buf)
	return err
}
