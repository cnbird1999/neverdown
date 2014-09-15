package neverdown

import (
	"bytes"
	"log"
	"text/template"

	"github.com/stathat/amzses"
)

func NotifyEmails(c *Check) error {
	log.Printf("NotifyEmails %v", c)
	var buf bytes.Buffer
	t := template.New("alert mail")
	template.Must(t.Parse("hello {{.URL}}!"))
	if err := t.Execute(&buf, c); err != nil {
		panic(err)
	}
	for _, email := range c.Emails {
		log.Printf("Sending mail to %v", email)
		if _, err := amzses.SendMail("thomas.sileo@gmail.com", email, "Welcome!", buf.String()); err != nil {
			return err
		}
	}
	return nil
}
