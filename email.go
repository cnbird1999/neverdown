package neverdown

import (
	"bytes"
	"log"
	"text/template"

	"github.com/stathat/amzses"
)

// TODO handle sender email as config

var alertEmailSubjectTpl = `{{.URL}} is {{ if .Up }} up {{ else }} down {{ end }}`
var alertEmailBodyTpl = `{{.URL}} is {{ if .Up }} up {{ else }} down {{ end }}`

func NotifyEmails(c *Check) error {
	log.Printf("NotifyEmails %v", c)
	var body, subject bytes.Buffer
	t := template.New("alert mail body")
	t2 := template.New("alert mail subject")
	template.Must(t.Parse(alertEmailBodyTpl))
	template.Must(t2.Parse(alertEmailSubjectTpl))
	if err := t.Execute(&body, c); err != nil {
		panic(err)
	}
	if err := t2.Execute(&subject, c); err != nil {
		panic(err)
	}
	for _, email := range c.Emails {
		log.Printf("Sending mail to %v", email)
		if _, err := amzses.SendMail("thomas.sileo@gmail.com", email, subject.String(), body.String()); err != nil {
			return err
		}
	}
	return nil
}
