package service

import (
	"math/rand"
	"testing"

	"landlord/common"
)

//魔鬼难度：农民这一侧启用预判，地主这一侧不启用
func TestDevilLookaheadOnlyForFarmers(t *testing.T) {
	_, _, clients := newTableWithChips(t, 3)
	farmer, landlord := clients[0], clients[1]
	farmer.BotLevel = botLevelDevil
	landlord.BotLevel = botLevelDevil

	farmer.UserInfo.Role = RoleFarmer
	landlord.UserInfo.Role = RoleLandlord
	if !farmer.botLookahead() {
		t.Fatal("the devil farmer does not use the lookahead")
	}
	if landlord.botLookahead() {
		t.Fatal("the devil landlord should not use the lookahead")
	}
}

//对手只剩一张牌、而且能压住我时，不能把这张牌送出去
func TestDevilFarmerDoesNotFeedTheLandlord(t *testing.T) {
	_, _, clients := newTableWithChips(t, 3)
	farmer, landlord := clients[0], clients[1]
	farmer.BotLevel = botLevelDevil
	farmer.UserInfo.Role = RoleFarmer
	landlord.UserInfo.Role = RoleLandlord
	farmer.HandPokers = botCards("678")
	landlord.HandPokers = botCards("9")

	//地主出 3，农民手里最小是 6：压过去地主用 9 走完，不能压
	if shot := farmer.botDecideFollow(botCards("3"), landlord); len(shot) != 0 {
		t.Fatalf("the devil farmer played %v, letting the landlord go out", shot)
	}
}

//对手压不住我时，就该果断压上去
func TestDevilFarmerBlocksTheLandlord(t *testing.T) {
	_, _, clients := newTableWithChips(t, 3)
	farmer, landlord := clients[0], clients[1]
	farmer.BotLevel = botLevelDevil
	farmer.UserInfo.Role = RoleFarmer
	landlord.UserInfo.Role = RoleLandlord
	farmer.HandPokers = botCards("678")
	landlord.HandPokers = botCards("5")

	shot := farmer.botDecideFollow(botCards("3"), landlord)
	if len(shot) != 1 {
		t.Fatalf("the devil farmer passed with %v, want a single to block the landlord", shot)
	}
	if rank := botRank(shot[0]); rank != botRank(botCards("6")[0]) {
		t.Fatalf("the devil farmer blocked with rank %d, want the cheapest 6", rank)
	}
}

//新拆手估算不能比老贪心更差
func TestSmartCounterIsNotWorseThanGreedy(t *testing.T) {
	random := rand.New(rand.NewSource(31))
	counter := botSmartCounter{}
	for trial := 0; trial < 200; trial++ {
		deck := make([]int, 54)
		for i := range deck {
			deck[i] = i
		}
		random.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
		hand := deck[:17]
		if smart, greedy := counter.handsLeft(hand), botHandsLeft(hand); smart > greedy {
			t.Fatalf("trial %d: smart estimate %d is worse than greedy %d", trial, smart, greedy)
		}
	}
}

//简单难度也要打出合法牌
func TestEasyBotAlwaysPlaysLegalCards(t *testing.T) {
	_, _, clients := newTableWithChips(t, 3)
	bot := clients[0]
	bot.BotLevel = botLevelEasy
	bot.HandPokers = botCards("3345679")

	shot := bot.botDecideLead()
	if _, ok := botPlayOf(shot); !ok {
		t.Fatalf("the easy bot led an illegal play: %v", shot)
	}
}

//三个魔鬼机器人要把一局打完，且不能出非法牌
func TestDevilBotsFinishAFullGame(t *testing.T) {
	table, _, clients := newTableWithChips(t, 3)
	for _, client := range clients {
		client.BotLevel = botLevelDevil
	}
	table.GameManage.FirstCallScore = clients[0]
	table.State = GameCallScore
	table.dealPoker()
	table.GameManage.MaxCallScore = 3
	table.GameManage.MaxCallScoreTurn = clients[0]
	table.callEnd()
	for _, client := range clients {
		drainMessages(client)
	}

	moves := 0
	for table.State == GamePlaying {
		if moves++; moves > 400 {
			t.Fatal("the devil bots could not finish the game in 400 moves")
		}
		turn := table.GameManage.Turn
		if turn == nil {
			t.Fatal("nobody's turn in the middle of a game")
		}
		var shot []int
		if len(table.GameManage.LastShotPoker) == 0 || table.GameManage.LastShotClient == turn {
			shot = turn.botDecideLead()
		} else {
			shot = turn.botDecideFollow(table.GameManage.LastShotPoker, table.GameManage.LastShotClient)
		}
		if len(shot) > 0 {
			if _, ok := botPlayOf(shot); !ok {
				t.Fatalf("the devil bot decided an illegal play: %v (hand %v)", shot, turn.HandPokers)
			}
		}
		request := []interface{}{float64(common.ReqShotPoker)}
		cards := make([]interface{}, 0, len(shot))
		for _, card := range shot {
			cards = append(cards, float64(card))
		}
		request = append(request, cards)
		before := len(turn.HandPokers)
		wsRequest(request, turn)
		if table.State != GameEnd && len(shot) > 0 && len(turn.HandPokers) == before {
			t.Fatalf("the server rejected the devil bot's play %v (last shot %v)", shot, table.GameManage.LastShotPoker)
		}
		for _, client := range clients {
			drainMessages(client)
		}
	}
	if table.State != GameEnd {
		t.Fatalf("the game ended in state %d, want GameEnd", table.State)
	}
}

//机器人走了一个再补一个，不能重名
func TestRobotNamesDoNotRepeat(t *testing.T) {
	room := newTestRoom()
	human := newFakeHuman(1, "human", room)
	table := room.newTable(human)
	table.joinTable(human)
	for i := 0; i < 2; i++ {
		if ok, reason := table.addRobotByUser(human, room); !ok {
			t.Fatalf("add robot refused: %s", reason)
		}
	}
	//一局结束，第一个机器人走了，再补一个
	table.State = GameEnd
	table.removeClient(table.robots()[0])
	if ok, reason := table.addRobotByUser(human, room); !ok {
		t.Fatalf("add robot refused: %s", reason)
	}
	names := map[string]bool{}
	for _, robot := range table.robots() {
		if names[robot.UserInfo.Username] {
			t.Fatalf("two robots are both named %s", robot.UserInfo.Username)
		}
		names[robot.UserInfo.Username] = true
	}
}

//机器人的 id 不能和真人账号撞号
func TestRobotIDsDoNotCollideWithHumanIDs(t *testing.T) {
	room := newTestRoom()
	human := newFakeHuman(7, "human", room)
	table := room.newTable(human)
	table.joinTable(human)
	if ok, reason := table.addRobotByUser(human, room); !ok {
		t.Fatalf("add robot refused: %s", reason)
	}
	for _, robot := range table.robots() {
		if robot.UserInfo.UserId >= 0 {
			t.Fatalf("the robot got the id %d, which can collide with a human account", robot.UserInfo.UserId)
		}
	}
}
