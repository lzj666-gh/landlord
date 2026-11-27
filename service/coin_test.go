package service

import (
	"testing"

	"landlord/common"
)

// 地主赢：地主收两份底注，两个农民各付一份
func TestGameOverLandlordWinSettlesCoins(t *testing.T) {
	table, room, clients := newTableWithChips(t, 3)
	room.EntranceFee = 500
	table.GameManage.MaxCallScore = 2
	table.GameManage.Multiple = 4 // 炸弹翻倍
	clients[0].UserInfo.Role = RoleLandlord
	clients[1].UserInfo.Role = RoleFarmer
	clients[2].UserInfo.Role = RoleFarmer
	for _, c := range clients {
		c.UserInfo.Coin = 10000
	}
	table.State = GamePlaying

	table.gameOver(clients[0])

	want := map[UserId][2]int{
		clients[0].UserInfo.UserId: {8000, 18000},
		clients[1].UserInfo.UserId: {-4000, 6000},
		clients[2].UserInfo.UserId: {-4000, 6000},
	}
	for _, c := range clients {
		msg := findCode(drainMessages(c), common.ResGameOver)
		if msg == nil {
			t.Fatalf("client %d got no game over message", c.UserInfo.UserId)
		}
		delta, coin := msg[2].(int), msg[5].(int)
		if got := want[c.UserInfo.UserId]; delta != got[0] || coin != got[1] {
			t.Fatalf("client %d settlement = delta %d coin %d, want %v", c.UserInfo.UserId, delta, coin, got)
		}
		if base := msg[6].(int); base != 4000 {
			t.Fatalf("client %d base = %d, want 4000", c.UserInfo.UserId, base)
		}
		if multiple := msg[7].(int); multiple != 4 {
			t.Fatalf("client %d multiple = %d, want 4", c.UserInfo.UserId, multiple)
		}
		if refill := msg[8].(int); refill != 0 {
			t.Fatalf("client %d got unexpected refill %d", c.UserInfo.UserId, refill)
		}
	}
}

// 农民赢：两个农民各收一份，地主付两份
func TestGameOverFarmerWinSettlesCoins(t *testing.T) {
	table, room, clients := newTableWithChips(t, 3)
	room.EntranceFee = 500
	table.GameManage.MaxCallScore = 1
	table.GameManage.Multiple = 1
	clients[0].UserInfo.Role = RoleLandlord
	clients[1].UserInfo.Role = RoleFarmer
	clients[2].UserInfo.Role = RoleFarmer
	for _, c := range clients {
		c.UserInfo.Coin = 10000
	}
	table.State = GamePlaying

	table.gameOver(clients[1])

	want := map[UserId][2]int{
		clients[0].UserInfo.UserId: {-1000, 9000},
		clients[1].UserInfo.UserId: {500, 10500},
		clients[2].UserInfo.UserId: {500, 10500},
	}
	for _, c := range clients {
		msg := findCode(drainMessages(c), common.ResGameOver)
		if msg == nil {
			t.Fatalf("client %d got no game over message", c.UserInfo.UserId)
		}
		delta, coin := msg[2].(int), msg[5].(int)
		if got := want[c.UserInfo.UserId]; delta != got[0] || coin != got[1] {
			t.Fatalf("client %d settlement = delta %d coin %d, want %v", c.UserInfo.UserId, delta, coin, got)
		}
	}
}

// 输光后金币低于 1000：补到 1000，并在结算消息里带上补足金额
func TestGameOverRefillsBelowMinimum(t *testing.T) {
	table, room, clients := newTableWithChips(t, 3)
	room.EntranceFee = 500
	table.GameManage.MaxCallScore = 1
	table.GameManage.Multiple = 1
	clients[0].UserInfo.Role = RoleLandlord
	clients[1].UserInfo.Role = RoleFarmer
	clients[2].UserInfo.Role = RoleFarmer
	clients[0].UserInfo.Coin = 10000
	clients[1].UserInfo.Coin = 300 // 不够付 500
	clients[2].UserInfo.Coin = 10000
	table.State = GamePlaying

	table.gameOver(clients[0])

	msg := findCode(drainMessages(clients[1]), common.ResGameOver)
	if msg == nil {
		t.Fatal("broke player got no game over message")
	}
	if coin := msg[5].(int); coin != common.MinCoin {
		t.Fatalf("coin after refill = %d, want %d", coin, common.MinCoin)
	}
	if refill := msg[8].(int); refill != common.MinCoin {
		t.Fatalf("refill = %d, want %d", refill, common.MinCoin)
	}
	if clients[1].UserInfo.Coin != common.MinCoin {
		t.Fatalf("player coin = %d, want %d", clients[1].UserInfo.Coin, common.MinCoin)
	}
}

// 结算只发生一次：牌局结束后状态变为 GameEnd，再来一次不会重复扣钱
func TestGameOverIsIdempotentByState(t *testing.T) {
	table, room, clients := newTableWithChips(t, 3)
	room.EntranceFee = 500
	table.GameManage.MaxCallScore = 1
	table.GameManage.Multiple = 1
	clients[0].UserInfo.Role = RoleLandlord
	clients[1].UserInfo.Role = RoleFarmer
	clients[2].UserInfo.Role = RoleFarmer
	for _, c := range clients {
		c.UserInfo.Coin = 10000
	}
	table.State = GamePlaying

	table.gameOver(clients[0])
	table.gameOver(clients[0]) // 重复调用不能二次结算
	if table.State != GameEnd {
		t.Fatalf("table state = %d, want GameEnd", table.State)
	}
	if coin := clients[0].UserInfo.Coin; coin != 11000 {
		t.Fatalf("landlord coin = %d, want 11000", coin)
	}
}
