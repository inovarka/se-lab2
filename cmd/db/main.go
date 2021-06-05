package main

import (
	"encoding/json"
	"errors"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"github.com/inovarka/se-lab2/datastore"
	"github.com/inovarka/se-lab2/httptools"
	"github.com/inovarka/se-lab2/signal"
)

func main() {
	var (
		port = flag.Int("port", 8080, "server port")
		dir  = flag.String("dir", ".", "database storage dir")
	)
	flag.Parse()

	db, err := datastore.NewDatastore(*dir)
	if err != nil {
		log.Printf("cannot create database instance: %v\n", err)

		return
	}

	h := new(http.ServeMux)
	h.HandleFunc("/db/", func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")

		key := strings.TrimPrefix(r.URL.Path, "/db/")
		if r.Method == http.MethodGet {
			var value []byte

			value, err = db.Get(key)
			if errors.Is(err, datastore.ErrNotFound) || value == nil {
				rw.WriteHeader(http.StatusNotFound)

				return
			}

			if err != nil {
				rw.WriteHeader(http.StatusInternalServerError)

				return
			}

			b, err := json.Marshal(GetResponse{Key: key, Value: value})
			if err != nil {
				rw.WriteHeader(http.StatusInternalServerError)

				return
			}

			if _, err = rw.Write(b); err != nil {
				rw.WriteHeader(http.StatusInternalServerError)

				return
			}
		} else if r.Method == http.MethodPost {
			body, err := ioutil.ReadAll(r.Body)
			defer func(Body io.ReadCloser) {
				_ = Body.Close()
			}(r.Body)

			if err != nil {
				rw.WriteHeader(http.StatusBadRequest)

				return
			}

			var req PutRequest

			if err = json.Unmarshal(body, &req); err != nil {
				rw.WriteHeader(http.StatusBadRequest)

				return
			}

			if err = db.Put(key, req.Value); err != nil {
				rw.WriteHeader(http.StatusInternalServerError)

				return
			}

			rw.WriteHeader(http.StatusOK)
		}
	})

	httptools.CreateServer(*port, h).Start()
	signal.WaitForTerminationSignal()
}
