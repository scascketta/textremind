package main

import (
	"encoding/json"
	"fmt"
	"github.com/garyburd/redigo/redis"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"
)

const (
	PORT int = 8000
)

var (
	// store the config vars in some local file (like `.env`) and to set them use:
	// `export $(cat .env | xargs)`
	TWILIO_ACCOUNT_SID string = os.Getenv("TWILIO_ACCOUNT_SID")
	TWILIO_AUTH_TOKEN  string = os.Getenv("TWILIO_AUTH_TOKEN")
	TWILIO_NUMBER      string = os.Getenv("TWILIO_NUMBER")
	TWILIO_BASE_URL    string = fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", TWILIO_ACCOUNT_SID)

	// format be like: `date time file:line:`
	dbglogger *log.Logger = log.New(os.Stdout, "[DBG] ", log.LstdFlags|log.Lshortfile)
	errlogger *log.Logger = log.New(os.Stderr, "[ERR] ", log.LstdFlags|log.Lshortfile)
)

func main() {
	rand.Seed(time.Now().UTC().UnixNano())
	// Scheduled messages are dispatched in a new goroutine
	go DispatchMessages()

	// middlewares add necessary headers (for CORS, content-type)
	middlewares := []Middleware{jsonMiddleware, crossOriginMiddleware}

	http.HandleFunc("/schedule", wrapMiddlewares(middlewares, scheduleHandler))
	http.HandleFunc("/check", wrapMiddlewares(middlewares, checkHandler))
	http.HandleFunc("/send_verification", wrapMiddlewares(middlewares, sendVerificationHandler))
	http.HandleFunc("/check_verification", wrapMiddlewares(middlewares, checkVerificationHandler))
	http.HandleFunc("/set_password", wrapMiddlewares(middlewares, setPasswordHandler))
	http.HandleFunc("/check_password", wrapMiddlewares(middlewares, checkPasswordHandler))

	dbglogger.Printf("Server listening on port %d...\n", PORT)
	http.ListenAndServe(":"+strconv.Itoa(PORT), nil)
}

// Middleware represents a function that wraps an http.HandlerFunc
type Middleware func(func(http.ResponseWriter, *http.Request)) http.HandlerFunc

// Wrap and return handler with wrappers in middlewares
func wrapMiddlewares(middlewares []Middleware, handler func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	wrapped := handler
	for _, middleware := range middlewares {
		wrapped = middleware(wrapped)
	}
	return wrapped
}

// Adds `application/json` content-type header to response
func jsonMiddleware(fn func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fn(w, r)
	}
}

// Adds `Access-Control-*` headers to response
func crossOriginMiddleware(fn func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST")
			w.Header().Set("Access-Control-Max-Age", strconv.Itoa(60*60*6))
			w.Header().Set("Access-Control-Allow-Headers", "CONTENT-TYPE, ACCEPT")
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		fn(w, r)
	}
}

// Write msg in JSON to response with an HTTP code
func writeJSON(w http.ResponseWriter, msg map[string]string, code int) {
	w.WriteHeader(code)
	b, _ := json.Marshal(msg)
	w.Write(b)
}

type ScheduleMsgStruct struct {
	Body string `json:"body"`
	To   string `json:"to"`
	Time int    `json:"time"`
}

// Handle requests to schedule messages
func scheduleHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	data, _ := ioutil.ReadAll(r.Body)
	msg := ScheduleMsgStruct{}
	err := json.Unmarshal([]byte(data), &msg)
	if err != nil {
		errlogger.Println(err)
	}
	err = ScheduleMessage(msg)
	if err != nil {
		errlogger.Println(err)
		writeJSON(w, map[string]string{"type": "api_error", "message": "Something went wrong while scheduling the message."}, http.StatusInternalServerError)
	}
}

func checkHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	args := make(map[string]string)
	if err := dec.Decode(&args); err != nil {
		errlogger.Println(err)
		writeJSON(w, map[string]string{"type": "api_error", "message": "Something went wrong while checking if phone number is verified."}, http.StatusInternalServerError)
		return
	}

	// TODO: also mention if passsword created for number if the number has been verified
	verified, err := CheckNumberVerified(args["number"])
	if err != nil {
		errlogger.Println(err)
		writeJSON(w, map[string]string{"type": "api_error", "message": "Something went wrong while checking if phone number is verified."}, http.StatusInternalServerError)
		return
	}
	enc := json.NewEncoder(w)
	if err = enc.Encode(&map[string]bool{"verified": verified}); err != nil {
		errlogger.Println(err)
		writeJSON(w, map[string]string{"type": "api_error", "message": "Something went wrong while checking if phone number is verified."}, http.StatusInternalServerError)
	}
}

func sendVerificationHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	args := make(map[string]string)
	if err := dec.Decode(&args); err != nil {
		errlogger.Println(err)
		writeJSON(w, map[string]string{"type": "api_error", "message": "Something went wrong while sending verification code."}, http.StatusInternalServerError)
		return
	}

	code, err := MakeVerificationCode(args["number"])
	if err != nil {
		errlogger.Println(err)
		writeJSON(w, map[string]string{"type": "api_error", "message": "Something went wrong while sending verification code."}, http.StatusInternalServerError)
		return
	}

	err = SendTwilioMessage(args["number"], fmt.Sprintf("Your verification code for TextRemind is %s.", code))
	if err != nil {
		errlogger.Println(err)
		writeJSON(w, map[string]string{"type": "api_error", "message": "Something went wrong while sending verification code."}, http.StatusInternalServerError)
	}
}

func checkVerificationHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	args := make(map[string]string)
	if err := dec.Decode(&args); err != nil {
		errlogger.Println(err)
		writeJSON(w, map[string]string{"type": "api_error", "message": "Something went wrong while checking verification code."}, http.StatusInternalServerError)
		return
	}

	valid, err := CheckVerificationCode(args["code"], args["number"])
	if err != nil {
		errlogger.Println(err)
		writeJSON(w, map[string]string{"type": "api_error", "message": "Something went wrong while checking verification code."}, http.StatusInternalServerError)
		return
	}

	if valid {
		MarkOnlyNumberVerified(args["number"])
	}

	enc := json.NewEncoder(w)
	if err = enc.Encode(&map[string]bool{"valid": valid}); err != nil {
		errlogger.Println(err)
		writeJSON(w, map[string]string{"type": "api_error", "message": "Something went wrong while checking verification code."}, http.StatusInternalServerError)
	}
}

func setPasswordHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	args := make(map[string]string)
	if err := dec.Decode(&args); err != nil {
		errlogger.Println(err)
		writeJSON(w, map[string]string{"type": "api_error", "message": "Something went wrong while setting password."}, http.StatusInternalServerError)
		return
	}

	only_number_verified, err := CheckOnlyNumberVerified(args["number"])
	if only_number_verified {
		err = SetPassword(args["number"], args["password"])
		MarkNumberVerified(args["number"])
		if err != nil {
			errlogger.Println(err)
			writeJSON(w, map[string]string{"type": "api_error", "message": "Something went wrong while setting password."}, http.StatusInternalServerError)
		}
	} else {
		writeJSON(w, map[string]string{"type": "invalid_request", "message": "This number has not been verified."}, http.StatusBadRequest)
	}
}

func checkPasswordHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	args := make(map[string]string)
	if err := dec.Decode(&args); err != nil {
		errlogger.Println(err)
		writeJSON(w, map[string]string{"type": "api_error", "message": "Something went wrong while checking password."}, http.StatusInternalServerError)
		return
	}

	verified, err := CheckNumberVerified(args["number"])
	if err != nil {
		errlogger.Println(err)
		http.Error(w, "Something went wrong while setting password.", http.StatusInternalServerError)
	}

	if verified {
		matches, err := CheckPassword(args["number"], args["password"])
		if err != nil {
			errlogger.Println(err)
			writeJSON(w, map[string]string{"type": "invalid_request", "message": "Something went wrong while checking password."}, http.StatusBadRequest)
		}

		enc := json.NewEncoder(w)
		if err = enc.Encode(&map[string]bool{"matches": matches}); err != nil {
			errlogger.Println(err)
			writeJSON(w, map[string]string{"type": "api_error", "message": "Something went wrong while checking verification code."}, http.StatusInternalServerError)
		}
	} else {
		writeJSON(w, map[string]string{"type": "invalid_request", "message": "This number has not been verified."}, http.StatusBadRequest)
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
