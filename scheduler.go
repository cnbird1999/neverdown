package neverdown

import (
	"log"
	"sort"
	"time"
)

type Scheduler struct {
	raft *Raft
	stop    chan struct{}
	Reloadch chan struct{}
	running bool
	checks    []*Check
}

func NewScheduler(raft *Raft) *Scheduler {
	return &Scheduler{
		raft: raft,
		stop: make(chan struct{}),
		Reloadch: make(chan struct{}),
	}
}

// Stop shutdown the Scheduler cleanly.
func (d *Scheduler) Stop() {
	d.stop <- struct{}{}
}

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
				go LeaderCheck(d.raft, check)
				d.raft.ExecCommand(check.ToPostCmd())
				check.LastCheck = check.Next.Unix()
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
