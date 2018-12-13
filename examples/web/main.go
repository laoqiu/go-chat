package main

import (
	"log"
	"time"

	gochat "github.com/laoqiu/go-chat"
	proto "github.com/laoqiu/go-chat/proto"
	"github.com/micro/go-micro/client"
	web "github.com/micro/go-web"
)

func main() {

	service := web.NewService(
		web.Name("go.micro.web.chat"),
		web.Version("latest"),
	)

	if err := service.Init(); err != nil {
		log.Fatal("Init", err)
	}

	rpcClient := client.NewClient(client.RequestTimeout(time.Second * 120))
	cli := proto.NewChatService("go.micro.srv.chat", rpcClient)

	// register chat handler
	service.HandleFunc("/stream", gochat.NewWebsocketHandler(cli))

	// run service
	if err := service.Run(); err != nil {
		log.Fatal(err)
	}
}
