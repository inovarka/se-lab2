package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/inovarka/se-lab2/httptools"
	"github.com/inovarka/se-lab2/signal"
)

const (
	confHealthFailure = "CONF_HEALTH_FAILURE"
)

func main() {
	port := flag.Int(
		"port",
		8080,
		"server port",
	)

	flag.Parse()

	h := http.NewServeMux()

	h.HandleFunc(
		"/health",
		func(rw http.ResponseWriter, r *http.Request) {
			rw.Header().Set("content-type", "text/plain")
			if failConfig := os.Getenv(confHealthFailure); failConfig == "true" {
				rw.WriteHeader(http.StatusInternalServerError)
				_, _ = rw.Write([]byte("FAILURE"))
			} else {
				rw.WriteHeader(http.StatusOK)
				_, _ = rw.Write([]byte("OK"))
			}
		},
	)

	report := make(Report)

	h.HandleFunc(
		"/api/v1/some-data",
		func(rw http.ResponseWriter, r *http.Request) {
			key := r.FormValue("key")
			if key == "" {
				rw.WriteHeader(http.StatusNotFound)

				return
			}

			reqURL := fmt.Sprintf("http://db:8080/db/%s", key)

			resp, err := http.Get(reqURL)
			if err != nil {
				rw.WriteHeader(http.StatusInternalServerError)

				return
			}

			for k, values := range resp.Header {
				for _, value := range values {
					rw.Header().Add(k, value)
				}
			}

			rw.WriteHeader(resp.StatusCode)
			defer resp.Body.Close()

			if _, err = io.Copy(rw, resp.Body); err != nil {
				return
			}
		},
	)

	h.Handle("/report", report)

	server := httptools.CreateServer(*port, h)
	server.Start()

	signal.WaitForTerminationSignal()
}
