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

func LogUnknowError(lvl string, err, baseErr error) {
	log.Printf(`INFO: unknow error at lvl %v "%+v", (base:%+v),
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
					LogUnknowError("2", cerr, nerr)
					pr.Error.Type = "timeout"
					pr.Error.Error = err.Error()
				}
			default:
				switch nerr.Err.Error() {
				case "net/http: request canceled while waiting for connection":
					pr.Error.Type = "timeout"
					pr.Error.Error = "timeout"
				default:
					LogUnknowError("1", cerr, nerr)
					pr.Error.Type = "unknow"
					errs := strings.Split(cerr.Error(), ": ")
					pr.Error.Error = errs[len(errs)-1]
				}
			}
		} else {
			LogUnknowError("0", err, nil)
			pr.Error.Type = "unknow"
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
		pr.Error.Type = "server"
		pr.Error.Error = string(body)
	}
	return pr, nil
}

// PerformAPICheck query the ping api of the given remote peer for the given URL.
func PerformAPICheck(peer, method, url string) (*PingResponse, error) {
	log.Printf("Calling %v...", url)
	pingResponse := &PingResponse{}
	request, err := http.NewRequest("GET", peer+"?method="+method+"&url="+url, nil)
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
	log.Printf("Check result: %+v", pr)
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
			return err
		}
		if ppr.Up {
			log.Printf("WARNING: leader flagged the check as \"down\", but others peers found it \"up\": %+v", pr)
			return nil
		}
		prs = append(prs, ppr)
	}
	check.Up = false
	check.LastDown = time.Now().UTC().Unix()
	check.LastError = pr.Error
	// POST request with list of ping reponse
	return nil
}
