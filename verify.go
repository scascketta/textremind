package main

import (
	"code.google.com/p/go.crypto/bcrypt"
	"github.com/garyburd/redigo/redis"
	"math/rand"
	"strconv"
)

func SetPassword(number, password string) error {
	c := GetConn()
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = c.Do("HSET", number, "password", string(hashed))
	return err
}

func CheckPassword(number, password string) (bool, error) {
	c := GetConn()
	hashed, err := redis.String(c.Do("HGET", number, "password"))
	if err != nil {
		return false, err
	}
	// CompareHashAndPassword() returns nil if passwords match
	matches := bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password)) == nil
	return matches, nil
}

func MakeVerificationCode(number string) (string, error) {
	code := ""
	for i := 0; i < 6; i++ {
		code += strconv.Itoa(rand.Intn(10))
	}
	c := GetConn()
	_, err := c.Do("HSET", number, "code", code)
	if err != nil {
		return "", err
	}
	return code, nil
}

func CheckVerificationCode(code, number string) (bool, error) {
	c := GetConn()
	actual_code, err := redis.String(c.Do("HGET", number, "code"))
	if err != nil {
		return false, err
	}
	return actual_code == code, nil
}

func MarkOnlyNumberVerified(number string) error {
	c := GetConn()
	_, err := c.Do("SADD", "only_number_verified", number)
	return err
}

func MarkNumberVerified(number string) error {
	c := GetConn()
	_, err := c.Do("SADD", "verified", number)
	return err
}

func CheckNumberVerified(number string) (bool, error) {
	c := GetConn()
	defer c.Close()

	verified, err := redis.Bool(c.Do("SISMEMBER", "verified", number))
	return verified, err
}

func CheckOnlyNumberVerified(number string) (bool, error) {
	c := GetConn()
	defer c.Close()

	verified, err := redis.Bool(c.Do("SISMEMBER", "only_number_verified", number))
	return verified, err
}
