package main

import (
	"log"

	"github.com/go-redis/redis"
	gochat "github.com/laoqiu/go-chat"
	proto "github.com/laoqiu/go-chat/proto"
	micro "github.com/micro/go-micro"
)

func main() {
	redisOpts := &redis.Options{}

	// create a service
	service := micro.NewService(
		micro.Name("go.micro.srv.chat"),
	)
	// parse command line
	service.Init()

	db := gochat.NewRedis("chat", redisOpts)

	pub := micro.NewPublisher("example.topic.pubsub.1", service.Client())

	hub := gochat.NewHub()
	go hub.Run()

	proto.RegisterChatHandler(service.Server(), gochat.NewHandler(db, hub, pub))

	micro.RegisterSubscriber("example.topic.pubsub.1", service.Server(), gochat.NewSubscriber(hub))

	if err := service.Run(); err != nil {
		log.Fatal(err)
	}
}
