package gochat

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/go-redis/redis"
	proto "github.com/laoqiu/go-chat/proto"
	"github.com/rs/xid"
)

var (
	DefaultAddr = "127.0.0.1:6379"
)

type DB interface {
	// 读取
	Read(uid string, start int64) ([]*proto.Event, error)
	// 写入
	Write(event *proto.Event) (string, error)
	// 删除
	Remove(messageId string) error
	// 房间列表
	RequestRooms(uid string) ([]*proto.Room, error)
	// 好友列表
	RequestUsers(uid string) ([]*proto.User, error)
	// 加入房间
	Join(uid, roomId string) error
	// 退出房间
	Out(uid, roomId string) error
	// 上线
	Online(uid string) error
	// 下线
	Offline(uid string) error
}

type Redis struct {
	prefix string
	opts   *redis.Options
}

func NewRedis(prefix string, opts *redis.Options) *Redis {
	if len(opts.Addr) == 0 {
		opts.Addr = DefaultAddr
	}
	return &Redis{
		prefix: prefix,
		opts:   opts,
	}
}

// getKey 返回redis的key，规则如下:
// prefix:M:id -> hset message data
// prefix:U:id -> hset user data
// prefix:R:id -> hset room data
// prefix:RU:id -> set users by room
// prefix:UF:id -> set friends (TODO)
// prefix:UR:id -> set rooms by user
// prefix:UM:id -> sortedset message by user
func (r *Redis) getKey(dest, id string) string {
	return r.prefix + ":" + dest + ":" + id
}

func (r *Redis) id() string {
	return xid.New().String()
}

func (r *Redis) Read(uid string, start int64) ([]*proto.Event, error) {
	c := redis.NewClient(r.opts)
	defer c.Close()

	key := r.getKey("UM", uid)
	result := []*proto.Event{}

	end := time.Now().Unix()

	for _, i := range c.ZRangeByScore(key, redis.ZRangeBy{
		Min: fmt.Sprintf("%d", start),
		Max: fmt.Sprintf("%d", end),
	}).Val() {
		event := c.HGetAll(i).Val()
		created, _ := strconv.ParseInt(event["created"], 10, 64)
		result = append(result, &proto.Event{
			Type:    "message",
			From:    event["from"],
			To:      event["to"],
			Body:    event["body"],
			Created: created,
		})
	}

	// 清除start之前的数据
	c.ZRemRangeByScore(key, "-inf", fmt.Sprintf("%d", start)).Val()

	return result, nil
}

// Write message to redis
func (r *Redis) Write(event *proto.Event) (string, error) {
	c := redis.NewClient(r.opts)
	defer c.Close()

	roomId, userId := splitDest(event.To)

	// 组织users
	users := []string{}
	if len(roomId) > 0 {
		// 查询room下所有用户
		for _, uid := range c.LRange(r.getKey("RU", roomId), 0, -1).Val() {
			users = append(users, uid)
		}
	} else {
		users = append(users, userId)
	}

	// 如果没有接收对象直接返回
	if len(users) == 0 {
		return "", errors.New("no receivable users")
	}

	lastid := r.id()
	key := r.getKey("M", lastid)

	if event.Created == 0 {
		event.Created = time.Now().Unix()
	}

	if _, err := c.HMSet(key, map[string]interface{}{
		"from":    event.From,
		"to":      event.To,
		"created": event.Created,
		"body":    event.Body,
	}).Result(); err != nil {
		return lastid, err
	}

	// 保存关联
	for _, uid := range users {
		if _, err := c.ZAdd(r.getKey("UM", uid), redis.Z{Score: float64(event.Created), Member: key}).Result(); err != nil {
			return lastid, err
		}
	}

	return lastid, nil
}

func (r *Redis) Remove(messageId string) error {
	c := redis.NewClient(r.opts)
	defer c.Close()

	key := r.getKey("M", messageId)
	if _, err := c.Del(key).Result(); err != nil {
		return err
	}
	return nil
}

func (r *Redis) RequestRooms(uid string) ([]*proto.Room, error) {
	c := redis.NewClient(r.opts)
	defer c.Close()

	key := r.getKey("UR", uid)
	result := []*proto.Room{}

	for _, i := range c.LRange(key, 0, -1).Val() {
		d := c.HGetAll(r.getKey("R", i)).Val()
		result = append(result, &proto.Room{
			Id:   d["id"],
			Name: d["name"],
		})
	}

	return result, nil
}

func (r *Redis) RequestUsers(uid string) ([]*proto.User, error) {
	c := redis.NewClient(r.opts)
	defer c.Close()

	//key := r.getKey("UF", uid)
	key := r.getKey("U", "*") // all users
	result := []*proto.User{}

	for _, i := range c.Keys(key).Val() {
		d := c.HGetAll(i).Val()
		result = append(result, &proto.User{
			Id:   d["id"],
			Name: d["name"],
		})
	}

	return result, nil
}

func (r *Redis) Join(uid, roomId string) error {
	c := redis.NewClient(r.opts)
	defer c.Close()

	// find room
	if c.Exists(r.getKey("R", roomId)).Val() == 0 {
		return errors.New("Room not exitsts")
	}

	if _, err := c.SAdd(r.getKey("RU", roomId), r.getKey("U", uid)).Result(); err != nil {
		return err
	}

	return nil
}

func (r *Redis) Out(uid, roomId string) error {
	c := redis.NewClient(r.opts)
	defer c.Close()

	if _, err := c.SRem(r.getKey("RU", roomId), r.getKey("U", uid)).Result(); err != nil {
		return err
	}

	return nil
}

func (r *Redis) Online(uid string) error {
	c := redis.NewClient(r.opts)
	defer c.Close()

	if _, err := c.HIncrBy(r.getKey("U", uid), "online", 1).Result(); err != nil {
		return err
	}

	return nil
}

func (r *Redis) Offline(uid string) error {
	c := redis.NewClient(r.opts)
	defer c.Close()

	if _, err := c.HIncrBy(r.getKey("U", uid), "online", -1).Result(); err != nil {
		return err
	}

	return nil
}
