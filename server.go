package main

import (
	"fmt"
	"github.com/garyburd/redigo/redis"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"
)

const (
	// Would make this an int to be proper, but it's only used as a string
	PORT = "8000"

	ERR_S                = "Something went wrong while "
	SCHEDULE_MSG_ERR_S   = ERR_S + "scheduling the message."
	VERIFY_ERR_S         = ERR_S + "checking if phone number is verified."
	SEND_VERIFY_ERR_S    = ERR_S + "sending verification code."
	CHECK_VERIFY_ERR_S   = ERR_S + "checking verification code."
	SET_PASSWORD_ERR_S   = ERR_S + "setting password."
	CHECK_PASSWORD_ERR_S = ERR_S + "checking password."
	DECODE_ERR_S         = ERR_S + "decoding request body."
)

var (
	dbglogger *log.Logger = log.New(os.Stdout, "[DBG] ", log.LstdFlags|log.Lshortfile)
	errlogger *log.Logger = log.New(os.Stderr, "[ERR] ", log.LstdFlags|log.Lshortfile)
)

func main() {
	// Seed PRNG for generating verification codes
	rand.Seed(time.Now().UTC().UnixNano())

	// Scheduled messages are dispatched in a new goroutine
	go DispatchMessages()

	http.HandleFunc("/schedule", CorsMiddleware(DecodeJSONMiddleware(schedule)))
	http.HandleFunc("/check", CorsMiddleware(check))
	http.HandleFunc("/send_verification", CorsMiddleware(DecodeJSONMiddleware(sendVerification)))
	http.HandleFunc("/check_verification", CorsMiddleware(checkVerification))
	http.HandleFunc("/set_password", CorsMiddleware(DecodeJSONMiddleware(setPassword)))

	dbglogger.Printf("Server listening on port %s...\n", PORT)
	http.ListenAndServe(":"+PORT, nil)
}

// Handle requests to schedule messages
func schedule(w http.ResponseWriter, r *http.Request, data map[string]string) {
	matches, err := CheckPassword(data["to"], []byte(data["password"]))
	if err != nil {
		errlogger.Println(err)
		WriteJSONError(w, SCHEDULE_MSG_ERR_S, http.StatusInternalServerError)
		return
	}
	if !matches {
		WriteJSONError(w, "Password doesn't match.", http.StatusBadRequest)
		return
	}

	err = ScheduleMessage(data["body"], data["to"], data["time"])
	if err != nil {
		errlogger.Println(err)
		WriteJSONError(w, SCHEDULE_MSG_ERR_S, http.StatusInternalServerError)
	}
}

func check(w http.ResponseWriter, r *http.Request) {
	v := r.URL.Query()
	number := v.Get("number")

	verified, err := CheckNumberVerified(number)
	if err != nil {
		errlogger.Println(err)
		WriteJSONError(w, VERIFY_ERR_S, http.StatusInternalServerError)
		return
	}

	WriteJSON(w, map[string]interface{}{"verified": verified}, http.StatusOK)
}

func sendVerification(w http.ResponseWriter, r *http.Request, data map[string]string) {
	code, err := MakeVerificationCode(data["number"])
	if err != nil {
		errlogger.Println(err)
		WriteJSONError(w, SEND_VERIFY_ERR_S, http.StatusInternalServerError)
		return
	}

	err = SendTwilioMessage(data["number"], fmt.Sprintf("Your verification code for TextRemind is %s.", code))
	if err != nil {
		errlogger.Println(err)
		WriteJSONError(w, SEND_VERIFY_ERR_S, http.StatusInternalServerError)
	}
}

func checkVerification(w http.ResponseWriter, r *http.Request) {
	v := r.URL.Query()
	code := v.Get("code")
	number := v.Get("number")

	valid, err := CheckVerificationCode(code, number)
	if err != nil {
		errlogger.Println(err)
		WriteJSONError(w, CHECK_VERIFY_ERR_S, http.StatusInternalServerError)
		return
	}

	if valid {
		MarkOnlyNumberVerified(number)
	}

	WriteJSON(w, map[string]interface{}{"valid": valid}, http.StatusOK)
}

func setPassword(w http.ResponseWriter, r *http.Request, data map[string]string) {
	only_number_verified, err := CheckOnlyNumberVerified(data["number"])
	if only_number_verified {
		err = SetPassword(data["number"], []byte(data["password"]))
		if err != nil {
			errlogger.Println(err)
			WriteJSONError(w, SET_PASSWORD_ERR_S, http.StatusInternalServerError)
		}
		MarkNumberVerified(data["number"])
	} else {
		WriteJSONError(w, "This number has not been verified.", http.StatusBadRequest)
	}
}

// Get connection to local redis server or exit
func GetConn() redis.Conn {
	c, err := redis.Dial("tcp", ":6379")
	if err != nil {
		errlogger.Fatal("Error connecting to redis server: ", err)
	}
	return c
}
