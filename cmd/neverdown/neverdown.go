package main

import (
	"log"
	"os"
	"strings"
	"time"
	"runtime"

	"github.com/tsileo/neverdown"
)

var (
    githash string = ""
	RaftWarmUpTime = 5*time.Second
)

func main() {
	log.Printf("Starting neverdown version %v+%v; %v (%v/%v)", neverdown.Version, githash, runtime.Version(), runtime.GOOS, runtime.GOARCH)
	leader := new(bool)
	log.Printf("Listening on %v", os.Getenv("UPCHECK_ADDR"))
	r, err := neverdown.NewRaft(os.Getenv("UPCHECK_PREFIX"), os.Getenv("UPCHECK_ADDR"), strings.Split(os.Getenv("UPCHECK_PEERS"), ","))
	if err != nil {
		panic(err)
	}
	defer r.Close()
	webhookSched := neverdown.NewWebHookScheduler(r)
	sched := neverdown.NewScheduler(r, webhookSched)
	go func() {
		for isLeader := range r.LeaderCh() {
			*leader = isLeader
			if *leader {
				go sched.Run()
				log.Printf("Node has been promoted leader")
			} else {
				sched.Stop()
				log.Printf("Node is not leader anymore")
			}
		}
	}()
	go func() {
		<-time.After(RaftWarmUpTime)
		sched.Reloadch<- struct{}{}
	}()
	log.Fatal(neverdown.APIListenAndserve(leader, r, sched))
}
