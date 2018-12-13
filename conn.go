package gochat

import (
	"log"
	"time"

	proto "github.com/laoqiu/go-chat/proto"
)

type Conn struct {
	// 用户id
	id string

	// 房间列表
	rooms []string

	// 好友列表
	users []string

	// hub
	hub *Hub

	// stream
	stream proto.Chat_StreamStream

	// send
	send chan *proto.Event
}

func NewConn(id string, hub *Hub, stream proto.Chat_StreamStream) *Conn {
	return &Conn{
		id:     id,
		hub:    hub,
		stream: stream,
		send:   make(chan *proto.Event),
	}
}

func (c *Conn) Init(db DB) error {
	rooms, err := db.RequestRooms(c.id)
	if err != nil {
		return err
	}
	users, err := db.RequestUsers(c.id)
	if err != nil {
		return err
	}
	c.rooms = getRoomId(rooms)
	c.users = getUserId(users)
	return nil
}

func (c *Conn) Run() {
	ticker := time.NewTicker(time.Second * 3)
	defer ticker.Stop()
	for {
		select {
		case event := <-c.send:
			if err := c.stream.Send(&proto.StreamResponse{
				Event: event,
			}); err != nil {
				log.Println("!!err", err)
				return
			}
		case <-ticker.C:
			// 健康检查
			if err := c.stream.Send(&proto.StreamResponse{
				Event: &proto.Event{
					Type: "checkhealth",
				},
			}); err != nil {
				log.Println("!!err", err)
				return
			}
		}
	}
}
