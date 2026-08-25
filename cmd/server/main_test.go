package main

import (
	"net/http"
	"testing"
)

func TestNewHTTPServerConfiguresResourceLimits(t *testing.T) {
	handler := http.NewServeMux()
	server := newHTTPServer("127.0.0.1:8080", handler)

	if server.Addr != "127.0.0.1:8080" {
		t.Errorf("Addr = %q, want %q", server.Addr, "127.0.0.1:8080")
	}
	if server.Handler != handler {
		t.Error("Handler does not match the supplied handler")
	}
	if server.ReadHeaderTimeout != readHeaderTimeout {
		t.Errorf("ReadHeaderTimeout = %v, want %v", server.ReadHeaderTimeout, readHeaderTimeout)
	}
	if server.ReadTimeout != readTimeout {
		t.Errorf("ReadTimeout = %v, want %v", server.ReadTimeout, readTimeout)
	}
	if server.IdleTimeout != idleTimeout {
		t.Errorf("IdleTimeout = %v, want %v", server.IdleTimeout, idleTimeout)
	}
	if server.MaxHeaderBytes != maximumHeaderSizeBytes {
		t.Errorf("MaxHeaderBytes = %d, want %d", server.MaxHeaderBytes, maximumHeaderSizeBytes)
	}
	if server.WriteTimeout != 0 {
		t.Errorf("WriteTimeout = %v, want zero for SSE", server.WriteTimeout)
	}
}
