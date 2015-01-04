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

	http.HandleFunc("/schedule", corsMiddleware(decodeJSONMiddleware(scheduleHandler)))
	http.HandleFunc("/check", corsMiddleware(decodeJSONMiddleware(checkHandler)))
	http.HandleFunc("/send_verification", corsMiddleware(decodeJSONMiddleware(sendVerificationHandler)))
	http.HandleFunc("/check_verification", corsMiddleware(decodeJSONMiddleware(checkVerificationHandler)))
	http.HandleFunc("/set_password", corsMiddleware(decodeJSONMiddleware(setPasswordHandler)))
	http.HandleFunc("/check_password", corsMiddleware(decodeJSONMiddleware(checkPasswordHandler)))

	dbglogger.Printf("Server listening on port %s...\n", PORT)
	http.ListenAndServe(":"+PORT, nil)
}

// Handle requests to schedule messages
func scheduleHandler(w http.ResponseWriter, r *http.Request, data map[string]string) {
	matches, err := CheckPassword(data["to"], data["password"])
	if err != nil {
		errlogger.Println(err)
		writeJSONError(w, SCHEDULE_MSG_ERR_S, http.StatusInternalServerError)
		return
	}
	if !matches {
		writeJSONError(w, "Password doesn't match.", http.StatusBadRequest)
		return
	}

	err = ScheduleMessage(data["body"], data["to"], data["time"])
	if err != nil {
		errlogger.Println(err)
		writeJSONError(w, SCHEDULE_MSG_ERR_S, http.StatusInternalServerError)
	}
}

func checkHandler(w http.ResponseWriter, r *http.Request, data map[string]string) {
	verified, err := CheckNumberVerified(data["number"])
	if err != nil {
		errlogger.Println(err)
		writeJSONError(w, VERIFY_ERR_S, http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]interface{}{"verified": verified}, http.StatusOK)
}

func sendVerificationHandler(w http.ResponseWriter, r *http.Request, data map[string]string) {
	code, err := MakeVerificationCode(data["number"])
	if err != nil {
		errlogger.Println(err)
		writeJSONError(w, SEND_VERIFY_ERR_S, http.StatusInternalServerError)
		return
	}

	err = SendTwilioMessage(data["number"], fmt.Sprintf("Your verification code for TextRemind is %s.", code))
	if err != nil {
		errlogger.Println(err)
		writeJSONError(w, SEND_VERIFY_ERR_S, http.StatusInternalServerError)
	}
}

func checkVerificationHandler(w http.ResponseWriter, r *http.Request, data map[string]string) {
	valid, err := CheckVerificationCode(data["code"], data["number"])
	if err != nil {
		errlogger.Println(err)
		writeJSONError(w, CHECK_VERIFY_ERR_S, http.StatusInternalServerError)
		return
	}

	if valid {
		MarkOnlyNumberVerified(data["number"])
	}

	writeJSON(w, map[string]interface{}{"valid": valid}, http.StatusOK)
}

func setPasswordHandler(w http.ResponseWriter, r *http.Request, data map[string]string) {
	only_number_verified, err := CheckOnlyNumberVerified(data["number"])
	if only_number_verified {
		err = SetPassword(data["number"], data["password"])
		if err != nil {
			errlogger.Println(err)
			writeJSONError(w, SET_PASSWORD_ERR_S, http.StatusInternalServerError)
		}
		MarkNumberVerified(data["number"])
	} else {
		writeJSONError(w, "This number has not been verified.", http.StatusBadRequest)
	}
}

func checkPasswordHandler(w http.ResponseWriter, r *http.Request, data map[string]string) {
	verified, err := CheckNumberVerified(data["number"])
	if err != nil {
		errlogger.Println(err)
		writeJSONError(w, CHECK_PASSWORD_ERR_S, http.StatusBadRequest)
		return
	}

	if verified {
		matches, err := CheckPassword(data["number"], data["password"])
		if err != nil {
			errlogger.Println(err)
			writeJSONError(w, CHECK_PASSWORD_ERR_S, http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]interface{}{"matches": matches}, http.StatusOK)
	} else {
		writeJSONError(w, "This number hasn't been verified.", http.StatusBadRequest)
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
