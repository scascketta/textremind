package main

import (
	"fmt"
	"net/http"
	"testing"
)

func TestBadRequest(t *testing.T) {
	sc := 400
	rb := `{"code": 20003, "detail": "Your AccountSid or AuthToken was incorrect.", "message": "Authentication Error - No credentials provided", "more_info": "https://www.twilio.com/docs/errors/20003", "status": 401}`
	c, server := MockClient(sc, []byte(rb), map[string]string{"Content-Type": "application/json"})
	defer server.Close()
	err := SendTwilioMessage(c, "5558675309", "asdf")

	if err != nil && err.Error() != fmt.Sprintf("SendTwilioMessage received statuscode %d, body: %s", sc, rb) {
		t.Error(err)
	}
}

func TestInternalError(t *testing.T) {
	rb := "internal error"
	sc := 500
	c, server := MockClient(sc, []byte(rb), map[string]string{"Content-Type": "application/json"})
	defer server.Close()
	err := SendTwilioMessage(c, "5558675309", "asdf")

	if err != nil && err.Error() != fmt.Sprintf("SendTwilioMessage received statuscode %d, body: %s", sc, rb) {
		t.Error(err)
	}
}

func TestOK(t *testing.T) {
	rb := "success"
	sc := 200
	c, server := MockClient(sc, []byte(rb), map[string]string{"Content-Type": "application/json"})
	defer server.Close()
	err := SendTwilioMessage(c, "5558675309", "asdf")

	if err != nil {
		t.Error(err)
	}
}

func TestRequestDetails(t *testing.T) {
	c, server := MockClientHandler(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			ErrorWithCode(t, w, "Request method is not POST", http.StatusBadRequest)
		}
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			ErrorWithCode(t, w, "Content type is not application/x-www-form-urlencoded", http.StatusBadRequest)
		}
		err := r.ParseForm()
		if err != nil {
			ErrorWithCode(t, w, err.Error(), http.StatusBadRequest)
		}
		params := []string{"To", "Body", "From"}
		for _, p := range params {
			if r.PostForm.Get(p) == "" {
				ErrorWithCode(t, w, fmt.Sprintf("Param %s not present in request body.", p), http.StatusBadRequest)
			}
		}
		w.WriteHeader(200)
	})
	defer server.Close()
	err := SendTwilioMessage(c, "5558675309", "asdf")
	if err != nil {
		t.Error(err)
	}
}
