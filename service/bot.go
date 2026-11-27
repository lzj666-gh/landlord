package service

import (
	"math/rand"
	"sort"

	"landlord/common"
)

//普通难度用的老估算器，保持原来的行为当基准
type botGreedyCounter struct{}

func (botGreedyCounter) handsLeft(hand []int) int {
	return botHandsLeft(hand)
}

//机器人决策。这里不依赖 rule.json 的牌型表“从大到小找一张能压的”，
//而是先按手里的牌枚举出所有能出的组合，再用 common.Pokers 校正合法性，
//然后按“出完这手之后还要几手才能走完”来挑。这样能避免拆炸弹、
//拿 2 和王乱冲、压队友这些低级行为。

const botRankCount = 15

//牌的大小：3..K -> 0..10，A -> 11，2 -> 12，小王 -> 13，大王 -> 14
func botRank(card int) int {
	if card == 52 {
		return 14
	}
	if card == 53 {
		return 13
	}
	switch rank := card % 13; rank {
	case 0:
		return 11
	case 1:
		return 12
	default:
		return rank - 2
	}
}

//按点数分组，组内牌号升序
func botGroup(hand []int) [botRankCount][]int {
	var byRank [botRankCount][]int
	for _, card := range hand {
		rank := botRank(card)
		byRank[rank] = append(byRank[rank], card)
	}
	for i := range byRank {
		sort.Ints(byRank[i])
	}
	return byRank
}

type botPlay struct {
	cards []int
	cType string
	score int
}

func (play botPlay) isBomb() bool   { return play.cType == "bomb" }
func (play botPlay) isRocket() bool { return play.cType == "rocket" }

//用规则表校验一手牌，顺便拿到牌型和大小
func botNewPlay(cards []int) (botPlay, bool) {
	if len(cards) == 0 {
		return botPlay{}, false
	}
	sorted := append([]int(nil), cards...)
	sort.Ints(sorted)
	combination, ok := common.Pokers[common.SortStr(common.ToPokers(sorted))]
	if !ok {
		return botPlay{}, false
	}
	return botPlay{cards: sorted, cType: combination.Type, score: combination.Score}, true
}

//枚举手里所有能出的牌型
func botPlays(hand []int) []botPlay {
	byRank := botGroup(hand)
	var counts [botRankCount]int
	for rank := range counts {
		counts[rank] = len(byRank[rank])
	}

	plays := make([]botPlay, 0, 256)
	seen := make(map[string]bool, 256)
	add := func(groups ...[]int) {
		cards := make([]int, 0, 20)
		for _, group := range groups {
			cards = append(cards, group...)
		}
		play, ok := botNewPlay(cards)
		if !ok {
			return
		}
		key := play.cType + "-"
		for _, card := range play.cards {
			key += string(rune('A' + card))
		}
		if seen[key] {
			return
		}
		seen[key] = true
		plays = append(plays, play)
	}

	//单张、对子、三张、炸弹
	for rank := 0; rank < botRankCount; rank++ {
		for size := 1; size <= counts[rank] && size <= 4; size++ {
			add(byRank[rank][:size])
		}
	}
	if counts[13] > 0 && counts[14] > 0 {
		add(byRank[13][:1], byRank[14][:1])
	}

	//三带一、三带二
	for rank := 0; rank < botRankCount; rank++ {
		if counts[rank] < 3 {
			continue
		}
		for wing := 0; wing < botRankCount; wing++ {
			if wing == rank {
				continue
			}
			if counts[wing] >= 1 {
				add(byRank[rank][:3], byRank[wing][:1])
			}
			if counts[wing] >= 2 {
				add(byRank[rank][:3], byRank[wing][:2])
			}
		}
	}

	//四带二、四带两对
	for rank := 0; rank < botRankCount; rank++ {
		if counts[rank] != 4 {
			continue
		}
		for first := 0; first < botRankCount; first++ {
			for second := first + 1; second < botRankCount; second++ {
				if first == rank || second == rank {
					continue
				}
				if counts[first] >= 1 && counts[second] >= 1 {
					add(byRank[rank], byRank[first][:1], byRank[second][:1])
				}
				if counts[first] >= 2 && counts[second] >= 2 {
					add(byRank[rank], byRank[first][:2], byRank[second][:2])
				}
			}
		}
	}

	//顺子（5 张以上，3..A）
	for start := 0; start <= 11; start++ {
		for end := start; end <= 11; end++ {
			if counts[end] == 0 {
				break
			}
			if end-start+1 < 5 {
				continue
			}
			groups := make([][]int, 0, 12)
			for rank := start; rank <= end; rank++ {
				groups = append(groups, byRank[rank][:1])
			}
			add(groups...)
		}
	}

	//连对（3 对以上）
	for start := 0; start <= 11; start++ {
		for end := start; end <= 11; end++ {
			if counts[end] < 2 {
				break
			}
			if end-start+1 < 3 {
				continue
			}
			groups := make([][]int, 0, 12)
			for rank := start; rank <= end; rank++ {
				groups = append(groups, byRank[rank][:2])
			}
			add(groups...)
		}
	}

	//飞机（连续三张）以及带单、带对
	for start := 0; start <= 11; start++ {
		for end := start; end <= 11; end++ {
			if counts[end] < 3 {
				break
			}
			length := end - start + 1
			if length < 2 {
				continue
			}
			body := make([][]int, 0, 6)
			for rank := start; rank <= end; rank++ {
				body = append(body, byRank[rank][:3])
			}
			add(body...)

			singleWings := make([]int, 0, 8)
			pairWings := make([]int, 0, 8)
			for rank := 0; rank < botRankCount; rank++ {
				if rank >= start && rank <= end {
					continue
				}
				if counts[rank] >= 1 {
					singleWings = append(singleWings, rank)
				}
				if counts[rank] >= 2 {
					pairWings = append(pairWings, rank)
				}
			}
			for _, wings := range botWingChoices(byRank, singleWings, length, 1) {
				groups := make([][]int, 0, length*2)
				groups = append(groups, body...)
				for _, rank := range wings {
					groups = append(groups, byRank[rank][:1])
				}
				add(groups...)
			}
			for _, wings := range botWingChoices(byRank, pairWings, length, 2) {
				groups := make([][]int, 0, length*2)
				groups = append(groups, body...)
				for _, rank := range wings {
					groups = append(groups, byRank[rank][:2])
				}
				add(groups...)
			}
		}
	}
	return plays
}

//给飞机挑翅膀：先按点数从小到大，再尽量选“刚好用完”的点数，避免拆对子、三张、炸弹
func botWingChoices(byRank [botRankCount][]int, candidates []int, count, need int) [][]int {
	if count == 0 {
		return [][]int{{}}
	}
	if len(candidates) < count {
		return nil
	}
	choices := make([][]int, 0, 2)
	seen := make(map[string]bool, 2)
	push := func(ranks []int) {
		if len(ranks) != count {
			return
		}
		key := ""
		for _, rank := range ranks {
			key += string(rune('A' + rank))
		}
		if seen[key] {
			return
		}
		seen[key] = true
		choices = append(choices, ranks)
	}

	push(append([]int(nil), candidates[:count]...))

	sorted := append([]int(nil), candidates...)
	sort.SliceStable(sorted, func(i, j int) bool {
		left, right := botWingWeight(byRank, sorted[i], need), botWingWeight(byRank, sorted[j], need)
		if left != right {
			return left < right
		}
		return sorted[i] < sorted[j]
	})
	push(append([]int(nil), sorted[:count]...))
	return choices
}

func botWingWeight(byRank [botRankCount][]int, rank, need int) int {
	switch size := len(byRank[rank]); {
	case size == need:
		return 0
	case size == 4:
		return 3
	case size == 3:
		return 2
	default:
		return 1
	}
}

//从手里扣掉已经打出去的牌
func botRemove(hand, cards []int) []int {
	rest := append([]int(nil), hand...)
	for _, card := range cards {
		for i, item := range rest {
			if item == card {
				rest = append(rest[:i], rest[i+1:]...)
				break
			}
		}
	}
	return rest
}

//打完这一手之后大概还要出几手才能走完（越少越好）
func botHandsLeft(hand []int) int {
	var counts [botRankCount]int
	for _, card := range hand {
		counts[botRank(card)]++
	}
	hands := 0
	if counts[13] > 0 && counts[14] > 0 {
		hands++
		counts[13]--
		counts[14]--
	}
	for rank := 0; rank < 13; rank++ {
		if counts[rank] == 4 {
			hands++
			counts[rank] = 0
		}
	}
	for {
		start, length := botLongestRun(&counts, 11, 1)
		if length < 5 {
			break
		}
		for rank := start; rank < start+length; rank++ {
			counts[rank]--
		}
		hands++
	}
	for {
		start, length := botLongestRun(&counts, 11, 2)
		if length < 3 {
			break
		}
		for rank := start; rank < start+length; rank++ {
			counts[rank] -= 2
		}
		hands++
	}
	for rank := 0; rank < 13; rank++ {
		if counts[rank] < 3 {
			continue
		}
		hands++
		counts[rank] -= 3
		for wing := 0; wing < botRankCount; wing++ {
			if counts[wing] == 1 {
				counts[wing] = 0
				break
			}
		}
	}
	for rank := 0; rank < botRankCount; rank++ {
		if counts[rank] >= 2 {
			hands++
			counts[rank] -= 2
		}
	}
	for rank := 0; rank < botRankCount; rank++ {
		if counts[rank] > 0 {
			hands += counts[rank]
			counts[rank] = 0
		}
	}
	return hands
}

//最长的一段连续点数（含头不含尾返回 start/length）
func botLongestRun(counts *[botRankCount]int, maxRank, need int) (int, int) {
	bestStart, bestLength := -1, 0
	for start := 0; start <= maxRank; start++ {
		if counts[start] < need {
			continue
		}
		end := start
		for end+1 <= maxRank && counts[end+1] >= need {
			end++
		}
		if end-start+1 > bestLength {
			bestStart, bestLength = start, end-start+1
		}
		start = end
	}
	return bestStart, bestLength
}

//手里的大牌（2、王）的分量，用来避免拿大牌去压小牌
func botPower(hand []int) int {
	power := 0
	for _, card := range hand {
		switch botRank(card) {
		case 14:
			power += 3
		case 13:
			power += 2
		case 12:
			power += 2
		case 11:
			power++
		}
	}
	return power
}

//这手牌里最大的点数，同样手数时先出小的
func botHighestRank(cards []int) int {
	highest := 0
	for _, card := range cards {
		if rank := botRank(card); rank > highest {
			highest = rank
		}
	}
	return highest
}

//出这手牌会拆掉多少牌型：拆三张/炸弹代价大，拆对子代价小
func botBreakCost(hand, cards []int) int {
	var total, used [botRankCount]int
	for _, card := range hand {
		total[botRank(card)]++
	}
	for _, card := range cards {
		used[botRank(card)]++
	}
	cost := 0
	for rank := 0; rank < botRankCount; rank++ {
		switch {
		case used[rank] == 0 || used[rank] == total[rank]:
		case total[rank] >= 3:
			cost += 3
		case total[rank] == 2 && used[rank] == 1:
			cost++
		}
	}
	return cost
}

//是否拆了炸弹（四张只用了一部分）
func botBreaksBomb(hand, cards []int) bool {
	var total, used [botRankCount]int
	for _, card := range hand {
		total[botRank(card)]++
	}
	for _, card := range cards {
		used[botRank(card)]++
	}
	for rank := 0; rank < botRankCount; rank++ {
		if total[rank] == 4 && used[rank] > 0 && used[rank] < 4 {
			return true
		}
	}
	return false
}

//对手（地主对两个农民，农民只对地主）里有没有快走完的
func (c *Client) botOpponentNearlyDone() bool {
	table := c.Table
	if table == nil {
		return false
	}
	for _, client := range table.TableClients {
		if client == c {
			continue
		}
		if c.UserInfo.Role == client.UserInfo.Role {
			continue
		}
		if len(client.HandPokers) <= 2 {
			return true
		}
	}
	return false
}

//领出：从所有合法牌型里挑“打完剩的手数最少、最不浪费大牌”的一手
func (c *Client) botDecideLead() []int {
	switch c.BotLevel {
	case botLevelEasy:
		return c.botDecideRandomLead()
	case botLevelNormal:
		return c.botDecideLeadScored(c.botCounter(), false)
	case botLevelHard:
		return c.botDecideLeadScored(c.botCounter(), false)
	default:
		//魔鬼难度：精确拆手 + 预判对手
		return c.botDecideLeadScored(c.botCounter(), c.botLookahead())
	}
}

//跟牌：能压就压，不压队友，炸弹留到关键时候
func (c *Client) botDecideFollow(last []int, lastClient *Client) []int {
	switch c.BotLevel {
	case botLevelEasy:
		return c.botDecideRandomFollow(last)
	case botLevelNormal:
		return c.botDecideFollowScored(last, lastClient, c.botCounter(), false)
	case botLevelHard:
		return c.botDecideFollowScored(last, lastClient, c.botCounter(), false)
	default:
		//魔鬼难度：预判对手能不能一手走完
		return c.botDecideFollowScored(last, lastClient, c.botCounter(), c.botLookahead())
	}
}

//这一手牌用哪种拆手估算
func (c *Client) botCounter() botHandCounter {
	switch c.BotLevel {
	case botLevelNormal:
		return botGreedyCounter{}
	case botLevelHard:
		return botSmartCounter{}
	default:
		return botSmartCounter{}
	}
}

//预判的权重。这些值是用 service/bot_baseline_test.go 的基线扫出来的，
//改之前请先跑一遍基线对比。
var botThreatWeight = 6 //对手离走完更近时，每多一手加多少分

//预判只在农民这一侧开。
//农民的对手是单独一个地主，模型（对手会怎么反击、还剩几手）比较准，
//实测能让农民的胜率从 55.7% 提到 65.5%。
//地主的对手是两个会互相配合的农民，拿单个贪心模型去套会误导地主，
//实测地主胜率会从 49.7% 掉到 40.8%，所以地主这一侧不启用预判。
func (c *Client) botLookahead() bool {
	return c.BotLevel >= botLevelDevil && c.UserInfo.Role == RoleFarmer
}

//领出的评分。
//注意：曾经试过给地主单独一套权重（更重节奏、更舍得用大牌），
//实测地主胜率从 42.5% 掉到 27%，反而更差，已回退，地主农民用同一套权重。
func (c *Client) botLeadScore(hand, rest []int, play botPlay, counter botHandCounter) int {
	score := counter.handsLeft(rest)*100 - len(play.cards)*3
	score += botBreakCost(hand, play.cards) * 20
	score += (botPower(hand) - botPower(rest)) * -4
	score += botHighestRank(play.cards)
	if play.isBomb() {
		score += 45
	}
	if play.isRocket() {
		score += 55
	}
	return score
}

//领出：按评分挑一手；魔鬼难度还会算一步对手能不能压住
func (c *Client) botDecideLeadScored(counter botHandCounter, lookahead bool) []int {
	hand := append([]int(nil), c.HandPokers...)
	if len(hand) == 0 {
		return nil
	}
	plays := botPlays(hand)
	if len(plays) == 0 {
		return []int{hand[0]}
	}

	//先排除会拆炸弹的出法，除非没有别的选择
	safe := make([]botPlay, 0, len(plays))
	for _, play := range plays {
		if !botBreaksBomb(hand, play.cards) {
			safe = append(safe, play)
		}
	}
	if len(safe) > 0 {
		plays = safe
	}

	best := plays[0].cards
	bestScore := 1 << 30
	for _, play := range plays {
		rest := botRemove(hand, play.cards)
		if len(rest) == 0 {
			return play.cards
		}
		score := c.botLeadScore(hand, rest, play, counter)
		if lookahead {
			score += c.botLeadThreat(rest, play, counter)
		}
		if score < bestScore {
			bestScore, best = score, play.cards
		}
	}
	return best
}

//预判：这一手打出去，对手是不是能压住并且直接走完
func (c *Client) botLeadThreat(rest []int, play botPlay, counter botHandCounter) int {
	threat := 0
	for _, opponent := range c.botOpponents() {
		oppHand := opponent.HandPokers
		if len(oppHand) == 0 {
			continue
		}
		_, oppRest, oppWins := botBestResponse(botPlays(oppHand), oppHand, play.cards, counter)
		if oppWins {
			//这一手打出去对手直接走完，这手不能要
			return 1 << 20
		}
		if oppRest == nil {
			//对手压不住这一手。实测“奖励抢节奏”反而让机器人变弱，
			//所以这里不加分，只是不罚分。
			continue
		}
		if diff := counter.handsLeft(rest) - counter.handsLeft(oppRest); diff > threat {
			//对手剩下的手数越少越危险，这一手要加罚
			threat = diff
		}
	}
	return threat * botThreatWeight
}

//跟牌：按评分挑一手；魔鬼难度会算一步对手的反击
func (c *Client) botDecideFollowScored(last []int, lastClient *Client, counter botHandCounter, lookahead bool) []int {
	hand := append([]int(nil), c.HandPokers...)
	if len(hand) == 0 {
		return nil
	}
	plays := botPlays(hand)

	teammate := lastClient != nil && lastClient != c &&
		c.UserInfo.Role == RoleFarmer && lastClient.UserInfo.Role == RoleFarmer
	urgent := c.botOpponentNearlyDone()
	lastIsBomb := false
	if combination, ok := common.Pokers[common.SortStr(common.ToPokers(last))]; ok {
		lastIsBomb = combination.Type == "bomb" || combination.Type == "rocket"
	}

	//对手（上一手出牌的人）的手牌，魔鬼难度会拿来做预判
	var oppHand []int
	var oppPlays []botPlay
	if lastClient != nil && len(lastClient.HandPokers) > 0 {
		oppHand = lastClient.HandPokers
		oppPlays = botPlays(oppHand)
	}

	best := botPlay{}
	bestType, bestScore, bestHigh := 3, 1<<30, 1<<30
	found := false
	for _, play := range plays {
		if compare, _ := common.ComparePoker(last, play.cards); compare <= 0 {
			continue
		}
		rest := botRemove(hand, play.cards)
		wins := len(rest) == 0
		//队友已经压住了就不抢，除非自己能一手走完
		if teammate && !wins {
			continue
		}
		//炸弹/火箭只在对手快走完、自己一手走完、或者对面先出炸弹时用
		if (play.isBomb() || play.isRocket()) && !wins && !urgent && !lastIsBomb {
			continue
		}
		//不拆炸弹去跟牌
		if !play.isBomb() && !wins && !urgent && botBreaksBomb(hand, play.cards) {
			continue
		}

		score := play.score + botBreakCost(hand, play.cards)*2
		if lookahead && !wins && len(oppPlays) > 0 {
			_, oppRest, oppWins := botBestResponse(oppPlays, oppHand, play.cards, counter)
			switch {
			case oppWins:
				//压过去对手直接走完，这一手不能要
				continue
			case oppRest != nil:
				//对手反击之后离走完更近了，要加罚
				score += (counter.handsLeft(rest) - counter.handsLeft(oppRest)) * botThreatWeight
			default:
				//对手压不住这一手，实测不加分更好
			}
		}
		playType := 0
		if play.isBomb() || play.isRocket() {
			playType = 1
		}
		high := botHighestRank(play.cards)
		//拆三张/对子去跟牌要算一点代价：能出 7 就别去拆 666，
		//但代价也不能大到宁可用 2 去顶
		if !found || playType < bestType ||
			(playType == bestType && score < bestScore) ||
			(playType == bestType && score == bestScore && high < bestHigh) {
			found = true
			best = play
			bestType, bestScore, bestHigh = playType, score, high
		}
	}
	if !found {
		return []int{}
	}
	return best.cards
}

//对手手里能压过我这手牌的最佳出法：返回出法、剩下的牌、能不能直接走完
func botBestResponse(oppPlays []botPlay, oppHand []int, myPlay []int, counter botHandCounter) (botPlay, []int, bool) {
	best := botPlay{}
	bestHands := 1 << 30
	found := false
	for _, play := range oppPlays {
		if compare, _ := common.ComparePoker(myPlay, play.cards); compare <= 0 {
			continue
		}
		rest := botRemove(oppHand, play.cards)
		if len(rest) == 0 {
			return play, rest, true
		}
		hands := counter.handsLeft(rest)
		if !found || hands < bestHands || (hands == bestHands && play.score < best.score) {
			found = true
			best, bestHands = play, hands
		}
	}
	if !found {
		return botPlay{}, nil, false
	}
	return best, botRemove(oppHand, best.cards), false
}

//对手（不是自己队友的那些人）
func (c *Client) botOpponents() []*Client {
	table := c.Table
	if table == nil {
		return nil
	}
	list := make([]*Client, 0, 2)
	for _, client := range table.TableClients {
		if client == c {
			continue
		}
		if c.UserInfo.Role == client.UserInfo.Role {
			continue
		}
		list = append(list, client)
	}
	return list
}

//简单难度：领出随便挑一手
func (c *Client) botDecideRandomLead() []int {
	plays := botPlays(c.HandPokers)
	if len(plays) == 0 {
		if len(c.HandPokers) > 0 {
			return []int{c.HandPokers[0]}
		}
		return nil
	}
	return plays[rand.Intn(len(plays))].cards
}

//简单难度：能压就随机压一张，压不住就不出
func (c *Client) botDecideRandomFollow(last []int) []int {
	plays := botPlays(c.HandPokers)
	candidates := make([]botPlay, 0, len(plays))
	for _, play := range plays {
		if compare, _ := common.ComparePoker(last, play.cards); compare <= 0 {
			continue
		}
		candidates = append(candidates, play)
	}
	if len(candidates) == 0 {
		return []int{}
	}
	//偏向出小牌，好让人赢
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].score < candidates[j].score })
	limit := len(candidates)/2 + 1
	return candidates[rand.Intn(limit)].cards
}

//叫分：按大牌、炸弹和张数评估，不再无脑叫三分
func (c *Client) botCallScore() int {
	hand := c.HandPokers
	var counts [botRankCount]int
	for _, card := range hand {
		counts[botRank(card)]++
	}
	strength := counts[11] + counts[12]*2 + counts[13]*2 + counts[14]*4
	if counts[13] > 0 && counts[14] > 0 {
		strength += 4
	}
	for rank := 0; rank < 13; rank++ {
		if counts[rank] == 4 {
			strength += 5
		}
	}
	switch hands := c.botCounter().handsLeft(hand); {
	case hands <= 6:
		strength += 5
	case hands <= 8:
		strength += 2
	case hands >= 11:
		strength -= 2
	}
	switch {
	case strength >= 13:
		return 3
	case strength >= 9:
		return 2
	case strength >= 5:
		return 1
	default:
		return 0
	}
}
