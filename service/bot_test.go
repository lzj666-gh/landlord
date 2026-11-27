package service

import (
	"math/rand"
	"testing"

	"landlord/common"
)

//把 "3345667wW" 这样的点数串变成具体的牌号
func botCards(ranks string) []int {
	used := make(map[byte]int, 4)
	cards := make([]int, 0, len(ranks))
	for i := 0; i < len(ranks); i++ {
		card := ranks[i]
		candidates := common.ToPoker(card)
		index := used[card]
		if index >= len(candidates) {
			index = 0
		}
		used[card] = index + 1
		cards = append(cards, candidates[index])
	}
	return cards
}

func botPlayOf(cards []int) (botPlay, bool) {
	return botNewPlay(cards)
}

//机器人枚举出来的每一手牌都必须是合法的、而且确实在手里
func TestBotPlaysAreLegalAndInHand(t *testing.T) {
	random := rand.New(rand.NewSource(7))
	for trial := 0; trial < 40; trial++ {
		deck := make([]int, 54)
		for i := range deck {
			deck[i] = i
		}
		random.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
		hand := append([]int(nil), deck[:20]...)

		plays := botPlays(hand)
		if len(plays) == 0 {
			t.Fatalf("trial %d: hand %v produced no play at all", trial, hand)
		}
		inHand := make(map[int]int, len(hand))
		for _, card := range hand {
			inHand[card]++
		}
		for _, play := range plays {
			used := make(map[int]int, len(play.cards))
			for _, card := range play.cards {
				used[card]++
				if used[card] > inHand[card] {
					t.Fatalf("trial %d: play %v uses card %d which is not in hand %v", trial, play.cards, card, hand)
				}
			}
			if _, ok := common.Pokers[common.SortStr(common.ToPokers(play.cards))]; !ok {
				t.Fatalf("trial %d: play %v is not a legal combination", trial, play.cards)
			}
		}
	}
}

//领出时要带走顺子，而不是一张张拆着出
func TestBotLeadKeepsStraight(t *testing.T) {
	table, _, clients := newTableWithChips(t, 3)
	bot := clients[0]
	bot.HandPokers = botCards("34567" + "9JQ")

	shot := bot.botDecideLead()
	play, ok := botPlayOf(shot)
	if !ok {
		t.Fatalf("lead %v is not a legal play", shot)
	}
	if play.cType != "seq_single5" {
		t.Fatalf("bot led %v (%s), want the 5-card straight", shot, play.cType)
	}
	_ = table
}

//手里有炸弹时不能为了跟一张小单牌就把炸弹拆了
func TestBotFollowDoesNotBreakBomb(t *testing.T) {
	_, _, clients := newTableWithChips(t, 3)
	bot, landlord := clients[0], clients[2]
	bot.UserInfo.Role = RoleFarmer
	landlord.UserInfo.Role = RoleLandlord
	bot.HandPokers = botCards("33339")

	shot := bot.botDecideFollow(botCards("5"), landlord)
	if len(shot) != 1 {
		t.Fatalf("follow = %v, want a single", shot)
	}
	if want := botRank(botCards("9")[0]); botRank(shot[0]) != want {
		t.Fatalf("bot played rank %d while holding a bomb, want the 9", botRank(shot[0]))
	}
}

//队友出的牌不去压
func TestBotFollowPassesTeammate(t *testing.T) {
	_, _, clients := newTableWithChips(t, 3)
	bot, teammate, landlord := clients[0], clients[1], clients[2]
	bot.UserInfo.Role = RoleFarmer
	teammate.UserInfo.Role = RoleFarmer
	landlord.UserInfo.Role = RoleLandlord
	bot.HandPokers = botCards("678")

	if shot := bot.botDecideFollow(botCards("5"), teammate); len(shot) != 0 {
		t.Fatalf("bot beat its teammate with %v", shot)
	}
}

//对手出牌时用最小的牌压，不浪费大牌
func TestBotFollowUsesSmallestBeatingCard(t *testing.T) {
	_, _, clients := newTableWithChips(t, 3)
	bot, landlord := clients[0], clients[2]
	bot.UserInfo.Role = RoleFarmer
	landlord.UserInfo.Role = RoleLandlord
	bot.HandPokers = botCards("6782W")

	shot := bot.botDecideFollow(botCards("5"), landlord)
	if len(shot) != 1 {
		t.Fatalf("follow = %v, want a single", shot)
	}
	if want := botRank(botCards("6")[0]); botRank(shot[0]) != want {
		t.Fatalf("bot followed with rank %d, want the smallest beating card 6", botRank(shot[0]))
	}
}

//对手只剩一张牌时该炸就炸
func TestBotBombsWhenOpponentIsAboutToWin(t *testing.T) {
	_, _, clients := newTableWithChips(t, 3)
	bot, landlord := clients[0], clients[2]
	bot.UserInfo.Role = RoleFarmer
	landlord.UserInfo.Role = RoleLandlord
	bot.HandPokers = botCards("3333567")
	landlord.HandPokers = botCards("2")

	shot := bot.botDecideFollow(botCards("AA"), landlord)
	play, ok := botPlayOf(shot)
	if !ok || play.cType != "bomb" {
		t.Fatalf("follow = %v (%s), want the bomb", shot, play.cType)
	}
}

//不着急的时候不拿炸弹去压普通牌
func TestBotKeepsBombWhenNotUrgent(t *testing.T) {
	_, _, clients := newTableWithChips(t, 3)
	bot, landlord := clients[0], clients[2]
	bot.UserInfo.Role = RoleFarmer
	landlord.UserInfo.Role = RoleLandlord
	bot.HandPokers = botCards("3333567")
	landlord.HandPokers = botCards("456789")

	if shot := bot.botDecideFollow(botCards("AA"), landlord); len(shot) != 0 {
		t.Fatalf("bot wasted its bomb early: %v", shot)
	}
}

//叫分要看牌：一手好牌叫高分，一手烂牌不抢
func TestBotCallScoreFollowsHandStrength(t *testing.T) {
	_, _, clients := newTableWithChips(t, 3)
	strong := clients[0]
	strong.HandPokers = botCards("Ww222AAKKQJ0")
	if score := strong.botCallScore(); score < 3 {
		t.Fatalf("strong hand called %d, want 3", score)
	}

	weak := clients[1]
	weak.HandPokers = botCards("34567890JQK")
	if score := weak.botCallScore(); score > 1 {
		t.Fatalf("weak hand called %d, want 0 or 1", score)
	}
}

//三个机器人互相打，必须能正常打完一局，而且不能出现被服务器拒绝的出牌
func TestBotsFinishAFullGame(t *testing.T) {
	table, _, clients := newTableWithChips(t, 3)
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
			t.Fatal("bots could not finish the game in 400 moves")
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
				t.Fatalf("bot %d decided an illegal play: %v (hand %v)", turn.UserInfo.UserId, shot, turn.HandPokers)
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
			t.Fatalf("server rejected bot %d play %v (last shot %v)", turn.UserInfo.UserId, shot, table.GameManage.LastShotPoker)
		}
		for _, client := range clients {
			drainMessages(client)
		}
	}
	if table.State != GameEnd {
		t.Fatalf("game ended in state %d, want GameEnd", table.State)
	}
	t.Logf("bots finished the game in %d moves", moves)
}
