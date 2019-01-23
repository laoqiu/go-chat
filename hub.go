package gochat

import (
	"encoding/json"
	"log"

	"github.com/micro/go-micro/broker"
)

type Hub struct {
	service string
	broker  broker.Broker
	clients map[*Conn]bool
}

func NewHub(service string, broker broker.Broker) *Hub {
	return &Hub{
		service: service,
		broker:  broker,
		clients: make(map[*Conn]bool),
	}
}

func (h *Hub) Register(conn *Conn) {
	log.Println("hub register", conn.id, conn.platform)
	h.clients[conn] = true
}

func (h *Hub) Unregister(conn *Conn) {
	log.Println("hub unregister")
	delete(h.clients, conn)
}

func (h *Hub) Subscribe() (broker.Subscriber, error) {
	return h.broker.Subscribe(h.service, func(p broker.Publication) error {
		log.Println("[hub] received message:", string(p.Message().Body), "header", p.Message().Header)
		data := map[string]interface{}{}
		if err := json.Unmarshal(p.Message().Body, &data); err != nil {
			return err
		}
		// 处理强制下线逻辑
		h.shutdown(data["id"].(string), data["platform"].(string))
		return nil
	})
}

func (h *Hub) shutdown(id, platform string) {
	for client := range h.clients {
		// platform为all时强制下线所有客户端
		if client.id == id && (platform == "all" || client.platform == platform) {
			client.done <- 1
		}
	}
}
