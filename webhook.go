package neverdown

import (
	"log"
	"net/http"
	neurl "net/url"
	"encoding/json"
	"fmt"
	"bytes"
	"strings"
	"io/ioutil"
)

//func ExecuteWebhooks(ra *Raft, check *Check) error {
//}

func ExecuteWebhook(ra *Raft, check *Check, url string) error {
	log.Printf("ExecuteWebhook %v: %v", check.ID, url)
	var req *http.Request
	payload, err := json.Marshal(check)
	if err != nil {
		return err
	}
	if strings.HasPrefix(url, "-") {
		nurl, err := neurl.Parse(url[1:])
		if err != nil {
			return err
		}
		if nurl.RawQuery == "" {
			nurl.RawQuery = "?check="+string(payload)
		} else {
			nurl.RawQuery += "&check="+string(payload)
		}
		greq, err := http.NewRequest("GET", nurl.String(), nil)
		if err != nil {
			return err
		}
		req = greq
	} else {
		var body *bytes.Buffer
		body.Write(payload)
		preq, err := http.NewRequest("POST", url, body)
		if err != nil {
			return err
		}
		req = preq
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
