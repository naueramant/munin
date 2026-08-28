package assets

import (
	"embed"
	"fmt"
	"net/http"

	"github.com/phayes/freeport"
)

//go:embed static/*
var assets embed.FS

type Server struct {
	port int
	mux  *http.ServeMux
}

func NewServer() (*Server, error) {
	port, err := freeport.GetFreePort()
	if err != nil {
		return nil, err
	}

	return &Server{
		port: port,
		mux:  http.NewServeMux(),
	}, nil
}

func (s *Server) Start() error {
	s.mux.Handle("/", http.FileServer(http.FS(assets)))

	return http.ListenAndServe(fmt.Sprintf(":%d", s.port), s.mux)
}

func (s *Server) Host() string {
	return fmt.Sprintf("http://localhost:%d", s.port)
}

func (s *Server) Port() int {
	return s.port
}
