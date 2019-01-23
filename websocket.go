package gochat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	proto "github.com/laoqiu/go-chat/proto"
	config "github.com/micro/go-config"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 50 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 8 * 1024
)

var (
	newline  = []byte{'\n'}
	space    = []byte{' '}
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	AcceptEvent = []string{"message", "notify", "receipt", "candidate", "sdp"}
)

type AuthBody struct {
	Id       string `json:"id"`
	Password string `json:"password"`
	Platform string `json:"platform"` // 来源平台
	Start    int64  `json:"start"`    // 同步消息开始时间
}

type connection struct {
	id       string // 用户id
	platform string
	start    int64
	send     chan *proto.Event
	ws       *websocket.Conn
	cli      proto.ChatService
	stream   proto.Chat_StreamService
}

func NewWebsocketHandler(cli proto.ChatService) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("Upgrade: ", err)
			return
		}
		defer ws.Close()

		// 初始化客户端
		conn := &connection{
			ws:   ws,
			cli:  cli,
			send: make(chan *proto.Event),
		}

		if err := conn.login(); err != nil {
			log.Println(err)
			return
		}

		// stream
		stream, err := cli.Stream(context.Background(), &proto.StreamRequest{
			Id:       conn.id,
			Platform: conn.platform,
			Start:    conn.start,
		})
		if err != nil {
			fmt.Println("stream err", err)
			return
		}
		defer stream.Close()

		go conn.subscribe(stream)
		go conn.writer()

		if err := conn.reader(); err != nil {
			log.Println(err)
		}
	}
}

func (c *connection) login() error {
	var (
		event proto.Event
		url   string
	)
	if err := c.ws.ReadJSON(&event); err != nil {
		return err
	}

	config.Get("auth").Scan(&url)

	if _, err := http.Post(url, "application/json", strings.NewReader(event.Body)); err != nil {
		return errors.New("认证请求错误:" + err.Error())
	}

	// 认证内容分析
	body := &AuthBody{}
	if len(event.Body) > 0 {
		if err := json.Unmarshal([]byte(event.Body), body); err != nil {
			return err
		}
	}

	// 登录成功
	c.id = body.Id
	c.platform = body.Platform
	c.start = body.Start

	return nil
}

func (c *connection) subscribe(stream proto.Chat_StreamService) {
	for {
		rsp, err := stream.Recv()
		if err != nil {
			fmt.Println("stream recv err", err)
			close(c.send)
			return
		}
		if rsp.Event.Type == "heartbeat" {
			continue
		}
		fmt.Println("rsp.event ->", rsp.Event)
		if in(AcceptEvent, rsp.Event.Type) {
			c.send <- rsp.Event
		}
	}
}

func (c *connection) reader() error {
	// setting
	c.ws.SetReadLimit(maxMessageSize)
	c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error { c.ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	// reader
	for {
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			// 判断客户端是否异常断开
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("IsUnexpectedCloseError: %v", err)
				break
			}
			return err
		}

		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))

		event := proto.Event{}
		if err := json.Unmarshal(message, &event); err != nil {
			return err
		}

		log.Println("websocket reader ->", event, c.id)

		if len(c.id) != 0 {
			switch event.Type {
			case "users":
				rsp, err := c.cli.Users(context.Background(), &proto.UsersRequest{
					Id: c.id,
				})
				if err != nil {
					c.send <- &proto.Event{
						Type: "error",
						Body: err.Error(),
					}
				} else {
					d, _ := json.Marshal(&rsp.Users)
					c.send <- &proto.Event{
						Type: "users",
						Body: string(d),
					}
				}
			case "rooms":
				rsp, err := c.cli.Rooms(context.Background(), &proto.RoomsRequest{
					Id: c.id,
				})
				if err != nil {
					c.send <- &proto.Event{
						Type: "error",
						Body: err.Error(),
					}
				} else {
					d, _ := json.Marshal(&rsp.Rooms)
					c.send <- &proto.Event{
						Type: "rooms",
						Body: string(d),
					}
				}
			case "join":
				if _, err := c.cli.Join(context.Background(), &proto.JoinRequest{
					Id:     c.id,
					RoomId: event.To,
				}); err != nil {
					c.send <- &proto.Event{
						Type: "error",
						Body: err.Error(),
					}
				}
			case "out":
				if _, err := c.cli.Out(context.Background(), &proto.OutRequest{
					Id:     c.id,
					RoomId: event.To,
				}); err != nil {
					c.send <- &proto.Event{
						Type: "error",
						Body: err.Error(),
					}
				}
			case "message", "receipt", "candidate", "sdp":
				// 重置From
				event.From = c.id
				// 发送
				_, err := c.cli.Send(context.Background(), &proto.SendRequest{
					Event: &event,
				})
				if err != nil {
					c.send <- &proto.Event{
						Type: "error",
						Body: err.Error(),
					}
				} else {
					c.send <- &proto.Event{
						Id:   event.Id,
						Type: "received",
					}
				}
			default:
				err := errors.New("not support event type")
				c.send <- &proto.Event{
					Type: "error",
					Body: err.Error(),
				}
			}
		}
	}
	return nil
}

func (c *connection) writer() {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	for {
		select {
		case event, ok := <-c.send:
			c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// 服务端断开连接
				log.Println("服务端断开连接")
				c.ws.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			//log.Println("websocket writejson ->", event)
			if err := c.ws.WriteJSON(event); err != nil {
				return
			}
		case <-ticker.C:
			// 心跳
			c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.ws.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}
