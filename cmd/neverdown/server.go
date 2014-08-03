package main

import (
	"log"
	"os"
	"net/http"
	"strings"
	"errors"
	"crypto/sha256"
	"io/ioutil"
	"time"
	"encoding/json"

	"github.com/tent/hawk-go"
	"github.com/gorilla/mux"
	monitoring "github.com/tsileo/neverdown"
)

var creds map[string]string

func CredentialsLookupFunc(c *hawk.Credentials) error {
    key, ok := creds[c.ID]
    if !ok {
        return errors.New("client ID does not exist: " + c.ID)
    }
    c.Key = key
    c.Hash = sha256.New
    return nil
}

func HawkHandler(handlerFunc http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth, err := hawk.NewAuthFromRequest(r, CredentialsLookupFunc, nil)
		if err != nil {
			w.Header().Set("WWW-Authenticate", "Hawk")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if err := auth.Valid(); err != nil {
			w.Header().Set("WWW-Authenticate", "Hawk")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		//context.Set(req, "token", token)
		handlerFunc.ServeHTTP(w, r)
	}
}

func init() {
    creds = make(map[string]string)
    creds["thomas"] = "debug" // TODO remove this
}

var RaftWarmUpTime = 5*time.Second

func WriteJSON(w http.ResponseWriter, data interface{}) {
	js, err := json.Marshal(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func checksHandler(reload chan<- struct{}, ra *monitoring.Raft) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			if err := ra.Sync(); err != nil {
				panic(err)
			}
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
			reload<- struct{}{}
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}

func checkHandler(reload chan<- struct{}, ra *monitoring.Raft) func(http.ResponseWriter, *http.Request) {
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

func clusterHandler(reload chan<- struct{}, ra *monitoring.Raft) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			log.Printf("URL:%+v", r)
			peers := ra.PeersAPI()
			leaderAddr := monitoring.ResolveAPIAddr(ra.Leader())
			WriteJSON(w, map[string]interface{}{"peers":peers, "leader": leaderAddr})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}

func pingHandler(ra *monitoring.Raft) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			method := r.FormValue("method")
			if method == "" {
				method = "HEAD"
			}
			pr, _ := monitoring.PerformCheck(method, r.FormValue("url"))
			WriteJSON(w, pr)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}

func main() {
	var leader bool
	r, err := monitoring.NewRaft(os.Getenv("UPCHECK_PREFIX"), os.Getenv("UPCHECK_ADDR"), strings.Split(os.Getenv("UPCHECK_PEERS"), ","))
	log.Printf("%+v/%v", r, err)
	defer r.Close()
	sched := monitoring.NewScheduler(r)
	go func() {
		for isLeader := range r.LeaderCh() {
			log.Printf("Leader change %v", isLeader)
			leader = isLeader
			if leader {
				go sched.Run()
				log.Printf("Starting scheduler")
			} else {
				sched.Stop()
				log.Printf("Stopping scheduler")
			}
		}
	}()
	go func() {
		<-time.After(RaftWarmUpTime)
		sched.Reloadch<- struct{}{}
	}()
	r2 := mux.NewRouter()
	r2.HandleFunc("/_cluster", HawkHandler(clusterHandler(sched.Reloadch, r)))
	r2.HandleFunc("/_ping", pingHandler(r))
	r2.HandleFunc("/check", checksHandler(sched.Reloadch, r))
	r2.HandleFunc("/check/{id}", checkHandler(sched.Reloadch, r))
	http.Handle("/", r2)
	http.ListenAndServe(monitoring.ResolveAPIAddr(r.Addr), nil)
	for {
		time.Sleep(time.Second)
	}
}