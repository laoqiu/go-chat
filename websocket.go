package gochat

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	proto "github.com/laoqiu/go-chat/proto"
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
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	AcceptEvent = []string{"message", "receipt", "candidate", "sdp"}
)

type AuthBody struct {
	LastTime int64 `json:"last_time"`
}

func NewWebsocketHandler(cli proto.ChatService) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("Upgrade: ", err)
			return
		}
		defer conn.Close()

		if err := Stream(cli, conn); err != nil {
			log.Println("Echo: ", err)
			return
		}
	}
}

func Stream(cli proto.ChatService, ws *websocket.Conn) error {
	var (
		event proto.Event
		id    string
		send  chan *proto.Event
	)

	// send
	send = make(chan *proto.Event)

	// setting
	ws.SetReadLimit(maxMessageSize)
	ws.SetReadDeadline(time.Now().Add(pongWait))
	ws.SetPongHandler(func(string) error { ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	// 认证 `TODO`
	if err := ws.ReadJSON(&event); err != nil {
		return err
	}
	id = event.From

	// 认证内容中获取最后一次消息时间
	authBody := &AuthBody{LastTime: 0}
	if len(event.Body) > 0 {
		if err := json.Unmarshal([]byte(event.Body), authBody); err != nil {
			return err
		}
	}

	// stream
	stream, err := cli.Stream(context.Background(), &proto.StreamRequest{
		Id:          id,
		LastCreated: authBody.LastTime,
	})
	if err != nil {
		return err
	}
	defer stream.Close()

	// 接收stream的消息推送
	go func() {
		defer close(send)
		for {
			rsp, err := stream.Recv()
			if err != nil {
				if err != io.EOF {
					//log.Println("stream recv err ->", err)
					return
				}
				return
			}
			log.Println("stream recv ->", rsp.Event)
			//if in(AcceptEvent, rsp.Event.Type) {
			//	send <- rsp.Event
			//}
			send <- rsp.Event
		}
	}()

	// writer
	go func() {
		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()
		for {
			select {
			case event, ok := <-send:
				if !ok {
					// 服务端断开连接
					ws.WriteMessage(websocket.CloseMessage, []byte{})
					return
				}
				//log.Println("websocket writejson ->", event)
				if err := ws.WriteJSON(event); err != nil {
					return
				}
			case <-ticker.C:
				ws.SetWriteDeadline(time.Now().Add(writeWait))
				if err := ws.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
					return
				}
			}
		}
	}()

	// reader
	for {
		if err := ws.ReadJSON(&event); err != nil {
			// 判断客户端是否异常断开
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("IsUnexpectedCloseError: %v", err)
				break
			}
			return err
		}

		log.Println("websocket reader ->", event, id)

		if len(id) != 0 {
			switch event.Type {
			case "users":
				rsp, err := cli.Users(context.Background(), &proto.UsersRequest{
					Id: id,
				})
				if err != nil {
					send <- &proto.Event{
						Type: "error",
						Body: err.Error(),
					}
				} else {
					d, _ := json.Marshal(&rsp.Users)
					send <- &proto.Event{
						Type: "users",
						Body: string(d),
					}
				}
			case "rooms":
				rsp, err := cli.Rooms(context.Background(), &proto.RoomsRequest{
					Id: id,
				})
				if err != nil {
					send <- &proto.Event{
						Type: "error",
						Body: err.Error(),
					}
				} else {
					d, _ := json.Marshal(&rsp.Rooms)
					send <- &proto.Event{
						Type: "rooms",
						Body: string(d),
					}
				}
			case "join":
				if _, err := cli.Join(context.Background(), &proto.JoinRequest{
					Id:     id,
					RoomId: event.To,
				}); err != nil {
					send <- &proto.Event{
						Type: "error",
						Body: err.Error(),
					}
				}
			case "out":
				if _, err := cli.Out(context.Background(), &proto.OutRequest{
					Id:     id,
					RoomId: event.To,
				}); err != nil {
					send <- &proto.Event{
						Type: "error",
						Body: err.Error(),
					}
				}
			case "message", "receipt", "candidate", "sdp":
				// 重置From
				event.From = id
				// 发送
				if _, err := cli.Send(context.Background(), &proto.SendRequest{
					Event: &event,
				}); err != nil {
					send <- &proto.Event{
						Type: "error",
						Body: err.Error(),
					}
				}
			default:
				err := errors.New("not support event type")
				send <- &proto.Event{
					Type: "error",
					Body: err.Error(),
				}
			}
		}
	}

	return nil
}
