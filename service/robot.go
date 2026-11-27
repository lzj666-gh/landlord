package service

import (
	"github.com/astaxie/beego/logs"
	"landlord/common"
	"time"
)

func (c *Client) runRobot() {
	for {
		//先处理自己排队好的叫分/出牌，再取牌桌发来的新消息
		select {
		case msg, ok := <-c.toServer:
			if !ok {
				return
			}
			wsRequest(msg, c)
			continue
		default:
		}
		msg, ok := c.inbox.pop()
		if !ok {
			return
		}
		logs.Debug("robot [%v] receive  message %v ", c.UserInfo.Username, msg)
		if len(msg) < 1 {
			logs.Error("send to robot [%v],message err ,%v", c.UserInfo.Username, msg)
			return
		}
		if act, ok := msg[0].(int); ok {
			protocolCode := int(act)
			switch protocolCode {
			case common.ResDealPoker:
				if !c.isFirstCallScore() {
					break
				}
				time.Sleep(botThinkTime(c.BotLevel))
				c.Table.Lock.RLock()
				if c.Table.GameManage.FirstCallScore == c {
					c.autoCallScore()
				}
				c.Table.Lock.RUnlock()

			case common.ResCallScore:
				if len(msg) < 4 {
					logs.Error("ResCallScore msg err:%v", msg)
					return
				}
				callEnd := false
				if res, ok := msg[3].(bool); ok {
					callEnd = bool(res)
				}
				//只有轮到自己叫分时才“想一秒”，别人叫分不该拖慢牌桌
				if callEnd || !c.isMyTurn() || c.IsCalled {
					break
				}
				time.Sleep(botThinkTime(c.BotLevel))
				c.Table.Lock.RLock()
				if c.Table.GameManage.Turn == c && !c.IsCalled && c.Table.State == GameCallScore {
					c.autoCallScore()
				}
				c.Table.Lock.RUnlock()

			case common.ResShotPoker:
				if !c.isMyTurn() {
					break
				}
				time.Sleep(botThinkTime(c.BotLevel))
				c.Table.Lock.RLock()
				if c.Table.GameManage.Turn == c && c.Table.State == GamePlaying {
					c.autoShotPoker()
				}
				c.Table.Lock.RUnlock()

			case common.ResShowPoker:
				if !c.isMyTurn() && c.UserInfo.Role != RoleLandlord {
					break
				}
				time.Sleep(botThinkTime(c.BotLevel))
				c.Table.Lock.RLock()
				if c.Table.GameManage.Turn == c || (c.Table.GameManage.Turn == nil && c.UserInfo.Role == RoleLandlord) {
					c.autoShotPoker()
				}
				c.Table.Lock.RUnlock()
			case common.ResGameOver:
				c.Ready = true
			}
		}
	}
}

//轮到自己了吗（只看一眼牌桌状态，不长时间持锁）
func (c *Client) isMyTurn() bool {
	if c.Table == nil {
		return false
	}
	c.Table.Lock.RLock()
	turn := c.Table.GameManage.Turn
	state := c.Table.State
	c.Table.Lock.RUnlock()
	return turn == c && (state == GameCallScore || state == GamePlaying)
}

//本局第一个叫分的是不是自己
func (c *Client) isFirstCallScore() bool {
	if c.Table == nil {
		return false
	}
	c.Table.Lock.RLock()
	first := c.Table.GameManage.FirstCallScore
	state := c.Table.State
	c.Table.Lock.RUnlock()
	return first == c && state == GameCallScore
}

//自动出牌。调用方必须持有牌桌读锁（runRobot 里已经拿着）。
func (c *Client) autoShotPoker() {
	//因为机器人休眠一秒后才出牌，有可能因用户退出而关闭chan
	defer func() {
		err := recover()
		if err != nil {
			logs.Warn("autoShotPoker err : %v", err)
		}
	}()
	game := c.Table.GameManage
	var shotPokers []int
	if len(game.LastShotPoker) == 0 || game.LastShotClient == c {
		shotPokers = c.botDecideLead()
	} else {
		shotPokers = c.botDecideFollow(game.LastShotPoker, game.LastShotClient)
	}
	if shotPokers == nil {
		shotPokers = []int{}
	}
	float64Pokers := make([]interface{}, 0, len(shotPokers))
	for _, poker := range shotPokers {
		float64Pokers = append(float64Pokers, float64(poker))
	}
	req := []interface{}{float64(common.ReqShotPoker)}
	req = append(req, float64Pokers)
	logs.Debug("robot [%v] autoShotPoker %v (hand %v)", c.UserInfo.Username, float64Pokers, c.HandPokers)
	c.toServer <- req
}

//自动叫分。调用方必须持有牌桌读锁。
func (c *Client) autoCallScore() {
	defer func() {
		err := recover()
		if err != nil {
			logs.Warn("autoCallScore err : %v", err)
		}
	}()
	score := c.botCallScore()
	if score <= c.Table.GameManage.MaxCallScore {
		score = 0
	}
	logs.Debug("robot [%v] autoCallScore %d (hand %v)", c.UserInfo.Username, score, c.HandPokers)
	c.toServer <- []interface{}{float64(common.ReqCallScore), float64(score)}
}
