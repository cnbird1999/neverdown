package neverdown

import (
	"net/http"
	"net"
	"log"
	"encoding/json"
	"strconv"

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

// RedirectToLeader redirects the request to the leader if needed.
func RedirectToLeader(leader *bool, ra *Raft, handlerFunc http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if *leader {
			handlerFunc.ServeHTTP(w, r)
		} else {
			hostname := "localhost"
			bhostname := ra.Leader().(*net.TCPAddr).IP
			if bhostname != nil {
				hostname = string(bhostname)
			}
			redirectTo := "http://"+hostname+":"+strconv.Itoa(ra.Leader().(*net.TCPAddr).Port-10)+r.URL.Path
			log.Printf("Redirect request to leader: %v", redirectTo)
			http.Redirect(w, r, redirectTo, http.StatusTemporaryRedirect)
		}
	}
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
			defer r.Body.Close()
			check := NewCheck()
			if err := json.NewDecoder(r.Body).Decode(check); err != nil {
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
			log.Printf("local /_ping request: %+v", pr)
			WriteJSON(w, pr)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}

func APIListenAndserve(leader *bool, ra *Raft, sched *Scheduler) error {
	r := mux.NewRouter()
	r.HandleFunc("/_cluster", clusterHandler(sched.Reloadch, ra))
	r.HandleFunc("/_ping", pingHandler(ra))
	r.HandleFunc("/check", RedirectToLeader(leader, ra, checksHandler(sched.Reloadch, ra)))
	r.HandleFunc("/check/{id}", RedirectToLeader(leader, ra, checkHandler(sched.Reloadch, ra)))
	http.Handle("/", r)
	return http.ListenAndServe(ResolveAPIAddr(ra.Addr), nil)
}
