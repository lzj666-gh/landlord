package service

import (
	"math/rand"
	"os"
	"strings"
	"time"
)

//机器人难度。越高的难度看得越多、算得越深、反应越快。
type botLevel int

const (
	botLevelNormal botLevel = iota //普通启发式策略，作为基准，也是零值
	botLevelEasy                   //随机挑一手合法牌，送给人类的陪练
	botLevelHard                   //精确拆手 + 调过的权重
	botLevelDevil                  //全知视角 + 一层预判 + 地主专属节奏
)

func (level botLevel) String() string {
	switch level {
	case botLevelEasy:
		return "easy"
	case botLevelHard:
		return "hard"
	case botLevelDevil:
		return "devil"
	default:
		return "normal"
	}
}

//解析难度名字，认不出来就当普通
func parseBotLevel(name string) (botLevel, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "easy":
		return botLevelEasy, true
	case "normal":
		return botLevelNormal, true
	case "hard":
		return botLevelHard, true
	case "devil":
		return botLevelDevil, true
	}
	return botLevelNormal, false
}

//全局默认难度，环境变量 BOT_LEVEL 可以改（easy/normal/hard/devil）
func defaultBotLevel() botLevel {
	if level, ok := parseBotLevel(os.Getenv("BOT_LEVEL")); ok {
		return level
	}
	return botLevelDevil
}

//机器人“想牌”的时间。难度越高反应越快，节奏也更有变化，不像机器人。
func botThinkTime(level botLevel) time.Duration {
	low, high := 0.4, 1.2
	switch level {
	case botLevelEasy:
		low, high = 1.5, 3.0
	case botLevelNormal:
		low, high = 1.0, 2.0
	case botLevelHard:
		low, high = 0.7, 1.6
	}
	return time.Duration((low + rand.Float64()*(high-low)) * float64(time.Second))
}
