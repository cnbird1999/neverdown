package monitoring

import (
	"fmt"
	"crypto/rand"
	"encoding/json"
	"sync"
)

func uuid() string {
    b := make([]byte, 16)
    rand.Read(b)
    b[6] = (b[6] & 0x0f) | 0x40
    b[8] = (b[8] & 0x3f) | 0x80
    return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// Store holds active checks, and pending webhook notification
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

// FromJSON loads the store from a JSON export.
func (s *Store) FromJSON(js []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data := map[string]interface{}{}
	if err := json.Unmarshal(js, &data); err != nil {
		return err
	}
	for _, c := range data["checks"].([]Check) {
		s.ChecksIndex[c.ID] = &c
	}
	for _, wh := range data["pending_webhooks"].([]WebHook) {
		s.PendingWebHooksIndex[wh.ID] = &wh
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
		s.ChecksIndex[check.ID] = check
	default:
		panic("unknow cmd type")
	}
	return nil
}

// Check represent an active monitoring check
type Check struct {
	ID string `json:"id"`
	URL string `json:"created_at"`
	LastCheck int64 `json:"created_at"`
	LastDown int64 `json:"last_down"`
	Interval int `json:"interval"`
	WebHooks []string `json:"webhooks"`
}

// NewCheck initialize an empty Check, generates an ID.
func NewCheck() *Check {
	return &Check{
		ID: uuid(),
	}
}

// WebHook represent a waiting webhook notification
type WebHook struct {
	ID string `json:"id"`
	Retries int `json:"retries"`
}

// NewWebHook initialize an empty WebHook, generates an ID;
func NewWebHook() *WebHook {
	return &WebHook{
		ID: uuid(),
	}
}
