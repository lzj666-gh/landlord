package service

import (
	"testing"

	"landlord/common"
)

//测试用的真人客户端：没有真实连接，sendMsg 会直接跳过
func newFakeHuman(uid UserId, name string, room *Room) *Client {
	return &Client{
		Room:       room,
		HandPokers: make([]int, 0, 21),
		UserInfo:   &UserInfo{UserId: uid, Username: name},
		toRobot:    make(chan []interface{}, 16),
		Ready:      true,
	}
}

func newTestRoom() *Room {
	return &Room{
		RoomId:      1,
		AllowRobot:  true,
		EntranceFee: 200,
		Tables:      make(map[TableId]*Table),
	}
}

//人数没满时，玩家可以手动加机器人补齐三个人
func TestAddRobotFillsEmptySeat(t *testing.T) {
	room := newTestRoom()
	human := newFakeHuman(1, "human", room)
	table := room.newTable(human)
	table.joinTable(human)

	ok, reason := table.addRobotByUser(human, room)
	if !ok {
		t.Fatalf("add robot refused: %s", reason)
	}
	if len(table.TableClients) != 2 {
		t.Fatalf("table has %d clients, want 2", len(table.TableClients))
	}
	if table.State != GameWaitting {
		t.Fatalf("table state = %d, want GameWaitting", table.State)
	}
	robots := table.robots()
	if len(robots) != 1 {
		t.Fatalf("table has %d robots, want 1", len(robots))
	}
	robot := robots[0]
	if !robot.IsRobot {
		t.Fatal("the new client is not a robot")
	}
	if human.Next != robot || robot.Next != human {
		t.Fatal("the robot was not linked into the turn order")
	}
	if table.Creator != human {
		t.Fatal("the robot took over the table creator")
	}
}

//人齐了才发牌，三个人都拿到 17 张牌
func TestAddRobotStartsRoundWhenTableFills(t *testing.T) {
	room := newTestRoom()
	first := newFakeHuman(1, "first", room)
	second := newFakeHuman(2, "second", room)
	table := room.newTable(first)
	table.joinTable(first)
	table.joinTable(second)

	if table.State != GameWaitting {
		t.Fatalf("table state = %d before the robot joined, want GameWaitting", table.State)
	}
	ok, reason := table.addRobotByUser(first, room)
	if !ok {
		t.Fatalf("add robot refused: %s", reason)
	}
	if len(table.TableClients) != 3 {
		t.Fatalf("table has %d clients, want 3", len(table.TableClients))
	}
	if table.State != GameCallScore {
		t.Fatalf("table state = %d after the robot joined, want GameCallScore", table.State)
	}
	for _, client := range table.TableClients {
		if len(client.HandPokers) != 17 {
			t.Fatalf("client %d holds %d cards, want 17", client.UserInfo.UserId, len(client.HandPokers))
		}
	}
	if len(table.GameManage.Pokers) != 3 {
		t.Fatalf("bottom cards = %d, want 3", len(table.GameManage.Pokers))
	}
	robot := table.robots()[0]
	if len(robot.HandPokers) != 17 {
		t.Fatalf("the robot holds %d cards, want 17", len(robot.HandPokers))
	}
	if robot.UserInfo.Username == "" {
		t.Fatal("the robot has no name")
	}
}

//桌子满了不能再加机器人
func TestAddRobotRefusedWhenTableFull(t *testing.T) {
	room := newTestRoom()
	first := newFakeHuman(1, "first", room)
	second := newFakeHuman(2, "second", room)
	third := newFakeHuman(3, "third", room)
	table := room.newTable(first)
	table.joinTable(first)
	table.joinTable(second)
	table.joinTable(third)

	ok, reason := table.addRobotByUser(first, room)
	if ok {
		t.Fatal("a full table accepted another robot")
	}
	if reason == "" {
		t.Fatal("the refusal did not explain why")
	}
	if len(table.TableClients) != 3 {
		t.Fatalf("table has %d clients, want 3", len(table.TableClients))
	}
}

//牌局进行中不能加机器人
func TestAddRobotRefusedWhilePlaying(t *testing.T) {
	room := newTestRoom()
	human := newFakeHuman(1, "human", room)
	table := room.newTable(human)
	table.joinTable(human)
	table.State = GamePlaying

	ok, _ := table.addRobotByUser(human, room)
	if ok {
		t.Fatal("a running game accepted another robot")
	}
	if len(table.TableClients) != 1 {
		t.Fatalf("table has %d clients, want 1", len(table.TableClients))
	}
}

//一局结束后人齐了，直接开新一局
func TestAddRobotAfterGameOverStartsNewRound(t *testing.T) {
	room := newTestRoom()
	first := newFakeHuman(1, "first", room)
	second := newFakeHuman(2, "second", room)
	table := room.newTable(first)
	table.joinTable(first)
	table.joinTable(second)
	table.State = GameEnd
	table.GameManage.MaxCallScore = 3
	table.GameManage.MaxCallScoreTurn = first

	ok, reason := table.addRobotByUser(first, room)
	if !ok {
		t.Fatalf("add robot refused: %s", reason)
	}
	if len(table.TableClients) != 3 {
		t.Fatalf("table has %d clients, want 3", len(table.TableClients))
	}
	if table.State != GameCallScore {
		t.Fatalf("table state = %d, want GameCallScore", table.State)
	}
	for _, client := range table.TableClients {
		if len(client.HandPokers) != 17 {
			t.Fatalf("client %d holds %d cards, want 17", client.UserInfo.UserId, len(client.HandPokers))
		}
		if client.Ready {
			t.Fatalf("client %d was not reset for the new round", client.UserInfo.UserId)
		}
	}
	if table.GameManage.MaxCallScore != 0 {
		t.Fatalf("max call score = %d, want 0", table.GameManage.MaxCallScore)
	}
}

//真人都走了，机器人和桌子一起收掉，房间里不留空桌
func TestRobotsLeaveWithTheLastHuman(t *testing.T) {
	room := newTestRoom()
	human := newFakeHuman(1, "human", room)
	table := room.newTable(human)
	table.joinTable(human)
	if ok, reason := table.addRobotByUser(human, room); !ok {
		t.Fatalf("add robot refused: %s", reason)
	}
	if len(table.TableClients) != 2 {
		t.Fatalf("table has %d clients, want 2", len(table.TableClients))
	}

	human.close()
	if len(table.TableClients) != 0 {
		t.Fatalf("table still has %d clients after the human left", len(table.TableClients))
	}
	if _, ok := room.Tables[table.TableId]; ok {
		t.Fatal("the empty table was left in the room")
	}
}

//牌局进行中真人掉线要保留座位等重连，机器人陪着一起等
func TestRobotsStayWhileTheHumanMayReconnect(t *testing.T) {
	room := newTestRoom()
	human := newFakeHuman(1, "human", room)
	table := room.newTable(human)
	table.joinTable(human)
	for i := 0; i < 2; i++ {
		if ok, reason := table.addRobotByUser(human, room); !ok {
			t.Fatalf("add robot refused: %s", reason)
		}
	}
	if len(table.TableClients) != 3 {
		t.Fatalf("table has %d clients, want 3", len(table.TableClients))
	}

	human.close()
	if !human.Offline {
		t.Fatal("the disconnected player lost the seat reserved for reconnection")
	}
	if len(table.TableClients) != 3 {
		t.Fatalf("table has %d clients after the disconnect, want 3", len(table.TableClients))
	}
	if len(table.robots()) != 2 {
		t.Fatalf("table has %d robots after the disconnect, want 2", len(table.robots()))
	}
}

//不是桌上有的人不能替别人加机器人
func TestAddRobotRefusedForOutsider(t *testing.T) {
	room := newTestRoom()
	human := newFakeHuman(1, "human", room)
	outsider := newFakeHuman(9, "outsider", room)
	table := room.newTable(human)
	table.joinTable(human)

	ok, _ := table.addRobotByUser(outsider, room)
	if ok {
		t.Fatal("a client outside the table added a robot")
	}
	if len(table.TableClients) != 1 {
		t.Fatalf("table has %d clients, want 1", len(table.TableClients))
	}
}

//前端请求加机器人时，服务端把结果回给请求的人
func TestRequestAddRobotAnswersTheClient(t *testing.T) {
	room := newTestRoom()
	human := newFakeHuman(1, "human", room)
	table := room.newTable(human)
	table.joinTable(human)

	wsRequest([]interface{}{float64(common.ReqAddRobot)}, human)
	if len(table.TableClients) != 2 {
		t.Fatalf("table has %d clients after the request, want 2", len(table.TableClients))
	}
	if len(table.robots()) != 1 {
		t.Fatalf("table has %d robots after the request, want 1", len(table.robots()))
	}
}
