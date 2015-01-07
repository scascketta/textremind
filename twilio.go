package main

import (
	"fmt"
	uuid "github.com/nu7hatch/gouuid"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
)

var (
	TWILIO_ACCOUNT_SID string = os.Getenv("TWILIO_ACCOUNT_SID")
	TWILIO_AUTH_TOKEN  string = os.Getenv("TWILIO_AUTH_TOKEN")
	TWILIO_NUMBER      string = os.Getenv("TWILIO_NUMBER")
	DEV_TWILIO_URL     string = fmt.Sprintf("http://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", TWILIO_ACCOUNT_SID)
	PROD_TWILIO_URL    string = fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", TWILIO_ACCOUNT_SID)
)

// Schedule a message to be sent to msg.To at msg.Time
func ScheduleMessage(body, to, time string) error {
	uid, _ := uuid.NewV4()
	id := uid.String()

	c := GetConn()
	defer c.Close()

	c.Send("MULTI")
	c.Send("ZADD", "messages", time, id)
	c.Send("HSET", id, "body", body)
	c.Send("HSET", id, "to", to)
	_, err := c.Do("EXEC")
	if err != nil {
		return nil
	}
	dbglogger.Printf("Message scheduled successfully for delivery at: %s", time)
	return nil
}

// Send a SMS using Twilio to phone number to, and given body.
func SendTwilioMessage(c *http.Client, to, body string) error {
	q := url.Values{}
	q.Set("From", TWILIO_NUMBER)
	q.Set("To", to)
	q.Set("Body", body)

	url := PROD_TWILIO_URL
	if ENV == "DEV" {
		url = DEV_TWILIO_URL
	}
	req, _ := http.NewRequest("POST", url, strings.NewReader(q.Encode()))
	req.SetBasicAuth(TWILIO_ACCOUNT_SID, TWILIO_AUTH_TOKEN)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	res, err := c.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode > 299 {
		errmsg, _ := ioutil.ReadAll(res.Body)
		return fmt.Errorf("SendTwilioMessage received statuscode %d, body: %s", res.StatusCode, errmsg)
	} else {
		dbglogger.Printf("Twilio msg sent, body: %s\n", body)
	}
	return nil
}
