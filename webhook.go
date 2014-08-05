package neverdown

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sort"
	"time"
)

var WebHookMaxRetry = 20

// ExecuteWebhooks try to execute all webhooks for a given check,
// if a webhook fail, it will be addedt to the pending webhook and will
// be managed by the WebHookScheduler.
func ExecuteWebhooks(ra *Raft, whSched *WebHookScheduler, check *Check) error {
	log.Println("ExecuteWebhooks")
	errc := make(chan error)
	payload, err := json.Marshal(check)
	if err != nil {
		return err
	}
	for _, url := range check.WebHooks {
		go func() {
			if err := ExecuteWebhook(ra, payload, url); err != nil {
				log.Printf("Failed to execute webhook %v for check %v: %v", url, check.ID, err)
				wh := &WebHook{
					ID:       uuid(),
					URL:      url,
					Payload:  payload,
					Tries:    1,
					FirstTry: time.Now().UTC().Unix(),
				}
				if err := ra.ExecCommand(wh.ToPostCmd()); err != nil {
					errc <- err
				}
				ra.Sync()
				whSched.Reload()
			}
		}()
	}
	for err := range errc {
		if err != nil {
			return err
		}
	}
	return nil
}

// ExecuteWebhook executes a single webhook (POST request to the given url,
// with the given payload).
func ExecuteWebhook(ra *Raft, payload []byte, url string) error {
	log.Printf("ExecuteWebhook %v: %v", string(payload), url)
	var body bytes.Buffer
	body.Write(payload)
	req, err := http.NewRequest("POST", url, &body)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		fmt.Errorf("request failed with status code %v %+v: %v", resp.StatusCode, resp, string(data))
	}
	return nil
}

// WebHookScheduler manages the retries of pending WebHooks.
type WebHookScheduler struct {
	raft            *Raft
	stop            chan struct{}
	Reloadch        chan struct{}
	running         bool
	pendingWebHooks []*WebHook
}

// NewScheduler initialize a new empty Scheduler.
func NewWebHookScheduler(raft *Raft) *WebHookScheduler {
	return &WebHookScheduler{
		raft:     raft,
		stop:     make(chan struct{}),
		Reloadch: make(chan struct{}),
	}
}

// Stop shutdown the Scheduler cleanly.
func (d *WebHookScheduler) Stop() {
	d.stop <- struct{}{}
}

// Reload will recompute the next execution time of every checks.
func (d *WebHookScheduler) Reload() {
	if err := d.raft.Sync(); err != nil {
		panic(err)
	}
	d.Reloadch <- struct{}{}
}

// update the pendingWebHooks slicde from the FSM PendingWebHooksIndex.
func (d *WebHookScheduler) update() error {
	d.pendingWebHooks = []*WebHook{}
	for _, wh := range d.raft.Store.PendingWebHooksIndex {
		d.pendingWebHooks = append(d.pendingWebHooks, wh)
	}
	return nil
}

// Run start the processing of webhooks, and listen for config update.
func (d *WebHookScheduler) Run() {
	if err := d.update(); err != nil {
		panic(err)
	}
	now := time.Now().UTC()
	d.running = true
	var checkTime time.Time
	for {
		sort.Sort(webhookByTime(d.pendingWebHooks))
		if d.pendingWebHooks != nil && len(d.pendingWebHooks) == 0 {
			// Sleep for 5 years until the config change
			checkTime = now.AddDate(5, 0, 0)
		} else {
			checkTime = d.pendingWebHooks[0].Next
		}
		select {
		case now = <-time.After(checkTime.Sub(now)):
			for _, wh := range d.pendingWebHooks {
				if now.Sub(wh.Next) < 0 {
					break
				}
				log.Printf("Retrying webhook %v/%v (tries:%v)", wh.ID, wh.URL, wh.Tries)
				err := ExecuteWebhook(d.raft, wh.Payload, wh.URL)
				if err != nil || wh.Tries == WebHookMaxRetry {
					wh.Tries++
					if err := d.raft.ExecCommand(wh.ToPostCmd()); err != nil {
						panic(err)
					}
					wh.ComputeNext(now)
				} else {
					if wh.Tries == WebHookMaxRetry {
						log.Printf("WARNING: the WebHook %+v will be deleted, after %v failed retries", wh, WebHookMaxRetry)
					}
					// WebHook sucessfully updated
					if err := d.raft.ExecCommand(wh.ToDeleteCmd()); err != nil {
						panic(err)
					}
					d.Reload()
				}
				continue
			}
		case <-d.stop:
			d.running = false
			return
		case <-d.Reloadch:
			log.Println("config updated")
			if err := d.update(); err != nil {
				panic(err)
			}
		}
	}
}
