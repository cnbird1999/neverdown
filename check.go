package neverdown

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	nurl "net/url"
	"strings"
	"time"
)

var client = &http.Client{
	Timeout: 10 * time.Second,
}

// PingResponse is the results of a check PING
type PingResponse struct {
	URL   string `json:"url"`
	Up    bool   `json:"up"`
	Error struct {
		StatusCode int    `json:"status_code"`
		Type       string `json:"type"`
		Error      string `json:"error"`
	} `json:"error"`
}

func LogUnknownError(lvl string, err, baseErr error) {
	log.Printf(`INFO: unknown error at lvl %v "%+v", (base:%+v),
Please open an issue in the GitHub repository at https://github.com/tsileo/neverdown.`, lvl, err, baseErr)
}

// PerformCheck execute the check request and returns a PingResponse.
func PerformCheck(method, url string) (*PingResponse, error) {
	// TODO better check url//better response
	log.Printf("Checking %v...", url)
	pr := &PingResponse{
		URL: url,
	}
	request, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(request)
	if err != nil {
		nerr, ok := err.(*nurl.Error)
		if ok {
			switch cerr := nerr.Err.(type) {
			case *net.OpError:
				switch cerr.Err.(type) {
				case *net.DNSError:
					pr.Error.Type = "dns"
					errs := strings.Split(cerr.Error(), ": ")
					pr.Error.Error = errs[len(errs)-1]
				default:
					LogUnknownError("2", cerr, nerr)
					errs := strings.Split(cerr.Error(), ": ")
					pr.Error.Error = errs[len(errs)-1]
					pr.Error.Type = "server"
				}
			default:
				switch nerr.Err.Error() {
				case "net/http: request canceled while waiting for connection":
					pr.Error.Type = "timeout"
					pr.Error.Error = "timeout exceeded"
				default:
					LogUnknownError("1", cerr, nerr)
					pr.Error.Type = "unknown"
					errs := strings.Split(cerr.Error(), ": ")
					pr.Error.Error = errs[len(errs)-1]
				}
			}
		} else {
			LogUnknownError("0", err, nil)
			pr.Error.Type = "unknown"
			pr.Error.Error = err.Error()
		}
		return pr, nil
	}
	defer resp.Body.Close()
	pr.Error.StatusCode = resp.StatusCode
	if resp.StatusCode == 200 {
		pr.Up = true
	} else {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return pr, nil
		}
		pr.Error.Type = "response"
		pr.Error.Error = string(body)
	}
	return pr, nil
}

// PerformAPICheck query the ping api of the given remote peer for the given URL.
func PerformAPICheck(peer, method, url string) (*PingResponse, error) {
	log.Printf("Calling remote peer %v for confirmation on %v...", peer, url)
	pingResponse := &PingResponse{}
	request, err := http.NewRequest("GET", "http://"+peer+"/_ping?method="+method+"&url="+url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("ping request failed %v", resp)
	}
	if err := json.NewDecoder(resp.Body).Decode(pingResponse); err != nil {
		return nil, err
	}
	return pingResponse, nil
}

// LeaderCheck is the check function called by the raft leader,
// if the website is down for the leader , it will ask followers for confirmation,
// if the website is down for one of the follower, a warning is emitted but the website isn't
// declared down.
func LeaderCheck(ra *Raft, check *Check) error {
	log.Printf("Checking %v (up:%v/prev:%v)", check.URL, check.Up, check.Prev)
	pr, err := PerformCheck(check.Method, check.URL)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Unix()
	log.Printf("Check result: %+v", pr)
	if check.FirstCheck == 0 {
		check.FirstCheck = check.Next.Unix()
	}
	check.Pings++
	if pr.Up {
		check.Up = true
		return nil
	}
	// If all the responses are down, too, the website is definitely down
	// and we execute webhooks
	prs := []*PingResponse{pr}
	for _, peer := range ra.PeersAPI() {
		ppr, err := PerformAPICheck(peer, check.Method, check.URL)
		if err != nil {
			log.Printf("WARNING: failed to ask confirmation from remote peer %v: %v", peer, err)
			continue
		}
		if ppr.Up {
			log.Printf("WARNING: leader flagged the check as \"down\", but others peers found it \"up\": %+v", pr)
			return nil
		}
		prs = append(prs, ppr)
	}
	if check.Up == true {
		check.Outages++
	}
	check.TimeDown += int64(check.Interval)
	check.Up = false
	check.LastDown = now
	check.LastError = pr.Error
	// POST request with list of ping reponse
	return nil
}
