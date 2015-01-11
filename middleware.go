package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

func HTTPSRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, fmt.Sprintf("https://%s:%s%s", SERVER_ADDR, SERVER_PORT, r.RequestURI), http.StatusMovedPermanently)
}

// Adds `Access-Control-*` headers to response
func CorsMiddleware(fn func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST")
			w.Header().Set("Access-Control-Max-Age", strconv.Itoa(60*60*6))
			w.Header().Set("Access-Control-Allow-Headers", "CONTENT-TYPE, ACCEPT")
			// FIXME: for some reason, the `Access-Control-Request-Headers` never seems to exist in requests
			// if v, ok := r.Header["Access-Control-Request-Headers"]; ok {
			//  w.Header().Set("Access-Control-Allow-Headers", v[0])
			// }
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		fn(w, r)
	}
}

// Decodes JSON from request, passes decoded data to handler
func DecodeJSONMiddleware(fn func(http.ResponseWriter, *http.Request, map[string]string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := decodeJSON(w, r)
		if err != nil {
			return
		}
		fn(w, r, data)
	}
}

// Decodes JSON from request, returns decoded map or err if couldn't decode
// Limited to decoding strings only. Could return a map[string]interface{}
// and type assert in handlers, but not for this project
func decodeJSON(w http.ResponseWriter, r *http.Request) (map[string]string, error) {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	data := make(map[string]string)
	if err := dec.Decode(&data); err != nil {
		errlogger.Println(err)
		WriteJSONError(w, DECODE_ERR_S, http.StatusBadRequest)
		return nil, err
	}
	return data, nil
}

// Encode JSON data, sets content-type, return err if problem encoding data
// logs an error on its own to let caller use this function with less hassle
// when encoding responses
func WriteJSON(w http.ResponseWriter, data map[string]interface{}, code int) error {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	w.WriteHeader(code)
	if err := enc.Encode(&data); err != nil {
		errlogger.Println(err)
		return err
	}
	return nil
}

// Writes msg as JSON to request with given code
func WriteJSONError(w http.ResponseWriter, msg string, code int) {
	WriteJSON(w, map[string]interface{}{"message": msg}, code)
}
