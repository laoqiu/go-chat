package gochat

import (
	"encoding/json"
	"log"
	"strings"
	"time"

	proto "github.com/laoqiu/go-chat/proto"
	stan "github.com/laoqiu/go-plugins/broker/nats-streaming"
	"github.com/micro/go-micro/broker"
)

var (
	// 持久化队列的平台，此值不可随意更改，否则会收不到队列消息
	MasterPlatform = "mobile"
)

type Conn struct {
	// service from config
	service string

	// 客户端信息
	id       string
	platform string
	start    int64

	// stream
	stream proto.Chat_StreamStream

	done chan byte

	broker broker.Broker
}

func NewConn(service, id, platform string, start int64, stream proto.Chat_StreamStream) *Conn {
	return &Conn{
		service:  service,
		id:       id,
		platform: platform,
		start:    start,
		stream:   stream,
		done:     make(chan byte),
	}
}

func (c *Conn) Init() error {
	clusterId := "test-cluster"
	// clientId不能重复(一个用户每个平台只允许一个订阅), 且只支持符号"-"和"_"
	clientId := strings.Replace(c.service+"."+c.id+"."+c.platform, ".", "-", -1)
	sbroker := stan.NewBroker(
		broker.Addrs("nats://admin:test@localhost:4222"),
		stan.ClusterID(clusterId),
		stan.ClientID(clientId),
	)
	if err := sbroker.Init(); err != nil {
		return err
	}
	// 连接broker(nats-streaming)
	if err := sbroker.Connect(); err != nil {
		return err
	}
	c.broker = sbroker
	return nil
}

func (c *Conn) Close() error {
	log.Println("broker disconnect")
	return c.broker.Disconnect()
}

func (c *Conn) Subscribe() (broker.Subscriber, error) {
	var master bool

	// 监听消息队列
	topic := c.service + "." + c.id // 每个用户一个订阅地址
	opts := []broker.SubscribeOption{}

	if c.platform == MasterPlatform {
		opts = append(opts, stan.SetManualAckMode(), stan.DurableName("messages"))
		master = true
	} else {
		if c.start == 0 {
			opts = append(opts, stan.DeliverAllAvailable())
		} else {
			opts = append(opts, stan.StartAtTime(time.Unix(c.start, 0)))
		}
	}

	return c.broker.Subscribe(topic, func(p broker.Publication) error {
		log.Println("[sub] received message:", string(p.Message().Body), "header", p.Message().Header)

		if c.stream != nil {
			// 解析event
			event := &proto.Event{}
			if err := json.Unmarshal(p.Message().Body, event); err != nil {
				return err
			}
			if err := c.stream.Send(&proto.StreamResponse{Event: event}); err != nil {
				return err
			}
		}

		// 非SetManualAckMode执行ack会返回ErrManualAck
		if master {
			return p.Ack()
		}

		return nil
	}, opts...)
}

func (c *Conn) Publish(topic string, event *proto.Event) error {
	return nil
}

func (c *Conn) Run() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := c.stream.Send(&proto.StreamResponse{
				Event: &proto.Event{Type: "heartbeat"},
			}); err != nil {
				return
			}
		case <-c.done:
			return
		}
	}
}
