package service

import (
	"testing"
	"time"

	"landlord/common"
)

// 造一张有 n 个假客户端（走 toRobot 通道收消息）的牌桌
func newTableWithChips(t *testing.T, n int) (*Table, *Room, []*Client) {
	t.Helper()
	room := &Room{RoomId: 9, AllowRobot: true, EntranceFee: 200, Tables: make(map[TableId]*Table)}
	table := &Table{
		TableId:      1,
		State:        GameEnd,
		TableClients: make(map[UserId]*Client, 3),
		GameManage:   &GameManage{Multiple: 1, LastShotPoker: make([]int, 0), Pokers: make([]int, 0, 54)},
	}
	room.Tables[table.TableId] = table

	clients := make([]*Client, n)
	for i := range clients {
		clients[i] = &Client{
			HandPokers: make([]int, 0, 21),
			UserInfo:   &UserInfo{UserId: UserId(i + 1), Username: "player"},
			Room:       room,
			Table:      table,
			msgBox:     make(chan []interface{}, 32),
		}
		table.TableClients[clients[i].UserInfo.UserId] = clients[i]
	}
	for i := range clients {
		clients[i].Next = clients[(i+1)%n]
	}
	table.Creator = clients[0]
	table.GameManage.FirstCallScore = clients[0]
	return table, room, clients
}

func drainMessages(c *Client) [][]interface{} {
	var msgs [][]interface{}
	for {
		select {
		case msg := <-c.msgBox:
			msgs = append(msgs, msg)
		default:
			return msgs
		}
	}
}

func findCode(msgs [][]interface{}, code int) []interface{} {
	for _, msg := range msgs {
		if len(msg) > 0 && msg[0] == code {
			return msg
		}
	}
	return nil
}

// 一局结束后点“再来一把”，不用等其他玩家点确认，直接发下一局
func TestRestartStartsNewRoundWithoutWaitingOtherReady(t *testing.T) {
	table, _, clients := newTableWithChips(t, 3)
	table.State = GameEnd
	clients[1].Ready = false
	clients[2].Ready = false

	wsRequest([]interface{}{float64(common.ReqRestart)}, clients[0])

	if table.State != GameCallScore {
		t.Fatalf("table state = %d after restart, want GameCallScore", table.State)
	}
	for _, client := range clients {
		msgs := drainMessages(client)
		restart := findCode(msgs, common.ResRestart)
		if restart == nil || restart[1] != true {
			t.Fatalf("client %d did not get a successful restart: %v", client.UserInfo.UserId, restart)
		}
		deal := findCode(msgs, common.ResDealPoker)
		if deal == nil {
			t.Fatalf("client %d was not dealt a new hand", client.UserInfo.UserId)
		}
		if cards, ok := deal[2].([]int); !ok || len(cards) != 17 {
			t.Fatalf("client %d got %v, want 17 cards", client.UserInfo.UserId, deal[2])
		}
	}
}

// 人不够时点“再来一把”要明确告诉客户端，而不是什么都不发
func TestRestartRefusedWithTwoPlayers(t *testing.T) {
	table, _, clients := newTableWithChips(t, 2)
	table.State = GameEnd

	wsRequest([]interface{}{float64(common.ReqRestart)}, clients[0])

	answer := findCode(drainMessages(clients[0]), common.ResRestart)
	if answer == nil || answer[1] != false || answer[2] == "" {
		t.Fatalf("restart answer = %v, want a refusal with a reason", answer)
	}
	if table.State != GameEnd {
		t.Fatalf("table state = %d, want GameEnd", table.State)
	}
}

// 一局结束后离开牌桌：座位被摘掉，剩下的玩家收到新的座位表
func TestLeaveTableAfterGameOver(t *testing.T) {
	table, room, clients := newTableWithChips(t, 3)
	table.State = GameEnd

	wsRequest([]interface{}{float64(common.ReqLeaveTable)}, clients[0])

	if _, ok := table.TableClients[clients[0].UserInfo.UserId]; ok {
		t.Fatal("the leaving player still occupies a seat")
	}
	if clients[0].Table != nil {
		t.Fatal("the leaving player still points at the table")
	}
	if len(table.TableClients) != 2 {
		t.Fatalf("table has %d players after leaving, want 2", len(table.TableClients))
	}
	if len(room.Tables) != 1 {
		t.Fatal("the table was removed from the room although players remain")
	}
	msgs := drainMessages(clients[0])
	answer := findCode(msgs, common.ResLeaveTable)
	if answer == nil || answer[1] != true {
		t.Fatalf("leave answer = %v", answer)
	}
	if findCode(msgs, common.ResTableList) == nil {
		t.Fatal("the leaving player did not receive the table list")
	}
	if sync := findCode(drainMessages(clients[1]), common.ResJoinTable); sync == nil || len(sync[2].([][2]interface{})) != 2 {
		t.Fatalf("remaining player did not see the updated seats: %v", sync)
	}
}

// 最后一个真人离开后，机器人和牌桌都要收掉，房间里不能留空桌
func TestLastHumanLeavingRemovesTableAndRobots(t *testing.T) {
	table, room, _ := newTableWithChips(t, 1)
	robot := &Client{
		Room:       room,
		HandPokers: make([]int, 0, 21),
		UserInfo:   &UserInfo{UserId: 99, Username: "ROBOT-9"},
		IsRobot:    true,
		toRobot:    make(chan []interface{}, 8),
		toServer:   make(chan []interface{}, 8),
	}
	table.TableClients[robot.UserInfo.UserId] = robot
	robot.Next = table.Creator
	table.Creator.Next = robot
	table.State = GameEnd
	human := table.TableClients[1]

	wsRequest([]interface{}{float64(common.ReqLeaveTable)}, human)

	if len(table.TableClients) != 0 {
		t.Fatalf("table still has %d players", len(table.TableClients))
	}
	if len(room.Tables) != 0 {
		t.Fatal("empty table was left in the room")
	}
}

// 牌局进行中不能把座位抽走
func TestLeaveTableRefusedWhilePlaying(t *testing.T) {
	table, _, clients := newTableWithChips(t, 3)
	table.State = GamePlaying

	if table.leaveTable(clients[0]) {
		t.Fatal("leaving was allowed in the middle of a hand")
	}
	if len(table.TableClients) != 3 {
		t.Fatalf("table has %d players, want 3", len(table.TableClients))
	}
}

// 不是自己回合的出牌请求不能把出牌权往后推
func TestShotOutOfTurnKeepsTurn(t *testing.T) {
	table, _, clients := newTableWithChips(t, 3)
	table.State = GamePlaying
	table.GameManage.Turn = clients[1]
	for i := range clients {
		clients[i].HandPokers = []int{i, i + 13, i + 26}
	}

	wsRequest([]interface{}{float64(common.ReqShotPoker), []interface{}{float64(clients[0].HandPokers[0])}}, clients[0])

	if table.GameManage.Turn != clients[1] {
		t.Fatalf("turn moved to %v after an out-of-turn request", table.GameManage.Turn.UserInfo.UserId)
	}
	if len(clients[0].HandPokers) != 3 {
		t.Fatal("out-of-turn request removed a card from the hand")
	}
	if msg := findCode(drainMessages(clients[0]), common.ResShotPoker); msg != nil {
		t.Fatalf("out-of-turn request was broadcast: %v", msg)
	}
}

// 不能给满桌再加人
func TestJoinFullTableRefused(t *testing.T) {
	table, _, _ := newTableWithChips(t, 3)
	extra := &Client{
		HandPokers: make([]int, 0, 21),
		UserInfo:   &UserInfo{UserId: 77, Username: "late"},
		msgBox:     make(chan []interface{}, 32),
	}

	if table.joinTable(extra) {
		t.Fatal("a full table accepted another player")
	}
	if len(table.TableClients) != 3 {
		t.Fatalf("table has %d players, want 3", len(table.TableClients))
	}
}

// 加入牌桌成功后只回座位表，不再推一份可选牌桌列表（那是“选择房间”浮层的来源）
func TestJoinTableDoesNotSendTableList(t *testing.T) {
	table, _, _ := newTableWithChips(t, 2)
	joiner := &Client{
		HandPokers: make([]int, 0, 21),
		UserInfo:   &UserInfo{UserId: 42, Username: "joiner"},
		Room:       table.TableClients[1].Room,
		msgBox:     make(chan []interface{}, 32),
	}

	wsRequest([]interface{}{float64(common.ReqJoinTable), float64(table.TableId)}, joiner)

	msgs := drainMessages(joiner)
	if msg := findCode(msgs, common.ResTableList); msg != nil {
		t.Fatalf("joining a table sent the table list again: %v", msg)
	}
	if findCode(msgs, common.ResJoinTable) == nil {
		t.Fatal("joining a table did not send the seat list")
	}
}

// 加入失败（比如桌号不存在）才需要刷新列表
func TestJoinMissingTableReturnsList(t *testing.T) {
	_, room, clients := newTableWithChips(t, 2)
	joiner := &Client{
		HandPokers: make([]int, 0, 21),
		UserInfo:   &UserInfo{UserId: 43, Username: "joiner"},
		Room:       room,
		msgBox:     make(chan []interface{}, 32),
	}

	wsRequest([]interface{}{float64(common.ReqJoinTable), float64(999)}, joiner)

	msg := findCode(drainMessages(joiner), common.ResTableList)
	if msg == nil {
		t.Fatal("a failed join did not refresh the table list")
	}
	if seats, ok := msg[1].([][2]int); !ok || len(seats) != 1 {
		t.Fatalf("table list = %v, want the existing table", msg[1])
	}
	_ = clients
}

// 座位满了再请求加入，客户端要拿到最新的牌桌列表
func TestJoinFullTableReturnsList(t *testing.T) {
	table, _, _ := newTableWithChips(t, 3)
	joiner := &Client{
		HandPokers: make([]int, 0, 21),
		UserInfo:   &UserInfo{UserId: 44, Username: "joiner"},
		Room:       table.TableClients[UserId(1)].Room,
		msgBox:     make(chan []interface{}, 32),
	}

	wsRequest([]interface{}{float64(common.ReqJoinTable), float64(table.TableId)}, joiner)

	msg := findCode(drainMessages(joiner), common.ResTableList)
	if msg == nil {
		t.Fatal("a refused join did not refresh the table list")
	}
	if seats, ok := msg[1].([][2]int); !ok || len(seats) != 0 {
		t.Fatalf("table list = %v, want no joinable table", msg[1])
	}
}

// 牌桌持锁给机器人发消息时不能因为队列满而阻塞（否则和机器人抢锁互等）
func TestRobotInboxDoesNotBlockTableSends(t *testing.T) {
	robot := &Client{
		UserInfo: &UserInfo{UserId: 1, Username: "ROBOT"},
		IsRobot:  true,
		inbox:    newRobotInbox(),
	}
	done := make(chan struct{})
	go func() {
		for i := 0; i < 200; i++ {
			robot.sendMsg([]interface{}{common.ResShotPoker, 1, []int{}})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("sending to a robot blocked while the mailbox was full")
	}
	for i := 0; i < 200; i++ {
		if _, ok := robot.inbox.pop(); !ok {
			t.Fatalf("message %d is missing", i)
		}
	}
	robot.inbox.close()
	if _, ok := robot.inbox.pop(); ok {
		t.Fatal("closed inbox still hands out messages")
	}
}

// 创建牌桌的人离开后，出牌顺序和房主身份要交给剩下的人
func TestCreatorLeavingHandsTableToRemainingPlayer(t *testing.T) {
	table, _, clients := newTableWithChips(t, 3)
	table.State = GameEnd

	wsRequest([]interface{}{float64(common.ReqLeaveTable)}, clients[0])

	if table.Creator != clients[1] {
		t.Fatalf("creator = %v after the owner left, want %v", table.Creator, clients[1])
	}
	if clients[2].Next != clients[1] || clients[1].Next != clients[2] {
		t.Fatal("turn chain is broken after the creator left")
	}
}
