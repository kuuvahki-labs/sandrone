package httpapi

import "net/http"

func (s *Server) getLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	snapshot, err := s.rt.Service.Logs()
	writeResult(w, snapshot, err)
}
