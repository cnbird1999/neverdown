package main

import (
	"log"
	"os"
	"net/http"
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

func indexHandler(ra *monitoring.Raft) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Set:%v", ra.SetData("ok"))
		WriteJSON(w, "ok")
	}
}

func main() {
	r, err := monitoring.NewRaft(os.Getenv("UPCHECK_PREFIX"), os.Getenv("UPCHECK_ADDR"))
	log.Printf("%+v/%v", r, err)
	defer r.Close()
	r2 := mux.NewRouter()
	r2.HandleFunc("/", indexHandler(r))
	http.Handle("/public/", http.StripPrefix("/public", http.FileServer(http.Dir("public"))))
	http.Handle("/", r2)
	http.ListenAndServe(os.Getenv("UPCHECK_HTTP"), nil)
	for {
		time.Sleep(time.Second)
	}
}