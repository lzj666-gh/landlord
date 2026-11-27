package service

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/astaxie/beego/logs"
	"github.com/gorilla/websocket"
	"landlord/common"
)

const (
	writeWait      = 1 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512

	RoleFarmer   = 0
	RoleLandlord = 1
)

var (
	newline  = []byte{'\n'}
	space    = []byte{' '}
	upGrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return false
			}
			parsed, err := url.Parse(origin)
			if err != nil || !strings.EqualFold(parsed.Host, r.Host) {
				return false
			}
			expectedScheme := "http"
			if r.TLS != nil {
				expectedScheme = "https"
			}
			return parsed.Scheme == expectedScheme
		},
	}
)

type UserId int

type UserInfo struct {
	UserId     UserId `json:"user_id"`
	Username   string `json:"username"`
	Coin       int    `json:"coin"`
	Background string `json:"background,omitempty"`
	CardFace   string `json:"cardface,omitempty"`
	Role       int
}

type Client struct {
	conn       *websocket.Conn
	UserInfo   *UserInfo
	Room       *Room
	Table      *Table
	HandPokers []int
	Ready      bool
	IsCalled   bool    //是否叫完分
	Next       *Client //链表
	IsRobot    bool
	BotLevel   botLevel           //机器人的难度
	Offline    bool               //连接已断开，座位保留等待重连
	toRobot    chan []interface{} //发送给robot的消息（测试里的假机器人用）
	inbox      *robotInbox        //真实机器人用：不设上限，发消息不会阻塞牌桌
	toServer   chan []interface{} //robot发送给服务器
	msgBox     chan []interface{} //测试用：不经过网络，直接收集下发的消息
}

//重置状态
func (c *Client) reset() {
	c.UserInfo.Role = RoleFarmer
	c.HandPokers = make([]int, 0, 21)
	c.Ready = false
	c.IsCalled = false
}

//发送房间内已有的牌桌信息
func (c *Client) sendRoomTables() {
	res := make([][2]int, 0)
	for _, table := range c.Room.Tables {
		if len(table.TableClients) < 3 {
			res = append(res, [2]int{int(table.TableId), table.humanCount()})
		}
	}
	c.sendMsg([]interface{}{common.ResTableList, res})
}

func (c *Client) sendMsg(msg []interface{}) {
	if c.Offline {
		//座位还在，但连接已经断开，等重连后再补发
		return
	}
	if c.msgBox != nil {
		//测试里的真人客户端没有 websocket，消息直接进收件箱
		c.msgBox <- msg
		return
	}
	if c.IsRobot {
		if c.inbox != nil {
			c.inbox.push(msg)
			return
		}
		c.toRobot <- msg
		return
	}
	if c.conn == nil {
		//没有连接的客户端（测试里的假客户端）不需要回消息
		return
	}
	msgByte, err := json.Marshal(msg)
	if err != nil {
		logs.Error("send msg [%v] marsha1 err:%v", string(msgByte), err)
		return
	}
	err = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
	if err != nil {
		logs.Error("send msg SetWriteDeadline [%v] err:%v", string(msgByte), err)
		return
	}
	w, err := c.conn.NextWriter(websocket.TextMessage)
	if err != nil {
		err = c.conn.Close()
		if err != nil {
			logs.Error("close client err: %v", err)
		}
	}
	_, err = w.Write(msgByte)
	if err != nil {
		logs.Error("Write msg [%v] err: %v", string(msgByte), err)
	}
	if err := w.Close(); err != nil {
		err = c.conn.Close()
		if err != nil {
			logs.Error("close err: %v", err)
		}
	}
}

//光比客户端
func (c *Client) close() {
	table := c.Table
	if table == nil {
		return
	}
	table.Lock.Lock()
	defer table.Lock.Unlock()
	if table.TableClients[c.UserInfo.UserId] != c {
		//座位已经被重连的新连接接管，旧连接不能再改动牌桌
		return
	}
	if !c.IsRobot && (table.State == GameCallScore || table.State == GamePlaying) {
		//牌局还没打完，保留座位等玩家重连，否则重连后会被当成新玩家重新发牌
		c.Offline = true
		logs.Debug("user [%d] offline, keep seat at table[%d]", c.UserInfo.UserId, table.TableId)
		return
	}
	table.removeClient(c)
	if !table.hasHuman() {
		//桌上一个真人都没有了，机器人和桌子一起收掉，别在房间里留下空桌
		for _, robot := range table.robots() {
			table.removeClient(robot)
		}
		table.Creator = nil
		if c.Room != nil {
			delete(c.Room.Tables, table.TableId)
		}
		return
	}
	table.syncUser()
}

//可能是因为版本问题，导致有些未处理的error
func (c *Client) readPump() {
	defer func() {
		//logs.Debug("readPump exit")
		c.conn.Close()
		c.close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logs.Error("websocket user_id[%d] unexpected close error: %v", c.UserInfo.UserId, err)
			}
			return
		}
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		var data []interface{}
		err = json.Unmarshal(message, &data)
		if err != nil {
			logs.Error("message unmarsha1 err, user_id[%d] err:%v", c.UserInfo.UserId, err)
		} else {
			wsRequest(data, c)
		}
	}
}

//心跳
func (c *Client) Ping() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
	}()
	for {
		select {
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func ServeWs(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	session, ok := common.GetSession(cookie.Value)
	if !ok {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	conn, err := upGrader.Upgrade(w, r, nil)
	if err != nil {
		logs.Error("upgrader err:%v", err)
		return
	}
	profile, err := common.LoadProfile(session.UserID)
	if err != nil {
		logs.Error("load profile for user[%d] failed: %v", session.UserID, err)
	}
	client := &Client{
		conn:       conn,
		HandPokers: make([]int, 0, 21),
		UserInfo: &UserInfo{
			UserId:     UserId(session.UserID),
			Username:   session.Username,
			Coin:       profile.Coin,
			Background: profile.Background,
			CardFace:   profile.CardFace,
		},
	}
	go client.readPump()
	go client.Ping()
}
