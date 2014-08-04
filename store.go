package neverdown

import (
	"fmt"
	"crypto/rand"
	"encoding/json"
	"sync"
	"time"
	"io"
)

func uuid() string {
    b := make([]byte, 16)
    rand.Read(b)
    b[6] = (b[6] & 0x0f) | 0x40
    b[8] = (b[8] & 0x3f) | 0x80
    return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// Store holds active checks, and pending WebHook notifications.
type Store struct {
	ChecksIndex map[string]*Check
	PendingWebHooksIndex map[string]*WebHook
	mu sync.Mutex
}

// NewStore initialize an empty Store
func NewStore() *Store {
	return &Store{
		ChecksIndex: map[string]*Check{},
		PendingWebHooksIndex: map[string]*WebHook{},
	}
}

// JSON serialize the Store to JSON (used for raft snapshot)
func (s *Store) JSON() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	checks := []*Check{}
	pendingWebhooks := []*WebHook{}
	for _, c := range s.ChecksIndex {
		checks = append(checks, c)
	}
	for _, wh := range s.PendingWebHooksIndex {
		pendingWebhooks = append(pendingWebhooks, wh)
	}
	data := map[string]interface{}{
		"checks": checks,
		"pending_webhooks": pendingWebhooks,
	}
	return json.Marshal(&data)
}

type JSONStore struct {
	Checks []*Check `json:"checks"`
	PendingWebHooks []*WebHook `json:"pending_webhooks"`
}

// FromJSON loads the store from a JSON export.
func (s *Store) FromJSON(r io.Reader) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data := JSONStore{}
	if err := json.NewDecoder(r).Decode(&data); err != nil {
		return err
	}
	for _, check := range data.Checks {
		s.ChecksIndex[check.ID] = check
	}
	for _, webhook := range data.PendingWebHooks {
		s.PendingWebHooksIndex[webhook.ID] = webhook
	}
	return nil
}

// ExecCommand decode a Raft log entry (cmdType byte + JSON encoded payload)
func (s *Store) ExecCommand(data []byte) error {
	cmdType := data[0]
	switch cmdType {
	case 0:
		check := NewCheck()
		if err := json.Unmarshal(data[1:], check); err != nil {
			return err
		}
		if check.WebHooks == nil {
			check.WebHooks = []string{}
			check.Prev = time.Unix(check.LastCheck, 0).UTC()
		}
		s.ChecksIndex[check.ID] = check
	case 1:
		checkID := string(data[1:])
		delete(s.ChecksIndex, checkID)
	case 2:
		webhook := NewWebHook()
		if err := json.Unmarshal(data[1:], webhook); err != nil {
			return err
		}
		s.PendingWebHooksIndex[webhook.ID] = webhook
	case 3:
		webhookID := string(data[1:])
		delete(s.PendingWebHooksIndex, webhookID)

	default:
		panic("unknow cmd type")
	}
	return nil
}

// Check represent an active monitoring check
type Check struct {
	ID string `json:"id"`
	URL string `json:"url"`
	Method string `json:"method"`
	LastCheck int64 `json:"last_check"`
	LastError interface{} `json:"last_error"`
	Up bool `json:"up"`
	LastDown int64 `json:"last_down"`
	Interval int `json:"interval"`
	WebHooks []string `json:"webhooks"`

	Prev time.Time `json:"-"`
	Next time.Time `json:"-"`
}

// NewCheck initialize an empty Check, generates an ID.
func NewCheck() *Check {
	return &Check{
		Prev: time.Time{},
		Next: time.Time{},
		WebHooks: []string{},
		Method: "HEAD",
		Interval: 60, // 60 seconds resolution between checks if no interval is provided.
	}
}

// ComputeNext computes the next check execution time
func (c *Check) ComputeNext(now time.Time) {
	elapsed := now.Sub(c.Next)
	delay := time.Duration(c.Interval)*time.Second
	if elapsed > 0 {
		if c.Next.IsZero() {
			c.Next = now.Add(delay)
		} else {
			c.Next = c.Next.Add(delay)
		}
	}
	return
}

// ToPostCmd serializes a Check into a raft POST command
func (c *Check) ToPostCmd() []byte {
	js, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}
	msg := make([]byte, len(js)+1)
	msg[0] = 0
	copy(msg[1:], js)
	return msg
}

type byTime []*Check

func (s byTime) Len() int      { return len(s) }
func (s byTime) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s byTime) Less(i, j int) bool {
	// Two zero times should return false.
	// Otherwise, zero is "greater" than any other time.
	// (To sort it at the end of the list.)
	if s[i].Next.IsZero() {
		return false
	}
	if s[j].Next.IsZero() {
		return true
	}
	return s[i].Next.Before(s[j].Next)
}

// WebHook represent a waiting webhook notification that hasn't been successfully executed.
type WebHook struct {
	ID string `json:"id"`
	URL string `json:"url"`
	Payload []byte `json:"payload"`
	Retries int `json:"retries"`
}

// NewWebHook initialize an empty WebHook.
func NewWebHook() *WebHook {
	return &WebHook{
		ID: uuid(),
	}
}

// ToPostCmd serializes a WebHook into a raft POST command
func (wh *WebHook) ToPostCmd() []byte {
	js, err := json.Marshal(wh)
	if err != nil {
		panic(err)
	}
	msg := make([]byte, len(js)+1)
	msg[0] = 2
	copy(msg[1:], js)
	return msg
}

// ToDeleteCmd serializes a WebHook into a raft delete command
func (wh *WebHook) ToDeleteCmd() []byte {
	buuid := []byte(wh.ID)
	msg := make([]byte, len(buuid)+1)
	msg[0] = 3
	copy(msg[1:], buuid)
	return msg
}
