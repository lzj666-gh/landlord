package service

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"testing"
	"time"

	"landlord/common"
)

//机器人基线：自对局的地主胜率、平均手数、单次决策耗时。
//改动机器人策略前后都跑一遍，用同一套参数做回归对比。
//局数用环境变量 BOT_BASELINE_TRIALS 调整（默认 200）。

type botBaselineMode int

const (
	botBaselineAllNormal     botBaselineMode = iota //三个都是普通机器人
	botBaselineLevelLandlord                        //地主用指定难度，两个农民是普通机器人
	botBaselineLevelFarmer                          //两个农民里有一个用指定难度，地主是普通机器人
)

type botBaselineResult struct {
	Trials       int
	LandlordWins int
	TotalMoves   int
	Decisions    int
	DecideTime   time.Duration
	Slowest      time.Duration
}

func (r botBaselineResult) String(label string) string {
	decisions := r.Decisions
	if decisions == 0 {
		decisions = 1
	}
	return fmt.Sprintf("%s：%d 局，地主胜 %d 局（%.1f%%），平均 %.1f 手/局，决策 %d 次，平均 %v，最慢 %v",
		label, r.Trials, r.LandlordWins, float64(r.LandlordWins)*100/float64(r.Trials),
		float64(r.TotalMoves)/float64(r.Trials), r.Decisions, r.DecideTime/time.Duration(decisions), r.Slowest)
}

func baselineTrials() int {
	if raw := os.Getenv("BOT_BASELINE_TRIALS"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			return n
		}
	}
	return 200
}

//跑一串机器人自对局，返回地主胜率、手数和决策耗时
func runBotBaseline(t *testing.T, trials int, mode botBaselineMode, level botLevel) botBaselineResult {
	t.Helper()
	random := rand.New(rand.NewSource(20261126))
	result := botBaselineResult{Trials: trials}

	for trial := 0; trial < trials; trial++ {
		table, _, clients := newTableWithChips(t, 3)
		for _, client := range clients {
			client.IsRobot = true
			client.BotLevel = botLevelNormal
		}
		landlordIdx := random.Intn(3)
		switch mode {
		case botBaselineLevelLandlord:
			clients[landlordIdx].BotLevel = level
		case botBaselineLevelFarmer:
			for i, client := range clients {
				if i != landlordIdx {
					client.BotLevel = level
				}
			}
		}

		table.GameManage.FirstCallScore = clients[0]
		table.State = GameCallScore
		table.dealPoker()
		table.GameManage.MaxCallScore = 3
		table.GameManage.MaxCallScoreTurn = clients[landlordIdx]
		table.callEnd()
		for _, client := range clients {
			drainMessages(client)
		}

		moves := 0
		for table.State == GamePlaying {
			if moves++; moves > 400 {
				t.Fatalf("trial %d: the game did not finish in 400 moves", trial)
			}
			turn := table.GameManage.Turn
			if turn == nil {
				t.Fatalf("trial %d: nobody's turn in the middle of a game", trial)
			}

			start := time.Now()
			var shot []int
			if len(table.GameManage.LastShotPoker) == 0 || table.GameManage.LastShotClient == turn {
				shot = turn.botDecideLead()
			} else {
				shot = turn.botDecideFollow(table.GameManage.LastShotPoker, table.GameManage.LastShotClient)
			}
			spent := time.Since(start)
			result.DecideTime += spent
			result.Decisions++
			if spent > result.Slowest {
				result.Slowest = spent
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
				t.Fatalf("trial %d: the server rejected the play %v (last shot %v)", trial, shot, table.GameManage.LastShotPoker)
			}
			for _, client := range clients {
				drainMessages(client)
			}
		}

		winner := -1
		for i, client := range clients {
			if len(client.HandPokers) == 0 {
				winner = i
			}
		}
		if winner == landlordIdx {
			result.LandlordWins++
		}
		result.TotalMoves += moves
	}
	return result
}

//三个普通机器人自对局：这是所有策略改动的回归基准
func TestBotBaselineAllNormal(t *testing.T) {
	result := runBotBaseline(t, baselineTrials(), botBaselineAllNormal, botLevelNormal)
	t.Log(result.String("全普通机器人自对局"))
}

//地主换成指定难度，两个农民不动
func TestBotBaselineLevelLandlord(t *testing.T) {
	for _, level := range []botLevel{botLevelHard, botLevelDevil} {
		result := runBotBaseline(t, baselineTrials(), botBaselineLevelLandlord, level)
		t.Log(result.String(level.String() + "当地主（农民普通）"))
	}
}

//农民里换一个指定难度，地主不动
func TestBotBaselineLevelFarmer(t *testing.T) {
	for _, level := range []botLevel{botLevelHard, botLevelDevil} {
		result := runBotBaseline(t, baselineTrials(), botBaselineLevelFarmer, level)
		t.Log(result.String(level.String() + "当农民（地主普通）"))
	}
}
