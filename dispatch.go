package main

import (
	"github.com/garyburd/redigo/redis"
	"strconv"
	"time"
)

// Dispatches scheduled messages at the start of every minute
func DispatchMessages() {
	c := GetConn()
	defer c.Close()

	dbglogger.Printf("Message dispatch goroutine running...")
	ticker := nextMinuteTicker()
	for {
		<-ticker.C
		// FIXME: cleaner way to do Time to unix_time string
		now := strconv.Itoa(int(time.Now().Unix()))

		// get messages that must be dispatched now
		uids, err := redis.Strings(c.Do("ZRANGEBYSCORE", "messages", 0, now))
		if err != nil {
			errlogger.Println(err)
		}

		for _, uid := range uids {
			body, _ := redis.String(c.Do("HGET", uid, "body"))
			to, _ := redis.String(c.Do("HGET", uid, "to"))
			err := SendTwilioMessage(HTTP_CLIENT, to, body)
			if err != nil {
				errlogger.Println(err)
				continue
			}
			c.Send("MULTI")
			c.Send("ZREM", "messages", uid)
			c.Send("HDEL", uid, "body")
			c.Send("HDEL", uid, "to")
			_, err = c.Do("EXEC")
			if err != nil {
				errlogger.Println(err)
			}
		}
		ticker = nextMinuteTicker()
	}
}

// Get a *time.Ticker which ticks at start of the next minute from now
func nextMinuteTicker() *time.Ticker {
	now := time.Now()
	nextTick := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute()+1, 0, 0, time.Local)
	return time.NewTicker(nextTick.Sub(now))
}
