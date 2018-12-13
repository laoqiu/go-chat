package gochat

import (
	"context"
	"errors"
	"log"

	proto "github.com/laoqiu/go-chat/proto"
	micro "github.com/micro/go-micro"
)

type Handler struct {
	db  DB
	hub *Hub
	pub micro.Publisher
}

func NewHandler(db DB, hub *Hub, pub micro.Publisher) *Handler {
	return &Handler{
		db:  db,
		hub: hub,
		pub: pub,
	}
}

func NewSubscriber(hub *Hub) func(ctx context.Context, event *proto.Event) error {
	return func(ctx context.Context, event *proto.Event) error {
		hub.received <- event
		return nil
	}
}

func (h *Handler) Users(ctx context.Context, req *proto.UsersRequest, rsp *proto.UsersResponse) error {
	users, err := h.db.RequestUsers(req.Id)
	if err != nil {
		return err
	}
	rsp.Users = users
	return nil
}

func (h *Handler) Rooms(ctx context.Context, req *proto.RoomsRequest, rsp *proto.RoomsResponse) error {
	rooms, err := h.db.RequestRooms(req.Id)
	if err != nil {
		return err
	}
	rsp.Rooms = rooms
	return nil
}

func (h *Handler) Join(ctx context.Context, req *proto.JoinRequest, rsp *proto.JoinResponse) error {
	if err := h.db.Join(req.Id, req.RoomId); err != nil {
		return err
	}
	if err := h.pub.Publish(ctx, &proto.Event{
		Type: "join",
		From: req.Id,
		To:   req.RoomId,
	}); err != nil {
		return err
	}
	return nil
}

func (h *Handler) Out(ctx context.Context, req *proto.OutRequest, rsp *proto.OutResponse) error {
	if err := h.db.Out(req.Id, req.RoomId); err != nil {
		return err
	}
	if err := h.pub.Publish(ctx, &proto.Event{
		Type: "out",
		From: req.Id,
		To:   req.RoomId,
	}); err != nil {
		return err
	}
	return nil
}

func (h *Handler) Send(ctx context.Context, req *proto.SendRequest, rsp *proto.SendResponse) error {
	log.Println("server recv event", req.Event)

	if !in(AcceptEvent, req.Event.Type) {
		return errors.New("不能接受的消息类型")
	}
	lastid, err := h.db.Write(req.Event)
	if err != nil {
		return err
	}
	if err := h.pub.Publish(ctx, req.Event); err != nil {
		h.db.Remove(lastid)
		return err
	}
	rsp.MessageId = lastid
	return nil
}

func (h *Handler) Stream(ctx context.Context, req *proto.StreamRequest, stream proto.Chat_StreamStream) error {

	cl := NewConn(req.Id, h.hub, stream)
	if err := cl.Init(h.db); err != nil {
		return err
	}

	defer func() {
		h.hub.unregister <- cl
		h.Offline(ctx, req.Id) // 离线
	}()

	h.hub.register <- cl

	// 在线
	if err := h.Online(ctx, req.Id); err != nil {
		return err
	}

	// 异步读取离线消息，防止send通道阻塞
	go func() {
		events, err := h.db.Read(req.Id, req.LastCreated)
		if err != nil {
			log.Println("!!err", err)
			return
		}
		log.Println("read events number:", len(events))
		for _, v := range events {
			cl.send <- v
		}
	}()

	// 运行
	cl.Run()

	return nil
}

func (h *Handler) Offline(ctx context.Context, id string) error {
	if err := h.db.Offline(id); err != nil {
		return err
	}
	if err := h.pub.Publish(ctx, &proto.Event{
		Type: "offline",
		From: id,
	}); err != nil {
		return err
	}
	return nil
}

func (h *Handler) Online(ctx context.Context, id string) error {
	if err := h.db.Online(id); err != nil {
		return err
	}
	if err := h.pub.Publish(ctx, &proto.Event{
		Type: "online",
		From: id,
	}); err != nil {
		return err
	}
	return nil
}

func getRoomId(items []*proto.Room) []string {
	rooms := make([]string, len(items))
	for i, v := range items {
		rooms[i] = v.Id
	}
	return rooms
}

func getUserId(items []*proto.User) []string {
	users := make([]string, len(items))
	for i, v := range items {
		users[i] = v.Id
	}
	return users
}
