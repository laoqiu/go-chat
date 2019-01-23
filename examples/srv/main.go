package main

import (
	"log"

	gochat "github.com/laoqiu/go-chat"
	proto "github.com/laoqiu/go-chat/proto"
	stan "github.com/laoqiu/go-plugins/broker/nats-streaming"
	"github.com/laoqiu/sqlxt"
	"github.com/micro/cli"
	micro "github.com/micro/go-micro"
	"github.com/micro/go-micro/broker"
)

func main() {
	var serviceName string

	dbOpts := []sqlxt.Option{}
	brokerOpts := []broker.Option{}

	// create a service
	service := micro.NewService(
		micro.Name("go.micro.srv.chat"),
		micro.Version("0.2"),
		micro.Flags(
			cli.StringFlag{
				Name:   "database_url",
				EnvVar: "DATABASE_URL",
				Usage:  "The database URL e.g root@tcp(127.0.0.1:3306)/test",
			},
			cli.StringFlag{
				Name:   "nats_client_id",
				EnvVar: "NATS_CLIENT_ID",
				Usage:  "The nats streaming client_id",
			},
			cli.StringFlag{
				Name:   "nats_address",
				EnvVar: "NATS_ADDRESS",
				Usage:  "The nats streaming address",
			},
		),
		micro.Action(func(c *cli.Context) {
			if len(c.String("server_name")) > 0 {
				serviceName = c.String("server_name")
			}
			if len(c.String("database_url")) > 0 {
				dbOpts = append(dbOpts, sqlxt.URI(c.String("database_url")))
			}
			if len(c.String("nats_address")) > 0 {
				brokerOpts = append(brokerOpts, broker.Addrs((c.String("nats_address"))))
			}
			if len(c.String("nats_client_id")) > 0 {
				brokerOpts = append(brokerOpts, stan.ClientID(c.String("nats_client_id")))
			}
		}),
	)

	// parse command line
	service.Init()

	// broker connect
	sbroker := stan.NewBroker(brokerOpts...)
	if err := sbroker.Connect(); err != nil {
		log.Fatal(err)
	}

	// 数据连接
	conn, err := gochat.Init(dbOpts...)
	if err != nil {
		log.Fatal(err)
	}
	repo := gochat.NewChatRepo(conn)

	// Hub
	hub := gochat.NewHub(serviceName, sbroker)
	sub, err := hub.Subscribe()
	if err != nil {
		log.Fatal(err)
	}
	defer sub.Unsubscribe()

	proto.RegisterChatHandler(service.Server(), gochat.NewHandler(serviceName, repo, hub, sbroker))

	if err := service.Run(); err != nil {
		log.Fatal(err)
	}
}
