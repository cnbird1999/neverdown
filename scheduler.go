package neverdown

import (
	"encoding/json"
	"log"
	"sort"
	"sync"
	"time"
)

// Scheduler schedules checks, it also manage the WebHookScheduler.
type Scheduler struct {
	raft         *Raft
	webhookSched *WebHookScheduler
	stop         chan struct{}
	Reloadch     chan struct{}
	running      bool
	checks       []*Check
}

// NewScheduler initializes a new empty Scheduler.
func NewScheduler(raft *Raft, webhookSched *WebHookScheduler) *Scheduler {
	return &Scheduler{
		raft:         raft,
		webhookSched: webhookSched,
		stop:         make(chan struct{}),
		Reloadch:     make(chan struct{}),
	}
}

// Stop shutdowns the Scheduler cleanly.
func (d *Scheduler) Stop() {
	log.Println("Stoppping scheduler...")
	d.webhookSched.Stop()
	d.stop <- struct{}{}
}

// Reload will recompute the next execution time of every checks.
func (d *Scheduler) Reload() {
	d.webhookSched.Reload()
	d.Reloadch <- struct{}{}
}

func (d *Scheduler) updateChecks() error {
	d.checks = []*Check{}
	for _, check := range d.raft.Store.ChecksIndex {
		d.checks = append(d.checks, check)
	}
	return nil
}

// Run starts the processing of jobs, and listens for config update.
func (d *Scheduler) Run() {
	go d.webhookSched.Run()
	log.Println("Starting scheduler...")
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
				if !check.Next.IsZero() {
					check.Prev = check.Next
				}
				go func(check *Check) {
					oldStatus := check.Up
					LeaderCheck(d.raft, check)
					if !check.Next.IsZero() {
						check.LastCheck = check.Next.Unix()
					}
					// Re-compute the uptime percentage
					if check.TimeDown > 0 {
						total := check.Interval * check.Pings
						check.Uptime = float32(int64(total)-check.TimeDown) / float32(total)
						log.Printf("uptime:%+v", check)
					}
					if check.Up != oldStatus {
						log.Printf("Check %v status changed from %v to %v", check.ID, oldStatus, check.Up)
						var wg sync.WaitGroup
						wg.Add(3)
						go func(check *Check) {
							defer wg.Done()
							if d.raft.Producer == nil {
								return
							}
							js, err := json.Marshal(check)
							if err != nil {
								panic(err)
							}
							if err := d.raft.Producer.Publish("neverdown", js); err != nil {
								panic(err)
							}
						}(check)
						go func(check *Check) {
							defer wg.Done()
							if err := NotifyEmails(check); err != nil {
								panic(err)
							}
						}(check)
						go func(check *Check) {
							defer wg.Done()
							if err := ExecuteWebhooks(d.raft, d.webhookSched, check); err != nil {
								panic(err)
							}
						}(check)
						wg.Wait()
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
			d.updateChecks()
		}
	}
}
