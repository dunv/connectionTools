package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	cw "github.com/dunv/connectionTools/connectionWrappers"
	"github.com/go-redis/redis/v8"
)

func main() {

	redisConn := NewRedisConn()
	conn := cw.NewReadCallbackConn(
		redisConn,
		cw.WithConnectBackoff(time.Second, 10*time.Second, 2),
		cw.WithOnConnect(func() {
			fmt.Println("connected")
		}),
		cw.WithOnDisconnect(func() {
			fmt.Println("disconnected")
		}),
	)
	channel := conn.Run()

	for msg := range channel {
		fmt.Println("read", msg.(string))
	}

}

func NewRedisConn() *RedisConn {
	return &RedisConn{
		topic: "test",
		client: redis.NewClient(&redis.Options{
			Addr:     "localhost:6379",
			Password: "", // no password set
			DB:       0,  // use default DB
		}),
		mu: &sync.Mutex{},
	}
}

type RedisConn struct {
	topic  string
	client *redis.Client
	pubSub *redis.PubSub
	mu     *sync.Mutex
}

func (s *RedisConn) Connect(ctx context.Context) error {
	s.pubSub = s.client.Subscribe(ctx, s.topic)
	_, err := s.pubSub.Receive(ctx)
	return err
}

func (s *RedisConn) Read(ctx context.Context) (interface{}, error) {
	s.mu.Lock()
	msg, err := s.pubSub.ReceiveMessage(ctx)
	s.mu.Unlock()

	if err != nil {
		return nil, err
	}
	return msg.Payload, nil
}

func (s *RedisConn) Disconnect(ctx context.Context) error {
	return s.pubSub.Close()
}
