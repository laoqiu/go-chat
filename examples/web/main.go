package main

import (
	"log"
	"regexp"

	gochat "github.com/laoqiu/go-chat"
	proto "github.com/laoqiu/go-chat/proto"
	config "github.com/micro/go-config"
	"github.com/micro/go-micro/client"
	web "github.com/micro/go-web"
)

func main() {

	// service
	service := web.NewService(
		web.Name("go.micro.web.chat"),
		web.Version("0.2"),
	)

	if err := service.Init(); err != nil {
		log.Fatal("Init", err)
	}

	// 加载配置
	if err := config.LoadFile("config.json"); err != nil {
		log.Fatal("config err:", err)
	}

	url := config.Get("auth").String("")
	if ok, _ := regexp.MatchString("^https?://[-A-Za-z0-9+&@#/%?=~_|!:,.;]+[-A-Za-z0-9+&@#/%=~_|]$", url); !ok {
		log.Fatal("请在config.json中配置正确的认证地址")
	}

	// register chat handler
	cli := proto.NewChatService("go.micro.srv.chat", client.DefaultClient)
	service.HandleFunc("/stream", gochat.NewWebsocketHandler(cli))

	// run service
	if err := service.Run(); err != nil {
		log.Fatal(err)
	}
}
