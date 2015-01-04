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
	TWILIO_BASE_URL    string = fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", TWILIO_ACCOUNT_SID)
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
func SendTwilioMessage(to, body string) error {
	payload := url.Values{}
	payload.Set("From", TWILIO_NUMBER)
	payload.Set("To", to)
	payload.Set("Body", body)

	req, _ := http.NewRequest("POST", TWILIO_BASE_URL, strings.NewReader(payload.Encode()))
	req.SetBasicAuth(TWILIO_ACCOUNT_SID, TWILIO_AUTH_TOKEN)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	res, err := client.Do(req)
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
