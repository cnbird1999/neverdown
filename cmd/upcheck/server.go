package main

import (
	"log"
	"os"
	"net/http"
	"io/ioutil"
	"time"
	"encoding/json"

	"github.com/gorilla/mux"
	"github.com/tsileo/monitoring"
)


func WriteJSON(w http.ResponseWriter, data interface{}) {
	js, err := json.Marshal(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func checksHandler(ra *monitoring.Raft) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			res := map[string][]*monitoring.Check{
				"checks": []*monitoring.Check{},
			}
			for _, check := range ra.Store.ChecksIndex {
				res["checks"] = append(res["checks"], check)
			}
			WriteJSON(w, res)
		case "POST":
			data, err := ioutil.ReadAll(r.Body)
			if err != nil {
				panic(err)
			}
			defer r.Body.Close()
			msg := make([]byte, len(data)+1)
			msg[0] = 0
			copy(msg[1:], data)
			if err := ra.ExecCommand(msg); err != nil {
				panic(err)
			}
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}

func checkHandler(ra *monitoring.Raft) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		switch r.Method {
		case "GET":
			check, exists := ra.Store.ChecksIndex[vars["id"]]
			if exists {
				WriteJSON(w, check)
			} else {
				http.Error(w, http.StatusText(404), 404)
			}
		case "DELETE":

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}

func main() {
	r, err := monitoring.NewRaft(os.Getenv("UPCHECK_PREFIX"), os.Getenv("UPCHECK_ADDR"))
	log.Printf("%+v/%v", r, err)
	defer r.Close()
	r2 := mux.NewRouter()
	r2.HandleFunc("/check", checksHandler(r))
	r2.HandleFunc("/check/{id}", checkHandler(r))
	http.Handle("/public/", http.StripPrefix("/public", http.FileServer(http.Dir("public"))))
	http.Handle("/", r2)
	http.ListenAndServe(os.Getenv("UPCHECK_HTTP"), nil)
	for {
		time.Sleep(time.Second)
	}
}