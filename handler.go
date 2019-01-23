package gochat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	proto "github.com/laoqiu/go-chat/proto"
	"github.com/micro/go-micro/broker"
	"github.com/nats-io/nats-streaming-server/server"
	uuid "github.com/satori/go.uuid"
)

type Handler struct {
	service string
	repo    Repository
	hub     *Hub
	broker  broker.Broker
}

func NewHandler(service string, repo Repository, hub *Hub, broker broker.Broker) *Handler {
	return &Handler{
		service: service,
		repo:    repo,
		hub:     hub,
		broker:  broker,
	}
}

func (h *Handler) Register(ctx context.Context, req *proto.RegisterRequest, rsp *proto.RegisterResponse) error {
	// 创建用户
	if err := h.repo.CreateUser(req.User); err != nil {
		return err
	}

	// 注册队列: 如果不注册将无法收到执久化消息
	conn := NewConn(h.service, req.User.Id, MasterPlatform, 0, nil)
	if err := conn.Init(); err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.Subscribe(); err != nil {
		return err
	}

	return nil
}

func (h *Handler) Unregister(ctx context.Context, req *proto.UnregisterRequest, rsp *proto.UnregisterResponse) error {
	// 查询用户
	// if _, err := h.repo.GetUser(req.Id); err != nil {
	// 	return err
	// }

	// 通知所有平台下线
	body, _ := json.Marshal(map[string]interface{}{
		"id":       req.Id,
		"platform": "all",
	})
	// 发送强制下线消息给所有srv
	if err := h.broker.Publish(h.service, &broker.Message{
		Body: body,
	}); err != nil {
		fmt.Println("stream publish err", err)
		return err
	}

	// 删除用户
	if err := h.repo.DeleteUser(req.Id); err != nil {
		return err
	}

	// 连上队列后执行退订
	conn := NewConn(h.service, req.Id, MasterPlatform, 0, nil)
	if err := conn.Init(); err != nil {
		return err
	}
	defer conn.Close()
	sub, err := conn.Subscribe()
	if err != nil {
		return err
	}
	// 退订
	defer sub.Unsubscribe()

	return nil
}

func (h *Handler) Users(ctx context.Context, req *proto.UsersRequest, rsp *proto.UsersResponse) error {
	users, err := h.repo.RequestUsers(req.Id)
	if err != nil {
		return err
	}
	rsp.Users = users
	return nil
}

func (h *Handler) Rooms(ctx context.Context, req *proto.RoomsRequest, rsp *proto.RoomsResponse) error {
	rooms, err := h.repo.RequestRooms(req.Id)
	if err != nil {
		return err
	}
	rsp.Rooms = rooms
	return nil
}

func (h *Handler) Join(ctx context.Context, req *proto.JoinRequest, rsp *proto.JoinResponse) error {

	if err := h.repo.Join(req.Id, req.RoomId); err != nil {
		return err
	}

	members, err := h.repo.Members(req.RoomId, false)
	if err != nil {
		return err
	}

	event, _ := json.Marshal(&proto.Event{
		Type: "join",
		From: req.Id,
		To:   req.RoomId,
	})

	for _, m := range members {
		topic := h.service + "." + m.Id
		if err := h.broker.Publish(topic, &broker.Message{Body: event}); err != nil {
			fmt.Println("DEBUG ->", err)
		}
	}

	return nil
}

func (h *Handler) Out(ctx context.Context, req *proto.OutRequest, rsp *proto.OutResponse) error {
	if err := h.repo.Out(req.Id, req.RoomId); err != nil {
		return err
	}

	managers, err := h.repo.Members(req.RoomId, true)
	if err != nil {
		return err
	}

	event, _ := json.Marshal(&proto.Event{
		Type: "out",
		From: req.Id,
		To:   req.RoomId,
	})

	for _, m := range managers {
		topic := h.service + "." + m.Id
		if err := h.broker.Publish(topic, &broker.Message{Body: event}); err != nil {
			fmt.Println("DEBUG ->", err)
		}
	}

	return nil
}

func (h *Handler) Send(ctx context.Context, req *proto.SendRequest, rsp *proto.SendResponse) error {
	log.Println("server recv event", req.Event)

	if !in(AcceptEvent, req.Event.Type) {
		return errors.New("不能接受的消息类型")
	}

	if len(req.Event.Id) == 0 {
		u1, _ := uuid.NewV4()
		req.Event.Id = strings.Replace(u1.String(), "-", "", -1)
	}

	event, err := json.Marshal(req.Event)
	if err != nil {
		return err
	}

	roomId, to := splitDest(req.Event.To)

	if len(roomId) > 0 {
		members, err := h.repo.Members(roomId, false)
		if err != nil {
			return err
		}
		for _, m := range members {
			topic := h.service + "." + m.Id
			if err := h.broker.Publish(topic, &broker.Message{Body: event}); err != nil {
				fmt.Println("Publish DEBUG ->", err)
			}
		}
	} else {
		// 判断用户是否存在
		if _, err := h.repo.GetUser(to); err != nil {
			return err
		}

		topic := h.service + "." + to
		if err := h.broker.Publish(topic, &broker.Message{Body: event}); err != nil {
			return err
		}
	}

	// 同时合并消息发送一条给管理后台订阅
	adminTopic := h.service + "." + "admin"
	if err := h.broker.Publish(adminTopic, &broker.Message{Body: event}); err != nil {
		return err
	}

	// 返回id
	rsp.Id = req.Event.Id

	return nil
}

func (h *Handler) Stream(ctx context.Context, req *proto.StreamRequest, stream proto.Chat_StreamStream) error {
	var (
		retry int
	)
	// 验证
	if err := req.Validate(); err != nil {
		return err
	}

	// 用户是否存在
	if _, err := h.repo.GetUser(req.Id); err != nil {
		return err
	}

	// 检查用户所有平台登录情况
	current, err := h.repo.AvailableClient(req.Id, req.Platform)
	if err != nil {
		return err
	}

	// 处理用户平台冲突强制下线逻辑
	if current.IsOnline {
		body, _ := json.Marshal(map[string]interface{}{
			"id":       current.Id,
			"platform": current.Platform,
		})
		// 发送强制下线消息给所有srv
		if err := h.broker.Publish(h.service, &broker.Message{
			Body: body,
		}); err != nil {
			fmt.Println("stream publish err", err)
			return err
		}
	}

	// 初始化
	conn := NewConn(h.service, req.Id, req.Platform, req.Start, stream)

	retry = 0
	for {
		if err := conn.Init(); err != nil {
			if err == server.ErrInvalidClient {
				// 同步等待200毫秒
				time.Sleep(200 * time.Millisecond)
				retry = retry + 1
				if retry > 3 {
					return err
				}
			}
			return err
		} else {
			break
		}
	}
	defer conn.Close()

	if _, err := conn.Subscribe(); err != nil {
		fmt.Println("subscribe err", err)
		return err
	}

	// 加入hub管理
	h.hub.Register(conn)
	defer h.hub.Unregister(conn)

	// 在线
	if err := h.repo.Online(req.Id, req.Platform); err != nil {
		return err
	}
	defer h.repo.Offline(req.Id, req.Platform)

	conn.Run()
	return nil
}
