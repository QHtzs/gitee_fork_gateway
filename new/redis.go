package main

/*
建立redis池，并封装部分redis操作
*/

import (
	"fmt"
	"strings"
	"time"

	"github.com/garyburd/redigo/redis"
)

func build_redis_client() *redis.Pool {
	redisClient := new(redis.Pool)
	*redisClient = redis.Pool{
		MaxIdle:     ConfigInstance.RedisCfg.MaxIdle,
		MaxActive:   ConfigInstance.RedisCfg.MaxActive,
		IdleTimeout: time.Duration(ConfigInstance.RedisCfg.TimeoutSec) * time.Second,
		Wait:        ConfigInstance.RedisCfg.Wait,
		Dial: func() (redis.Conn, error) {
			c, err := redis.DialURL(ConfigInstance.RedisCfg.RedisUrl)
			if err != nil {
				return nil, fmt.Errorf("redis connection error: %s", err)
			}
			passwd := strings.Trim(ConfigInstance.RedisCfg.PassWord, " ")
			if passwd != "" {
				if _, authErr := c.Do("AUTH", passwd); authErr != nil {
					return nil, fmt.Errorf("redis auth password error: %s", authErr)
				}
			}
			return c, err
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			if err != nil {
				return fmt.Errorf("ping redis error: %s", err)
			}
			return nil
		},
	}
	return redisClient
}

//创建全局redis池
var redisClinet *redis.Pool = build_redis_client()

//封装HSET
func HSet(name, key, value string) error {
	con := redisClinet.Get()
	defer con.Close()
	_, err := con.Do("HSET", name, key, value)
	return err
}

//封装HGET
func HGet(name, key string) (string, error) {
	con := redisClinet.Get()
	defer con.Close()
	val, err := redis.String(con.Do("HGET", name, key))
	return val, err
}
