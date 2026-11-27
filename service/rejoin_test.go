package service

import (
	"reflect"
	"testing"

	"landlord/common"
)

// newChipClient 生成一个用 toRobot 通道收消息的假客户端，便于断言服务端下发的消息。
func newChipClient(uid UserId, name string) *Client {
	return &Client{
		HandPokers: make([]int, 0, 21),
		UserInfo:   &UserInfo{UserId: uid, Username: name},
		IsRobot:    true,
		toRobot:    make(chan []interface{}, 16),
		Ready:      true,
	}
}

// midHandTable 造一张已经发完牌、地主刚拿到三张底牌（共 20 张）的牌桌。
func midHandTable(t *testing.T) (*Table, []*Client) {
	t.Helper()
	clients := make([]*Client, 3)
	table := &Table{
		TableId:      1,
		State:        GameEnd,
		TableClients: make(map[UserId]*Client, 3),
		GameManage:   &GameManage{Multiple: 1, LastShotPoker: make([]int, 0), Pokers: make([]int, 0, 54)},
	}
	for i := range clients {
		clients[i] = newChipClient(UserId(i+1), "player")
		clients[i].Table = table
		table.TableClients[clients[i].UserInfo.UserId] = clients[i]
	}
	for i := range clients {
		clients[i].Next = clients[(i+1)%len(clients)]
	}
	table.Creator = clients[0]
	table.GameManage.FirstCallScore = clients[0]

	table.reset()
	for _, client := range clients {
		drainChipMessages(client)
	}

	// 叫分结束：地主拿走三张底牌，进入出牌阶段
	table.State = GamePlaying
	table.GameManage.MaxCallScoreTurn = clients[0]
	table.GameManage.Turn = clients[0]
	table.GameManage.LastShotClient = clients[1]
	table.GameManage.LastShotPoker = []int{clients[1].HandPokers[0]}
	for _, poker := range append([]int(nil), table.GameManage.Pokers...) {
		clients[0].HandPokers = append(clients[0].HandPokers, poker)
	}
	table.GameManage.Pokers = table.GameManage.Pokers[:0]
	if len(clients[0].HandPokers) != 20 {
		t.Fatalf("landlord holds %d cards, want 20", len(clients[0].HandPokers))
	}
	return table, clients
}

func drainChipMessages(c *Client) [][]interface{} {
	var msgs [][]interface{}
	for {
		select {
		case msg := <-c.toRobot:
			msgs = append(msgs, msg)
		default:
			return msgs
		}
	}
}

// 断线重连（同一账号再次加入）不能重新发牌：其他两家手里还是这局的牌，再发一次就会叠成 34~37 张。
func TestRejoinKeepsRunningHand(t *testing.T) {
	table, clients := midHandTable(t)
	old := clients[0]
	old.IsRobot = false // 真人：断线时会走 close()

	landlordHand := append([]int(nil), old.HandPokers...)
	farmerHands := [][]int{append([]int(nil), clients[1].HandPokers...), append([]int(nil), clients[2].HandPokers...)}

	old.close()
	if _, ok := table.TableClients[old.UserInfo.UserId]; !ok {
		t.Fatal("seat was dropped while the hand is still running")
	}

	rejoin := newChipClient(old.UserInfo.UserId, "player")
	table.joinTable(rejoin)

	if len(table.TableClients) != 3 {
		t.Fatalf("table has %d clients after rejoin, want 3", len(table.TableClients))
	}
	if table.TableClients[old.UserInfo.UserId] != rejoin {
		t.Fatal("rejoin did not take over the original seat")
	}
	if !reflect.DeepEqual(rejoin.HandPokers, landlordHand) {
		t.Fatalf("reconnected player got %d cards %v, want the running hand %v", len(rejoin.HandPokers), rejoin.HandPokers, landlordHand)
	}
	for i, want := range farmerHands {
		if !reflect.DeepEqual(clients[i+1].HandPokers, want) {
			t.Fatalf("farmer %d hand was redealt: %d cards %v, want %v", i, len(clients[i+1].HandPokers), clients[i+1].HandPokers, want)
		}
		for _, msg := range drainChipMessages(clients[i+1]) {
			if len(msg) > 0 && msg[0] == common.ResDealPoker {
				t.Fatalf("farmer %d was dealt a new hand after rejoin: %v", i, msg)
			}
		}
	}
	if table.State != GamePlaying {
		t.Fatalf("table state = %d after rejoin, want GamePlaying", table.State)
	}
	if table.GameManage.Turn != rejoin || table.GameManage.MaxCallScoreTurn != rejoin ||
		table.GameManage.FirstCallScore != clients[1] || table.GameManage.LastShotClient != clients[1] {
		t.Fatal("game pointers were not moved to the reconnected client")
	}
	if rejoin.Next != clients[1] || clients[2].Next != rejoin {
		t.Fatal("turn chain is broken after rejoin")
	}

	var resume []interface{}
	for _, msg := range drainChipMessages(rejoin) {
		if len(msg) > 0 && msg[0] == common.ResDealPoker {
			if resume != nil {
				t.Fatalf("reconnected player received more than one deal message: %v", msg)
			}
			resume = msg
		}
	}
	if resume == nil {
		t.Fatal("reconnected player did not receive the resume message")
	}
	if len(resume) < 4 || resume[0] != common.ResDealPoker || resume[3] != GamePlaying {
		t.Fatalf("resume message = %v, want deal message carrying the playing state", resume)
	}
	cards, ok := resume[2].([]int)
	if !ok || !reflect.DeepEqual(cards, landlordHand) && !reflect.DeepEqual(sortedInts(cards), sortedInts(landlordHand)) {
		t.Fatalf("resume message hand = %v, want %v", resume[2], landlordHand)
	}
}

// 断线后旧连接迟到的 close() 不能把已经接管的座位再删掉。
func TestLateCloseKeepsReconnectedSeat(t *testing.T) {
	table, clients := midHandTable(t)
	old := clients[0]
	old.IsRobot = false
	old.close()

	rejoin := newChipClient(old.UserInfo.UserId, "player")
	table.joinTable(rejoin)

	old.close()
	if table.TableClients[old.UserInfo.UserId] != rejoin {
		t.Fatal("late close from the stale connection dropped the reconnected seat")
	}
	if len(table.TableClients) != 3 {
		t.Fatalf("table has %d clients, want 3", len(table.TableClients))
	}
}

func sortedInts(in []int) []int {
	out := append([]int(nil), in...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
