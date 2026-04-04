package artifacts

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
)

type Server struct {
	mu     sync.Mutex
	server *http.Server
	port   string
	baseDir string
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) Start(ctx context.Context, dir string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.server != nil {
		return "http://127.0.0.1:" + s.port, nil
	}

	s.baseDir = dir

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}

	mux := http.NewServeMux()
	fs := http.FileServer(http.Dir(dir))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		fs.ServeHTTP(w, r)
	})

	s.port = fmt.Sprintf("%d", listener.Addr().(*net.TCPAddr).Port)
	s.server = &http.Server{Handler: mux}

	go func() {
		<-ctx.Done()
		s.server.Close()
	}()

	go s.server.Serve(listener)

	return "http://127.0.0.1:" + s.port, nil
}

func (s *Server) UpdateDir(dir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.baseDir = dir
	if s.server != nil {
		mux := http.NewServeMux()
		fs := http.FileServer(http.Dir(dir))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
			fs.ServeHTTP(w, r)
		})
		s.server.Handler = mux
	}
}
