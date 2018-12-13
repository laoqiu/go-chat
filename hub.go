package gochat

import (
	"log"

	proto "github.com/laoqiu/go-chat/proto"
)

type Hub struct {
	clients    map[*Conn]bool
	send       chan *proto.Event
	received   chan *proto.Event
	register   chan *Conn
	unregister chan *Conn
}

func NewHub() *Hub {
	return &Hub{
		received:   make(chan *proto.Event),
		register:   make(chan *Conn),
		unregister: make(chan *Conn),
		clients:    make(map[*Conn]bool),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			log.Println("register")
			h.clients[client] = true
		case client := <-h.unregister:
			log.Println("unregister")
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		case event := <-h.received:
			log.Println("hub received ->", event)
			switch event.Type {
			case "message", "receipt", "candidate", "sdp":
				for client := range h.clients {
					roomId, userId := splitDest(event.To)
					if userId == client.id || (event.Type == "message" && in(client.rooms, roomId)) {
						client.send <- event
					}
				}
			case "join", "out":
				for client := range h.clients {
					if client.id == event.From {
						if event.Type == "join" {
							client.rooms = append(client.rooms, event.To)
						} else {
							client.rooms = remove(client.rooms, event.To)
						}
					}
				}
			case "online", "offline":
				for client := range h.clients {
					if in(client.users, event.From) {
						client.send <- event
					}
				}
			}
		}
	}
}
