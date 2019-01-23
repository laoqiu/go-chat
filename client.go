package gochat

import (
	"net/url"

	"github.com/gorilla/websocket"
	proto "github.com/laoqiu/go-chat/proto"
)

const (
	defaultHost = "localhost:8082"
	defaultPath = "/chat/stream"
)

// A Client represents the connection between the application to the HipChat
// service.
type Client struct {
	Id          string
	MentionName string
	Password    string

	// private
	connection      *websocket.Conn
	receivedUsers   chan []*User
	receivedRooms   chan []*Room
	receivedMessage chan *Message
	host            string
}

// A Message represents a message received from HipChat.
type Message struct {
	From        string
	To          string
	Body        string
	Type        string
	MentionName string
}

// A User represents a member of the HipChat service.
type User struct {
	Id          string
	Name        string
	MentionName string
}

// A Room represents a room in HipChat the Client can join to communicate with
// other members..
type Room struct {
	Id   string
	Name string
}

func NewClient(id, pass string) (*Client, error) {
	return NewClientWithServerInfo(id, pass, defaultHost, defaultPath)
}

func NewClientWithServerInfo(id, pass, host, path string) (*Client, error) {
	u := url.URL{Scheme: "ws", Host: host, Path: path}
	connection, _, err := websocket.DefaultDialer.Dial(u.String(), nil)

	c := &Client{
		Id:       id,
		Password: pass,

		// private
		connection:      connection,
		receivedUsers:   make(chan []*User),
		receivedRooms:   make(chan []*Room),
		receivedMessage: make(chan *Message),
		host:            host,
	}

	if err != nil {
		return c, err
	}

	err = c.authenticate()
	if err != nil {
		return c, err
	}

	go c.listen()
	return c, nil
}

func (c *Client) Close() error {
	return c.connection.Close()
}

// Messages returns a read-only channel of Message structs. After joining a
// room, messages will be sent on the channel.
func (c *Client) Messages() <-chan *Message {
	return c.receivedMessage
}

// Rooms returns a channel of Room slices
func (c *Client) Rooms() <-chan []*Room {
	return c.receivedRooms
}

// Users returns a channel of User slices
func (c *Client) Users() <-chan []*User {
	return c.receivedUsers
}

func (c *Client) Join(roomId string) error {
	return c.connection.WriteJSON(&proto.Event{
		Type: "join",
		To:   roomId,
	})
}

func (c *Client) Out(roomId string) error {
	return c.connection.WriteJSON(&proto.Event{
		Type: "out",
		To:   roomId,
	})
}

func (c *Client) Say(roomId, name, body string) error {
	return c.connection.WriteJSON(&proto.Event{
		Type: "message",
		To:   name + "/" + roomId,
		Body: body,
	})
}

func (c *Client) RequestRooms() error {
	return c.connection.WriteJSON(&proto.Event{
		Type: "rooms",
	})
}

func (c *Client) RequestUsers() error {
	return c.connection.WriteJSON(&proto.Event{
		Type: "users",
	})
}

func (c *Client) authenticate() error {
	return c.connection.WriteJSON(&proto.Event{
		Type: "auth",
		From: c.Id,
	})
}

func (c *Client) listen() {
}
