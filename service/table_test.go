package service

import (
	"reflect"
	"testing"

	"landlord/common"
)

func TestResetDealsFreshHandsToEveryClient(t *testing.T) {
	clients := make([]*Client, 3)
	table := &Table{
		State:        GameEnd,
		TableClients: make(map[UserId]*Client, 3),
		GameManage:   &GameManage{Multiple: 1, LastShotPoker: make([]int, 0), Pokers: make([]int, 0, 54)},
	}
	for i := range clients {
		clients[i] = &Client{
			HandPokers: make([]int, 21),
			UserInfo:   &UserInfo{UserId: UserId(i + 1), Username: "robot"},
			IsRobot:    true,
			toRobot:    make(chan []interface{}, 8),
			Ready:      true,
		}
		table.TableClients[clients[i].UserInfo.UserId] = clients[i]
	}
	for i := range clients {
		clients[i].Next = clients[(i+1)%len(clients)]
	}
	table.Creator = clients[0]
	table.GameManage.FirstCallScore = clients[0]

	firstRound := dealRoundForTest(t, table, clients)
	if table.GameManage.FirstCallScore != clients[1] {
		t.Fatal("first caller did not rotate for the next round")
	}

	secondRound := dealRoundForTest(t, table, clients)
	if table.GameManage.FirstCallScore != clients[2] {
		t.Fatal("first caller did not rotate on the following round")
	}
	for i := range clients {
		if reflect.DeepEqual(firstRound[i], secondRound[i]) {
			t.Fatalf("client %d received the same hand in consecutive rounds", clients[i].UserInfo.UserId)
		}
		if clients[i].UserInfo.Role != RoleFarmer {
			t.Fatalf("client %d retained a non-farmer role after reset", clients[i].UserInfo.UserId)
		}
	}
}

func dealRoundForTest(t *testing.T, table *Table, clients []*Client) [][]int {
	t.Helper()
	for _, client := range clients {
		client.UserInfo.Role = RoleLandlord
	}
	table.reset()

	hands := make([][]int, len(clients))
	seen := make(map[int]bool, 54)
	for i, client := range clients {
		restart := <-client.toRobot
		if len(restart) < 1 || restart[0] != common.ResRestart {
			t.Fatalf("client %d did not receive restart before new deal: %v", client.UserInfo.UserId, restart)
		}
		deal := <-client.toRobot
		if len(deal) != 3 || deal[0] != common.ResDealPoker {
			t.Fatalf("client %d received unexpected deal message: %v", client.UserInfo.UserId, deal)
		}
		cards, ok := deal[2].([]int)
		if !ok || len(cards) != 17 {
			t.Fatalf("client %d received %d cards, want 17", client.UserInfo.UserId, len(cards))
		}
		hands[i] = append([]int(nil), cards...)
		for _, card := range cards {
			if seen[card] {
				t.Fatalf("card %d was dealt more than once", card)
			}
			seen[card] = true
		}
		if len(client.HandPokers) != 17 {
			t.Fatalf("client %d retained %d cards, want 17", client.UserInfo.UserId, len(client.HandPokers))
		}
		if client.Ready || client.IsCalled {
			t.Fatalf("client %d was not reset before dealing", client.UserInfo.UserId)
		}
	}
	if len(table.GameManage.Pokers) != 3 {
		t.Fatalf("undealt cards = %d, want 3", len(table.GameManage.Pokers))
	}
	for _, card := range table.GameManage.Pokers {
		if seen[card] {
			t.Fatalf("undealt card %d was also dealt", card)
		}
		seen[card] = true
	}
	if len(seen) != 54 {
		t.Fatalf("unique cards in round = %d, want 54", len(seen))
	}
	return hands
}
