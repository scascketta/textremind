package main

import (
	"fmt"
	"github.com/garyburd/redigo/redis"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
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

	ENV_VARS    []string = []string{"TWILIO_ACCOUNT_SID", "TWILIO_AUTH_TOKEN", "TWILIO_NUMBER", "TEXTREMIND_ENV", "TEXTREMIND_ADDR", "TEXTREMIND_PORT"}
	HTTP_CLIENT *Client  = &Client{URL: TWILIO_URL, HTTPClient: &http.Client{}}
	ENV         string   = os.Getenv("TEXTREMIND_ENV")
	SERVER_ADDR string   = os.Getenv("TEXTREMIND_ADDR")
	SERVER_PORT string   = os.Getenv("TEXTREMIND_PORT")
)

func main() {
	checkRequiredEnvVars(ENV_VARS)

	// Seed PRNG for generating verification codes
	rand.Seed(time.Now().UTC().UnixNano())

	// Scheduled messages are dispatched in a new goroutine
	go DispatchMessages()

	http.HandleFunc("/schedule", CorsMiddleware(DecodeJSONMiddleware(schedule)))
	http.HandleFunc("/check", CorsMiddleware(check))
	http.HandleFunc("/send_verification", CorsMiddleware(DecodeJSONMiddleware(sendVerification)))
	http.HandleFunc("/check_verification", CorsMiddleware(checkVerification))
	http.HandleFunc("/set_password", CorsMiddleware(DecodeJSONMiddleware(setPassword)))
	http.Handle("/", http.FileServer(http.Dir("static/")))

	startServer()
}

func checkRequiredEnvVars(env_vars []string) {
	missing := make([]string, 0)

	for _, ev := range env_vars {
		if os.Getenv(ev) == "" {
			missing = append(missing, ev)
		}
	}

	if len(missing) > 0 {
		missing_msg := strings.Join(missing, "\n")
		errlogger.Fatalf("The following environment variables are missing:\n%s", missing_msg)
	}
}

func startServer() {
	if ENV == "DEV" {
		dbglogger.Printf("HTTP server listening on %s:%s\n", SERVER_ADDR, SERVER_PORT)
		err := http.ListenAndServe(SERVER_ADDR+":"+SERVER_PORT, nil)
		if err != nil {
			errlogger.Fatal("Problem starting HTTP server:", err)
		}
	} else {

		// Start HTTPS and HTTP server, HTTP server redirects to HTTPS server.
		go func() {
			dbglogger.Printf("HTTPS server listening on %s:443\n", SERVER_ADDR)
			err := http.ListenAndServeTLS(SERVER_ADDR+":443", "textremind.net.cert", "textremind.net.key", nil)
			if err != nil {
				errlogger.Fatal("Problem starting HTTPS server:", err)
			}
		}()

		dbglogger.Printf("HTTP server listening on %s:80\n", SERVER_ADDR)
		if err := http.ListenAndServe(SERVER_ADDR+":80", http.HandlerFunc(HTTPSRedirect)); err != nil {
			errlogger.Fatal("Problem starting HTTP server:", err)
		}
	}
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

	err = SendTwilioMessage(HTTP_CLIENT, data["number"], fmt.Sprintf("Your verification code for TextRemind is %s.", code))
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
