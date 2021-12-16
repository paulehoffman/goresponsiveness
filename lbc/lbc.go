package lbc

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"sync/atomic"
)

var chunkSize int = 50

type LoadBearingConnection interface {
	Start(context.Context, bool) bool
	Transferred() uint64
	Client() *http.Client
	IsValid() bool
}

type LoadBearingConnectionDownload struct {
	Path       string
	downloaded uint64
	client     *http.Client
	debug      bool
	valid      bool
}

func (lbd *LoadBearingConnectionDownload) Transferred() uint64 {
	transferred := atomic.LoadUint64(&lbd.downloaded)
	if lbd.debug {
		fmt.Printf("download: Transferred: %v\n", transferred)
	}
	return transferred
}

func (lbd *LoadBearingConnectionDownload) Client() *http.Client {
	return lbd.client
}

func (lbd *LoadBearingConnectionDownload) Start(ctx context.Context, debug bool) bool {
	lbd.downloaded = 0
	lbd.client = &http.Client{}
	lbd.debug = debug
	lbd.valid = true

	// At some point this might be useful: It is a snippet of code that will enable
	// logging of per-session TLS key material in order to make debugging easier in
	// Wireshark.
	/*
		lbd.client = &http.Client{
			Transport: &http2.Transport{
				TLSClientConfig: &tls.Config{
					KeyLogWriter: w,

					Rand:               utilities.RandZeroSource{}, // for reproducible output; don't do this.
					InsecureSkipVerify: true,                       // test server certificate is not trusted.
				},
			},
		}
	*/

	if debug {
		fmt.Printf("Started a load-bearing download.\n")
	}
	go lbd.doDownload(ctx)
	return true
}
func (lbd *LoadBearingConnectionDownload) IsValid() bool {
	return lbd.valid
}

func (lbd *LoadBearingConnectionDownload) doDownload(ctx context.Context) {
	get, err := lbd.client.Get(lbd.Path)
	if err != nil {
		lbd.valid = false
		return
	}
	for ctx.Err() == nil {
		n, err := io.CopyN(ioutil.Discard, get.Body, int64(chunkSize))
		if err != nil {
			lbd.valid = false
			break
		}
		atomic.AddUint64(&lbd.downloaded, uint64(n))
	}
	get.Body.Close()
	if lbd.debug {
		fmt.Printf("Ending a load-bearing download.\n")
	}
}

type LoadBearingConnectionUpload struct {
	Path     string
	uploaded uint64
	client   *http.Client
	debug    bool
	valid    bool
}

func (lbu *LoadBearingConnectionUpload) Transferred() uint64 {
	transferred := atomic.LoadUint64(&lbu.uploaded)
	if lbu.debug {
		fmt.Printf("upload: Transferred: %v\n", transferred)
	}
	return transferred
}

func (lbu *LoadBearingConnectionUpload) Client() *http.Client {
	return lbu.client
}

func (lbu *LoadBearingConnectionUpload) IsValid() bool {
	return lbu.valid
}

type syntheticCountingReader struct {
	n   *uint64
	ctx context.Context
}

func (s *syntheticCountingReader) Read(p []byte) (n int, err error) {
	if s.ctx.Err() != nil {
		return 0, io.EOF
	}
	err = nil
	n = len(p)
	n = chunkSize
	atomic.AddUint64(s.n, uint64(n))
	return
}

func (lbu *LoadBearingConnectionUpload) doUpload(ctx context.Context) bool {
	lbu.uploaded = 0
	s := &syntheticCountingReader{n: &lbu.uploaded, ctx: ctx}
	resp, _ := lbu.client.Post(lbu.Path, "application/octet-stream", s)
	lbu.valid = false
	resp.Body.Close()
	if lbu.debug {
		fmt.Printf("Ending a load-bearing upload.\n")
	}
	return true
}

func (lbu *LoadBearingConnectionUpload) Start(ctx context.Context, debug bool) bool {
	lbu.uploaded = 0
	lbu.client = &http.Client{}
	lbu.debug = debug
	lbu.valid = true

	if debug {
		fmt.Printf("Started a load-bearing upload.\n")
	}
	go lbu.doUpload(ctx)
	return true
}