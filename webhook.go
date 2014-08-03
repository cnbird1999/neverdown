package neverdown

import (
	"log"
	"net/http"
	"encoding/json"
	"fmt"
	"bytes"
	"io/ioutil"
)

func ExecuteWebhooks(ra *Raft, check *Check) error {
	log.Println("ExecuteWebhooks")
	errc := make(chan error)
	for _, url := range check.WebHooks {
		go func() {
			errc<- ExecuteWebhook(ra, check, url)
		}()
	}
	for err := range errc {
		if err != nil {
			return err
		}
	}
	return nil
}

func ExecuteWebhook(ra *Raft, check *Check, url string) error {
	log.Printf("ExecuteWebhook %v: %v", check.ID, url)
	payload, err := json.Marshal(check)
	if err != nil {
		return err
	}
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
