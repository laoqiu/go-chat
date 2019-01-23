package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
	proto "github.com/laoqiu/go-chat/proto"
)

var (
	id, to, body string
	addr         string = "localhost:8082"
)

func main() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	recv := make(chan *proto.Event)
	send := make(chan *proto.Event)

	u := url.URL{Scheme: "ws", Host: addr, Path: "/chat/stream"}
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	fmt.Println("用户名:")
	fmt.Scanln(&id)

	d, _ := json.Marshal(map[string]interface{}{
		"id":       id,
		"platform": "mobile",
		"start":    time.Now().Unix(),
	})
	if err := c.WriteJSON(&proto.Event{
		Type: "auth",
		From: id,
		Body: string(d),
	}); err != nil {
		log.Println(err)
	}

	go func() {
		defer close(recv)
		var event proto.Event
		for {
			if err := c.ReadJSON(&event); err != nil {
				log.Println("readjson err", err)
				return
			}
			recv <- &event
		}
	}()

	go func() {
		for {
			fmt.Println("接收者 内容")
			fmt.Scanln(&to, &body)
			send <- &proto.Event{
				Type: "message",
				To:   to,
				Body: body,
			}
		}
	}()

	for {
		select {
		case event, ok := <-recv:
			if !ok {
				log.Println("server stream closed")
				return
			}
			log.Println("recv event ->", event)
		case event := <-send:
			log.Println("send event ->", event)
			if err := c.WriteJSON(event); err != nil {
				log.Println("!!err", err)
				return
			}
		case <-interrupt:
			log.Println("interrupt")
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write close:", err)
				return
			}
			return
		}
	}
}
