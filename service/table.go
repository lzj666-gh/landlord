package service

import (
	"fmt"
	"github.com/astaxie/beego/logs"
	"landlord/common"
	"math/rand"
	"sort"
	"sync"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type TableId int

const (
	GameWaitting = iota
	GameCallScore
	GamePlaying
	GameEnd
)

type Table struct {
	Lock         sync.RWMutex
	TableId      TableId
	State        int
	Creator      *Client
	TableClients map[UserId]*Client
	GameManage   *GameManage
	robotSeq     int //机器人编号，只增不减
}

type GameManage struct {
	Turn             *Client
	FirstCallScore   *Client //每局轮转
	MaxCallScore     int     //最大叫分
	MaxCallScoreTurn *Client
	LastShotClient   *Client
	Pokers           []int
	LastShotPoker    []int
	Multiple         int //加倍
}

func (table *Table) allCalled() bool {
	for _, client := range table.TableClients {
		if !client.IsCalled {
			return false
		}
	}
	return true
}

//一局结束：按 底注×叫分×倍数 结算金币，赢家按地主/农民阵营收付
func (table *Table) gameOver(winner *Client) {
	if winner == nil {
		logs.Error("table[%d] game over without a winner", table.TableId)
		return
	}
	if table.State == GameEnd {
		//重复到达结算入口时只结算一次
		return
	}
	entranceFee := 0
	if table.Creator != nil && table.Creator.Room != nil {
		entranceFee = table.Creator.Room.EntranceFee
	}
	multiple := table.GameManage.Multiple
	if multiple < 1 {
		multiple = 1
	}
	score := table.GameManage.MaxCallScore
	if score < 1 {
		score = 1
	}
	base := entranceFee * score * multiple
	landlordWin := winner.UserInfo.Role == RoleLandlord
	table.State = GameEnd

	type settlement struct {
		delta  int
		coin   int
		refill int
	}
	settlements := make(map[UserId]settlement, len(table.TableClients))
	for _, c := range table.TableClients {
		delta := base
		if c.UserInfo.Role == RoleLandlord {
			delta = base * 2
		}
		if landlordWin != (c.UserInfo.Role == RoleLandlord) {
			//地主赢则农民输，农民赢则地主输；地主一人承担两份底注
			delta = -delta
		}
		coin := c.UserInfo.Coin + delta
		if coin < 0 {
			coin = 0
		}
		refill := 0
		if coin < common.MinCoin {
			//金币见底时的救济：补到 1000，避免玩家卡在桌上没法继续
			refill = common.MinCoin - coin
			coin = common.MinCoin
		}
		c.UserInfo.Coin = coin
		if !c.IsRobot {
			if err := common.SaveCoin(int(c.UserInfo.UserId), coin); err != nil {
				logs.Error("save coin for user[%d] failed: %v", c.UserInfo.UserId, err)
			}
		}
		settlements[c.UserInfo.UserId] = settlement{delta: delta, coin: coin, refill: refill}
		logs.Info("table[%d] settle user[%d] role[%d] delta[%d] coin[%d] refill[%d]",
			table.TableId, c.UserInfo.UserId, c.UserInfo.Role, delta, coin, refill)
	}

	for _, c := range table.TableClients {
		s := settlements[c.UserInfo.UserId]
		res := []interface{}{common.ResGameOver, winner.UserInfo.UserId, s.delta}
		for _, cc := range table.TableClients {
			if cc != c {
				userPokers := make([]int, 0, len(cc.HandPokers)+1)
				userPokers = append(append(userPokers, int(cc.UserInfo.UserId)), cc.HandPokers...)
				res = append(res, userPokers)
			}
		}
		res = append(res, s.coin, base, multiple, s.refill)
		c.sendMsg(res)
	}
	logs.Info("table[%d] game over winner[%d] landlordWin[%v] base[%d] multiple[%d]",
		table.TableId, winner.UserInfo.UserId, landlordWin, base, multiple)
}

//叫分阶段结束
func (table *Table) callEnd() {
	table.State = GamePlaying
	table.GameManage.FirstCallScore = table.nextFirstCallScore()
	if table.GameManage.MaxCallScoreTurn == nil || table.GameManage.MaxCallScore == 0 ||
		table.TableClients[table.GameManage.MaxCallScoreTurn.UserInfo.UserId] != table.GameManage.MaxCallScoreTurn {
		table.GameManage.MaxCallScoreTurn = table.pickClient()
		table.GameManage.MaxCallScore = 1
		//return
	}
	landLord := table.GameManage.MaxCallScoreTurn
	if landLord == nil {
		logs.Error("table[%d] callEnd with nobody on the table", table.TableId)
		return
	}
	landLord.UserInfo.Role = RoleLandlord
	table.GameManage.Turn = landLord
	for _, poker := range table.GameManage.Pokers {
		landLord.HandPokers = append(landLord.HandPokers, poker)
	}
	res := []interface{}{common.ResShowPoker, landLord.UserInfo.UserId, table.GameManage.Pokers}
	for _, c := range table.TableClients {
		c.sendMsg(res)
	}
}

//客户端加入牌桌，返回是否成功入座
func (table *Table) joinTable(c *Client) bool {
	table.Lock.Lock()
	defer table.Lock.Unlock()
	return table.joinClient(c)
}

//真正入座的动作，调用方必须持有牌桌锁
func (table *Table) joinClient(c *Client) bool {
	if old, ok := table.TableClients[c.UserInfo.UserId]; ok {
		//同一账号再次加入说明是断线重连，接替原座位继续这一局
		table.takeOverSeat(old, c)
		return true
	}
	if len(table.TableClients) >= 3 {
		logs.Error("Player[%d] JOIN Table[%d] FULL", c.UserInfo.UserId, table.TableId)
		return false
	}
	logs.Debug("[%v] user [%v] request join table", c.UserInfo.UserId, c.UserInfo.Username)

	c.Table = table
	c.Ready = true
	table.appendToChain(c)
	table.TableClients[c.UserInfo.UserId] = c
	table.syncUser()
	if len(table.TableClients) == 3 {
		switch {
		case table.State == GameEnd:
			//上一局结束、现在人齐了，直接开新一局
			table.reset()
		case table.State == GameWaitting:
			table.State = GameCallScore
			table.dealPoker()
		case table.State == GameCallScore && !table.hasDealt():
			//之前人不够没能发牌，现在补齐了
			table.dealPoker()
		}
	}
	return true
}

//把新玩家接到出牌顺序的最后，牌桌上的出牌顺序是一个环
func (table *Table) appendToChain(c *Client) {
	ordered := table.clientsInOrder()
	if len(ordered) > 0 {
		ordered[len(ordered)-1].Next = c
	}
	c.Next = table.Creator
}

//按出牌顺序返回牌桌上的玩家，链表断了或者成环也不会漏人
func (table *Table) clientsInOrder() []*Client {
	ordered := make([]*Client, 0, len(table.TableClients))
	seen := make(map[*Client]bool, len(table.TableClients))
	for c := table.Creator; c != nil && len(ordered) < len(table.TableClients); c = c.Next {
		if seen[c] {
			break
		}
		seen[c] = true
		ordered = append(ordered, c)
	}
	for _, c := range table.TableClients {
		if !seen[c] {
			seen[c] = true
			ordered = append(ordered, c)
		}
	}
	return ordered
}

//把玩家从牌桌上摘掉，同时维护出牌顺序
func (table *Table) removeClient(c *Client) {
	ordered := table.clientsInOrder()
	delete(table.TableClients, c.UserInfo.UserId)

	removed := false
	for i, client := range ordered {
		if client != c {
			continue
		}
		removed = true
		if len(ordered) > 1 {
			prev := ordered[(i-1+len(ordered))%len(ordered)]
			next := ordered[(i+1)%len(ordered)]
			prev.Next = next
			if table.Creator == c {
				table.Creator = next
			}
		} else {
			table.Creator = nil
		}
		break
	}
	if !removed && table.Creator == c {
		table.Creator = nil
	}
	c.Next = nil
	if c.IsRobot {
		if c.inbox != nil {
			c.inbox.close()
		} else if c.toRobot != nil {
			close(c.toRobot)
		}
		if c.toServer != nil {
			close(c.toServer)
		}
	}
}

//玩家主动离开牌桌（一局结束后返回房间界面），返回是否真的离开了
func (table *Table) leaveTable(c *Client) bool {
	table.Lock.Lock()
	if table.TableClients[c.UserInfo.UserId] != c {
		table.Lock.Unlock()
		return false
	}
	if table.State == GameCallScore || table.State == GamePlaying {
		//牌局还在进行，不能把座位抽走
		table.Lock.Unlock()
		return false
	}
	room := c.Room
	table.removeClient(c)
	c.Table = nil
	empty := !table.hasHuman()
	if empty {
		//桌上一个真人都没有了，机器人和桌子一起收掉
		for _, robot := range table.robots() {
			table.removeClient(robot)
		}
		table.Creator = nil
	} else {
		table.syncUser()
	}
	//房间锁要在牌桌锁之外拿：加入牌桌时是先锁房间再锁牌桌，反过来会死锁
	table.Lock.Unlock()

	if empty && room != nil {
		room.Lock.Lock()
		delete(room.Tables, table.TableId)
		room.Lock.Unlock()
	}
	return true
}

//桌上还有没有真人
func (table *Table) hasHuman() bool {
	for _, client := range table.TableClients {
		if !client.IsRobot {
			return true
		}
	}
	return false
}

//桌上的真人数量
func (table *Table) humanCount() int {
	count := 0
	for _, client := range table.TableClients {
		if !client.IsRobot {
			count++
		}
	}
	return count
}

//牌局是否已经发过牌，叫分阶段桌面上会留三张底牌
func (table *Table) hasDealt() bool {
	return len(table.GameManage.Pokers) == 3
}

//本局第一个叫分的玩家：沿着上一局的顺序往下排，人不在桌上了就重新挑一个
func (table *Table) nextFirstCallScore() *Client {
	current := table.GameManage.FirstCallScore
	if current != nil {
		if next := current.Next; next != nil && table.TableClients[next.UserInfo.UserId] == next {
			return next
		}
		if table.TableClients[current.UserInfo.UserId] == current {
			return current
		}
	}
	return table.pickClient()
}

//随便挑一个还在牌桌上的玩家
func (table *Table) pickClient() *Client {
	if c := table.Creator; c != nil && table.TableClients[c.UserInfo.UserId] == c {
		return c
	}
	for _, c := range table.clientsInOrder() {
		return c
	}
	return nil
}

//同一账号断线重连：新连接接替原座位，牌局和手牌都保留，不能再发一次牌
func (table *Table) takeOverSeat(old, c *Client) {
	logs.Debug("[%v] user [%v] reconnect table[%d]", c.UserInfo.UserId, c.UserInfo.Username, table.TableId)
	c.Table = table
	c.Room = old.Room
	c.HandPokers = old.HandPokers
	c.UserInfo.Role = old.UserInfo.Role
	c.IsCalled = old.IsCalled
	c.Ready = old.Ready
	c.Next = old.Next

	for _, client := range table.TableClients {
		if client.Next == old {
			client.Next = c
		}
	}
	if table.Creator == old {
		table.Creator = c
	}
	if table.GameManage.Turn == old {
		table.GameManage.Turn = c
	}
	if table.GameManage.FirstCallScore == old {
		table.GameManage.FirstCallScore = c
	}
	if table.GameManage.MaxCallScoreTurn == old {
		table.GameManage.MaxCallScoreTurn = c
	}
	if table.GameManage.LastShotClient == old {
		table.GameManage.LastShotClient = c
	}

	table.TableClients[c.UserInfo.UserId] = c
	old.Table = nil //旧连接已经退出牌桌，之后它的 close() 不能再改动这张桌子
	if old.conn != nil {
		old.conn.Close() //同一账号只保留一个连接
	}
	table.syncUser()
	table.resumeGame(c)
}

//把当前牌局进度补发给重连的客户端
func (table *Table) resumeGame(c *Client) {
	turn := table.GameManage.Turn
	if turn == nil {
		turn = table.GameManage.FirstCallScore
	}
	if turn == nil {
		return
	}
	res := []interface{}{
		common.ResDealPoker,
		turn.UserInfo.UserId,
		append([]int(nil), c.HandPokers...),
		table.State,
		append([]int(nil), table.GameManage.LastShotPoker...),
	}
	if table.GameManage.LastShotClient != nil {
		res = append(res, table.GameManage.LastShotClient.UserInfo.UserId)
	} else {
		res = append(res, 0)
	}
	if table.GameManage.MaxCallScoreTurn != nil {
		res = append(res, table.GameManage.MaxCallScoreTurn.UserInfo.UserId)
	} else {
		res = append(res, 0)
	}
	c.sendMsg(res)
}

//玩家手动添加机器人补齐人数：牌桌还没开局、或者一局刚结束时可以添加
func (table *Table) addRobotByUser(requester *Client, room *Room) (bool, string) {
	table.Lock.Lock()
	defer table.Lock.Unlock()

	if room == nil || !room.AllowRobot {
		return false, "该房间不允许添加机器人"
	}
	if table.TableClients[requester.UserInfo.UserId] != requester {
		return false, "你不在这个牌桌上"
	}
	if len(table.TableClients) >= 3 {
		return false, "座位已经满了"
	}
	if table.State != GameWaitting && table.State != GameEnd {
		return false, "牌局进行中，不能添加机器人"
	}
	robot := table.newRobotClient(room)
	table.joinClient(robot)
	logs.Debug("robot [%v] join table[%d] by user [%d]", robot.UserInfo.Username, table.TableId, requester.UserInfo.UserId)
	return true, ""
}

//生成一个机器人玩家
func (table *Table) newRobotClient(room *Room) *Client {
	robot := &Client{
		Room:       room,
		HandPokers: make([]int, 0, 21),
		UserInfo: &UserInfo{
			UserId:   table.newRobotID(),
			Username: fmt.Sprintf("ROBOT-%d", table.nextRobotSeq()),
			Coin:     common.StartCoin,
		},
		IsRobot:  true,
		BotLevel: room.BotLevel,
		inbox:    newRobotInbox(),
		toServer: make(chan []interface{}, 8),
	}
	go robot.runRobot()
	return robot
}

//机器人的编号只增不减，走了一个再补也不会重名
func (table *Table) nextRobotSeq() int {
	table.robotSeq++
	return table.robotSeq
}

//桌上已有几个机器人了
func (table *Table) robotCount() int {
	return len(table.robots())
}

//桌上的机器人
func (table *Table) robots() []*Client {
	list := make([]*Client, 0, 2)
	for _, client := range table.TableClients {
		if client.IsRobot {
			list = append(list, client)
		}
	}
	return list
}

//生成不和桌上玩家冲突的机器人ID，调用方必须持有牌桌锁。
//机器人用负数ID，和真人账号的ID空间分开，不会撞号。
func (table *Table) newRobotID() UserId {
	for i := 0; i < 100; i++ {
		robot := UserId(-1 - rand.Intn(10000))
		if _, ok := table.TableClients[robot]; !ok {
			return robot
		}
	}
	return UserId(-1 - rand.Intn(10000))
}

//发牌
func (table *Table) dealPoker() {
	if len(table.TableClients) != 3 || table.GameManage.FirstCallScore == nil {
		logs.Error("cannot deal cards: table[%d] requires three players and a first caller", table.TableId)
		return
	}
	logs.Debug("deal poker")
	table.GameManage.Pokers = make([]int, 0, 54)
	for i := 0; i < 54; i++ {
		table.GameManage.Pokers = append(table.GameManage.Pokers, i)
	}
	table.ShufflePokers()
	for _, client := range table.TableClients {
		client.HandPokers = make([]int, 0, 21)
		client.UserInfo.Role = RoleFarmer
		client.IsCalled = false
	}
	for i := 0; i < 17; i++ {
		for _, client := range table.TableClients {
			client.HandPokers = append(client.HandPokers, table.GameManage.Pokers[len(table.GameManage.Pokers)-1])
			table.GameManage.Pokers = table.GameManage.Pokers[:len(table.GameManage.Pokers)-1]
		}
	}
	for _, client := range table.TableClients {
		sort.Ints(client.HandPokers)
		response := []interface{}{
			common.ResDealPoker,
			table.GameManage.FirstCallScore.UserInfo.UserId,
			append([]int(nil), client.HandPokers...),
		}
		client.sendMsg(response)
	}
}

func (table *Table) chat(client *Client, msg string) {
	res := []interface{}{common.ResChat, client.UserInfo.UserId, msg}
	for _, c := range table.TableClients {
		c.sendMsg(res)
	}
}

func (table *Table) reset() {
	table.GameManage = &GameManage{
		FirstCallScore:   table.nextFirstCallScore(),
		Turn:             nil,
		MaxCallScore:     0,
		MaxCallScoreTurn: nil,
		LastShotClient:   nil,
		Pokers:           table.GameManage.Pokers[:0],
		LastShotPoker:    table.GameManage.LastShotPoker[:0],
		Multiple:         1,
	}
	table.State = GameCallScore
	restart := []interface{}{common.ResRestart, true}
	for _, client := range table.TableClients {
		client.sendMsg(restart)
	}
	for _, c := range table.TableClients {
		c.reset()
	}
	if len(table.TableClients) == 3 {
		table.dealPoker()
	}
}

//洗牌
func (table *Table) ShufflePokers() {
	logs.Debug("ShufflePokers")
	i := len(table.GameManage.Pokers)
	for i > 0 {
		randIndex := rand.Intn(i)
		table.GameManage.Pokers[i-1], table.GameManage.Pokers[randIndex] = table.GameManage.Pokers[randIndex], table.GameManage.Pokers[i-1]
		i--
	}
}

//同步用户信息
func (table *Table) syncUser() {
	logs.Debug("sync user")
	response := make([]interface{}, 0, 3)
	response = append(append(response, common.ResJoinTable), table.TableId)
	tableUsers := make([][2]interface{}, 0, 3)
	for _, current := range table.clientsInOrder() {
		tableUsers = append(tableUsers, [2]interface{}{current.UserInfo.UserId, current.UserInfo.Username})
	}
	response = append(response, tableUsers)
	for _, client := range table.TableClients {
		client.sendMsg(response)
	}
}
