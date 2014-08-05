package neverdown

import (
	"log"
	"sort"
	"time"
)

// Scheduler schedule checks, it also manage the WebHookScheduler.
type Scheduler struct {
	raft         *Raft
	webhookSched *WebHookScheduler
	stop         chan struct{}
	Reloadch     chan struct{}
	running      bool
	checks       []*Check
}

// NewScheduler initialize a new empty Scheduler.
func NewScheduler(raft *Raft, webhookSched *WebHookScheduler) *Scheduler {
	return &Scheduler{
		raft:         raft,
		webhookSched: webhookSched,
		stop:         make(chan struct{}),
		Reloadch:     make(chan struct{}),
	}
}

// Stop shutdown the Scheduler cleanly.
func (d *Scheduler) Stop() {
	d.webhookSched.Stop()
	d.stop <- struct{}{}
}

// Reload will recompute the next execution time of every checks.
func (d *Scheduler) Reload() {
	d.Reloadch <- struct{}{}
}

func (d *Scheduler) updateChecks() error {
	d.checks = []*Check{}
	for _, check := range d.raft.Store.ChecksIndex {
		d.checks = append(d.checks, check)
	}
	return nil
}

// Run start the processing of jobs, and listen for config update.
func (d *Scheduler) Run() {
	go d.webhookSched.Run()
	log.Println("Scheduler Run")
	if err := d.updateChecks(); err != nil {
		panic(err)
	}
	now := time.Now().UTC()
	d.running = true
	var checkTime time.Time
	for {
		sort.Sort(byTime(d.checks))
		if d.checks == nil {
			d.updateChecks()
		}
		if d.checks != nil && len(d.checks) == 0 {
			// Sleep for 5 years until the config change
			checkTime = now.AddDate(5, 0, 0)
		} else {
			checkTime = d.checks[0].Next
		}
		select {
		case now = <-time.After(checkTime.Sub(now)):
			for _, check := range d.checks {
				if now.Sub(check.Next) < 0 {
					break
				}
				check.Prev = check.Next
				go func(check *Check) {
					oldStatus := check.Up
					LeaderCheck(d.raft, check)
					if !check.Next.IsZero() {
						check.LastCheck = check.Next.Unix()
					}
					if check.Up != oldStatus {
						if err := ExecuteWebhooks(d.raft, d.webhookSched, check); err != nil {
							panic(err)
						}
					}
					if err := d.raft.ExecCommand(check.ToPostCmd()); err != nil {
						panic(err)
					}
				}(check)
				check.ComputeNext(now)
				continue
			}
		case <-d.stop:
			d.running = false
			return
		case <-d.Reloadch:
			log.Println("config updated")
			d.updateChecks()
		}
	}
}
