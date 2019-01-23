package gochat

import (
	"database/sql"

	"github.com/jmoiron/sqlx"
	proto "github.com/laoqiu/go-chat/proto"
	"github.com/laoqiu/sqlxt"
)

var (
	Debug = true
)

type Repository interface {
	NewTx() (*sqlx.Tx, error)
	// 查询用户已登录客户端
	AvailableClient(uid, platform string) (*proto.Client, error)
	// 房间列表
	RequestRooms(uid string) ([]*proto.Room, error)
	// 好友列表
	RequestUsers(uid string) ([]*proto.User, error)
	// 查询用户
	GetUser(id string) (*proto.User, error)
	// 注册用户
	CreateUser(user *proto.User) error
	// 修改用户信息
	UpdateUser(user *proto.User) error
	// 注销用户
	DeleteUser(id string) error
	// TODO 按关键字搜索房间
	// TODO 获取房间信息
	GetRoom(id string) (*proto.Room, error)
	// TODO 创建房间(群聊)
	CreateRoom(room *proto.Room) error
	// TODO 更新部分群组信息
	UpdateRoom(room *proto.Room) error
	// TODO 删除房间
	DeleteRoom(roomId string) error
	// 成员列表
	Members(roomId string, onlyManager bool) ([]*proto.User, error)
	// TODO 加入房间
	Join(uid, roomId string) error
	// TODO 退出房间
	Out(uid, roomId string) error
	// 上线
	Online(uid, platform string) error
	// 下线
	Offline(uid, platform string) error
}

func NewChatRepo(db *sqlx.DB) *chatRepo {
	return &chatRepo{
		db: db,
	}
}

type chatRepo struct {
	db *sqlx.DB
}

func (r *chatRepo) NewTx() (*sqlx.Tx, error) {
	return r.db.Beginx()
}

func (r *chatRepo) AvailableClient(uid, platform string) (*proto.Client, error) {
	client := &proto.Client{}
	if err := r.db.Get(client, `
		SELECT id, platform, is_online FROM user_status WHERE id = ? AND platform = ?
		`, uid, platform); err != nil && err != sql.ErrNoRows {
		return client, err
	}
	return client, nil
}

func (r *chatRepo) RequestRooms(uid string) ([]*proto.Room, error) {
	rooms := []*proto.Room{}
	err := r.db.Select(&rooms, `
		SELECT g.id, g.name FROM chatgroup_members AS gm JOIN chatgroup AS g ON g.id = gm.group_id Where gm.member = ?
	`, uid)
	return rooms, err
}

func (r *chatRepo) RequestUsers(uid string) ([]*proto.User, error) {
	users := []*proto.User{}
	err := r.db.Select(&users, `SELECT id, name FROM users`)
	return users, err
}

func (r *chatRepo) CreateUser(user *proto.User) error {
	_, err := sqlxt.New(r.db, sqlxt.Table("users"), Debug).InsertIgnore(user)
	return err
}

func (r *chatRepo) UpdateUser(user *proto.User) error {
	_, err := sqlxt.New(r.db, sqlxt.Table("users"), Debug).InsertOnDuplicateUpdate(user)
	return err
}

func (r *chatRepo) DeleteUser(id string) error {
	_, err := sqlxt.New(r.db, sqlxt.Table("users").Where("id = ?", id), Debug).Delete()
	return err
}

func (r *chatRepo) GetUser(id string) (*proto.User, error) {
	user := &proto.User{}
	err := sqlxt.New(r.db, sqlxt.Table("users").Fields("id", "name"), Debug).Get(user)
	return user, err
}

func (r *chatRepo) GetRoom(id string) (*proto.Room, error) {
	room := &proto.Room{}
	err := sqlxt.New(r.db, sqlxt.Table("chatgroup").Where("id = ?", id), Debug).Get(room)
	return room, err
}

func (r *chatRepo) CreateRoom(room *proto.Room) error {
	return nil
}

func (r *chatRepo) UpdateRoom(room *proto.Room) error {
	return nil
}

func (r *chatRepo) DeleteRoom(roomId string) error {
	return nil
}

func (r *chatRepo) Members(roomId string, onlyManager bool) ([]*proto.User, error) {
	users := []*proto.User{}
	query := sqlxt.Table("chatgroup_members").Fields("users.id", "users.name").
		Join("users", "users.id = chatgroup_members.member").
		Where("group_id = ?", roomId)
	if onlyManager {
		query = query.Where("chatgroup_members.is_manager = 1")
	}
	err := sqlxt.New(r.db, query, Debug).All(&users)
	return users, err
}

func (r *chatRepo) Join(uid, roomId string) error {
	return nil
}

func (r *chatRepo) Out(uid, roomId string) error {
	return nil
}

func (r *chatRepo) Online(uid, platform string) error {
	if _, err := r.db.Exec(`
		INSERT INTO user_status (user_id, platform, is_online) VALUES (?, ?, 1) 
		ON DUPLICATE KEY UPDATE is_online = 1
		`, uid, platform); err != nil {
		return err
	}
	return nil
}

func (r *chatRepo) Offline(uid, platform string) error {
	if _, err := r.db.Exec(`
		UPDATE user_status SET is_online = 0 WHERE user_id = ? AND platform = ?
		`, uid, platform); err != nil {
		return err
	}
	return nil
}
