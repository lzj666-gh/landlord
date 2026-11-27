package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"landlord/common"
)

//测试用的websocket客户端：后台收消息，测试按顺序取
type wsTestClient struct {
	conn *websocket.Conn
	msgs chan []interface{}
}

func newWsTestClient(t *testing.T, server *httptest.Server, token string) *wsTestClient {
	t.Helper()
	header := http.Header{}
	header.Set("Cookie", "session="+token)
	header.Set("Origin", server.URL)
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/ws", header)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	client := &wsTestClient{conn: conn, msgs: make(chan []interface{}, 64)}
	go client.readLoop()
	return client
}

func (c *wsTestClient) readLoop() {
	defer close(c.msgs)
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		var packet []interface{}
		if err := json.Unmarshal(data, &packet); err != nil {
			continue
		}
		c.msgs <- packet
	}
}

func (c *wsTestClient) send(t *testing.T, packet []interface{}) {
	t.Helper()
	if err := c.conn.WriteJSON(packet); err != nil {
		t.Fatalf("write %v: %v", packet, err)
	}
}

//等某一条消息，队列里其他消息先跳过
func (c *wsTestClient) wait(t *testing.T, code int) []interface{} {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case packet, ok := <-c.msgs:
			if !ok {
				t.Fatalf("connection closed while waiting for message %d", code)
			}
			if len(packet) > 0 && packet[0] == float64(code) {
				return packet
			}
		case <-timeout:
			t.Fatalf("timed out waiting for message %d", code)
		}
	}
}

//入座消息里的座位数量
func seatCount(packet []interface{}) int {
	seats, ok := packet[2].([]interface{})
	if !ok {
		return -1
	}
	return len(seats)
}

//通过真实的websocket连上服务端，走一遍“加机器人补齐三个人”的流程
func TestAddRobotOverWebSocket(t *testing.T) {
	token, err := common.CreateSession(4242, "e2e-user")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", ServeWs)
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newWsTestClient(t, server, token)
	defer client.conn.Close()

	//进房间
	client.send(t, []interface{}{float64(common.ReqJoinRoom), float64(1)})
	client.wait(t, common.ResJoinRoom)

	//建一张桌子，只有自己一个人
	client.send(t, []interface{}{float64(common.ReqNewTable)})
	join := client.wait(t, common.ResJoinTable)
	if seats := seatCount(join); seats != 1 {
		t.Fatalf("table has %d seats, want 1: %v", seats, join[2])
	}

	//加第一个机器人
	client.send(t, []interface{}{float64(common.ReqAddRobot)})
	join = client.wait(t, common.ResJoinTable)
	if seats := seatCount(join); seats != 2 {
		t.Fatalf("table has %d seats after one robot, want 2: %v", seats, join[2])
	}
	answer := client.wait(t, common.ResAddRobot)
	if answer[1] != true {
		t.Fatalf("add robot refused: %v", answer[2])
	}

	//加第二个机器人，人齐了就该发牌
	client.send(t, []interface{}{float64(common.ReqAddRobot)})
	join = client.wait(t, common.ResJoinTable)
	if seats := seatCount(join); seats != 3 {
		t.Fatalf("table has %d seats after two robots, want 3: %v", seats, join[2])
	}
	deal := client.wait(t, common.ResDealPoker)
	cards, ok := deal[2].([]interface{})
	if !ok || len(cards) != 17 {
		t.Fatalf("deal message = %v, want 17 cards", deal[2])
	}
	answer = client.wait(t, common.ResAddRobot)
	if answer[1] != true {
		t.Fatalf("add robot refused: %v", answer[2])
	}

	//满座了不能再加
	client.send(t, []interface{}{float64(common.ReqAddRobot)})
	answer = client.wait(t, common.ResAddRobot)
	if answer[1] != false {
		t.Fatal("a full table accepted another robot")
	}
	if answer[2] == "" {
		t.Fatal("the refusal did not explain why")
	}
}
