package service

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"landlord/common"
)

//端到端：真人 + 两个魔鬼机器人，通过真实 websocket 把一局牌打起来。
//机器人会自己叫分、自己出牌，这条链路走的是和生产完全一样的代码路径。
func TestDevilBotsPlayARoundOverWebSocket(t *testing.T) {
	token, err := common.CreateSession(9001, "devil-e2e")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", ServeWs)
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newWsTestClient(t, server, token)
	defer client.conn.Close()

	client.send(t, []interface{}{float64(common.ReqJoinRoom), float64(1)})
	client.wait(t, common.ResJoinRoom)
	client.send(t, []interface{}{float64(common.ReqNewTable)})
	client.wait(t, common.ResJoinTable)

	//补两个机器人。服务端下发的顺序是：入座 -> （满三人时）发牌 -> 结果
	client.send(t, []interface{}{float64(common.ReqAddRobot)})
	client.wait(t, common.ResJoinTable)
	client.wait(t, common.ResAddRobot)
	client.send(t, []interface{}{float64(common.ReqAddRobot)})
	client.wait(t, common.ResJoinTable)
	deal := client.wait(t, common.ResDealPoker)
	answer := client.wait(t, common.ResAddRobot)
	if answer[1] != true {
		t.Fatalf("add robot refused: %v", answer[2])
	}
	cards, ok := deal[2].([]interface{})
	if !ok || len(cards) != 17 {
		t.Fatalf("deal message = %v, want 17 cards", deal[2])
	}

	//我先叫一分，机器人接着叫
	client.send(t, []interface{}{float64(common.ReqCallScore), float64(1)})
	show := client.wait(t, common.ResShowPoker)
	landlord, ok := show[1].(float64)
	if !ok {
		t.Fatalf("show poker message = %v, want the landlord id", show[1])
	}

	//轮到地主先出牌：地主是机器人就等它出，地主是我就过一手
	if int(landlord) == 9001 {
		client.send(t, []interface{}{float64(common.ReqShotPoker), []interface{}{}})
	}

	//等一个机器人真的出牌（机器人自己叫分、自己出牌，这条链路和生产完全一样）
	deadline := time.After(15 * time.Second)
	botCalled := false
	for {
		select {
		case packet := <-client.msgs:
			if len(packet) == 0 {
				continue
			}
			switch packet[0] {
			case float64(common.ResCallScore):
				if player, ok := packet[1].(float64); ok && int(player) != 9001 {
					botCalled = true
				}
			case float64(common.ResShotPoker):
				if player, ok := packet[1].(float64); ok && int(player) != 9001 {
					if !botCalled {
						t.Log("a bot played before calling score, the landlord may lead directly")
					}
					return
				}
			}
		case <-deadline:
			t.Fatal("no bot played a card in 15 seconds")
		}
	}
}
