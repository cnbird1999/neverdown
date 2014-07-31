package monitoring

import (
	"log"
	"net/http"
	"encoding/json"
	"fmt"
	"io/ioutil"
)

var client = &http.Client{}

type PingResponse struct {
	StatusCode int `json:"status_code"`
	Error error `json:"error"`
	Success bool `json:"success"`
}

func PerformCheck(url string) *PingResponse {
	// TODO better check url//better response
	log.Printf("Checking %v...", url)
	pingResp := &PingResponse{}
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		pingResp.Error = err
		return pingResp
	}
	resp, err := client.Do(request)
	if err != nil {
		pingResp.Error = err
		return pingResp
	}
	defer resp.Body.Close()
	pingResp.StatusCode = resp.StatusCode
	if resp.StatusCode == 200 {
		pingResp.Success = true
	} else {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			panic(err)
		}
		pingResp.Error = fmt.Errorf("%v", string(body))
	}
	return pingResp
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
}