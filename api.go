package neverdown

import (
	"net/http"
	"io/ioutil"
	"log"
	"encoding/json"

	"github.com/gorilla/mux"
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

func checksHandler(reload chan<- struct{}, ra *Raft) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			if err := ra.Sync(); err != nil {
				panic(err)
			}
			res := map[string][]*Check{
				"checks": []*Check{},
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
			check := NewCheck()
			if err := json.Unmarshal(data[1:], check); err != nil {
				panic(err)
			}
			if check.ID == "" {
				check.ID = uuid()
			}
			js, err := json.Marshal(check)
			if err != nil {
				panic(err)
			}
			msg := make([]byte, len(js)+1)
			msg[0] = 0
			copy(msg[1:], js)
			if err := ra.ExecCommand(msg); err != nil {
				panic(err)
			}
			reload<- struct{}{}
			return
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}

func checkHandler(reload chan<- struct{}, ra *Raft) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		switch r.Method {
		case "GET":
			if err := ra.Sync(); err != nil {
				panic(err)
			}
			check, exists := ra.Store.ChecksIndex[vars["id"]]
			if exists {
				WriteJSON(w, check)
			} else {
				http.Error(w, http.StatusText(404), 404)
			}
		case "DELETE":
			id := []byte(vars["id"])
			msg := make([]byte, len(id)+1)
			msg[0] = 1
			copy(msg[1:], id)
			if err := ra.ExecCommand(msg); err != nil {
				panic(err)
			}
			reload<- struct{}{}
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}

func clusterHandler(reload chan<- struct{}, ra *Raft) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			peers := ra.PeersAPI()
			leaderAddr := ResolveAPIAddr(ra.Leader())
			WriteJSON(w, map[string]interface{}{"peers":peers, "leader": leaderAddr})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}

func pingHandler(ra *Raft) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			method := r.FormValue("method")
			if method == "" {
				method = "HEAD"
			}
			pr, _ := PerformCheck(method, r.FormValue("url"))
			log.Printf("PING:%+v", pr)
			WriteJSON(w, pr)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}

func APIListenAndserve(ra *Raft, sched *Scheduler) error {
	r := mux.NewRouter()
	r.HandleFunc("/_cluster", clusterHandler(sched.Reloadch, ra))
	r.HandleFunc("/_ping", pingHandler(ra))
	r.HandleFunc("/check", checksHandler(sched.Reloadch, ra))
	r.HandleFunc("/check/{id}", checkHandler(sched.Reloadch, ra))
	http.Handle("/", r)
	return http.ListenAndServe(ResolveAPIAddr(ra.Addr), nil)
}
