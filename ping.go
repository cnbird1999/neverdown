package neverdown

import (
	"log"
	nurl "net/url"
	"net"
	"net/http"
	"encoding/json"
	"fmt"
	"strings"
	"io/ioutil"
	"time"
)

var client = &http.Client{
	Timeout: 10*time.Second,
}

type PingResponse struct {
	URL string `json:"url"`
	Up bool `json:"up"`
	Error struct {
		StatusCode int `json:"status_code"`
		Type string `json:"type"`
		Error string `json:"error"`
	} `json:"error"`
}

func PerformCheck(url string) (*PingResponse, error) {
	// TODO better check url//better response
	log.Printf("Checking %v...", url)
	pr := &PingResponse{
		URL: url,
	}
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(request)
	if err != nil {
		nerr, ok := err.(*nurl.Error)
		if ok {
			switch err := nerr.Err.(type) {
			case *net.OpError:
				switch err.Err.(type) {
				case *net.DNSError:
					pr.Error.Type = "dns"
					errs := strings.Split(err.Error(), ": ")
					pr.Error.Error = errs[len(errs)-1]
				default:
					pr.Error.Type = "unknow"
					pr.Error.Error = err.Error()
				}
			default:
				pr.Error.Type = "unknow"
				pr.Error.Error = err.Error()
			}
		} else {
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

func PerformAPICheck(peer, url string) (*PingResponse, error) {
	// TODO better check url//better response
	log.Printf("Calling %v...", url)
	pingResponse := &PingResponse{}
	request, err := http.NewRequest("GET", peer+"?url="+url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		return nil, fmt.Errorf("failed %v", resp)
	}
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(pingResponse); err != nil {
		return nil,  err
	}
	return pingResponse, nil
}

func LeaderCheck(ra *Raft, check *Check) {
	log.Printf("LeaderCheck %+v", check)
	pr, err := PerformCheck(check.URL)
	if err != nil {
		panic(err)
	}
	if pr.Up {
		check.Up = true
		return
	}
	prs := []*PingResponse{pr}
	for _, peer := range ra.PeersAPI() {
		ppr, err := PerformAPICheck(peer, check.URL)
		if err != nil {
			panic(err)
		}
		if ppr.Up {
			// TODO notify that the leader see the check as down
			return
		}
		prs = append(prs, ppr)
	}
	js, err := json.Marshal(prs)
	if err != nil {
		panic(err)
	}
	log.Printf("Webhook BODY: %v", string(js))
	check.Up = false
	check.LastDown = time.Now().UTC().Unix()
	// If all the responses are down, too, the website is definitely down
	// and we execute webhooks

	// POST request with list of ping reponse
	return
}


