package server

import (
	"net/http"
	"runtime"
	"runtime/debug"
)

// Version is the build version. Override at build time with
// -ldflags "-X github.com/thefcan/ragdesk/api/internal/server.Version=v1.2.3".
var Version = "dev"

type versionResponse struct {
	Service   string `json:"service"`
	Version   string `json:"version"`
	GoVersion string `json:"go_version"`
	Revision  string `json:"revision,omitempty"`
}

// handleVersion reports build metadata, useful for verifying what is deployed.
func (s *Server) handleVersion(w http.ResponseWriter, _ *http.Request) {
	resp := versionResponse{
		Service:   "ragdesk-api",
		Version:   Version,
		GoVersion: runtime.Version(),
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, kv := range info.Settings {
			if kv.Key == "vcs.revision" {
				resp.Revision = kv.Value
			}
		}
	}
	writeJSON(w, http.StatusOK, resp)
}
