package service

import (
	"github.com/astaxie/beego/logs"
	"sort"
	"sync"
)

var (
	roomManager = RoomManager{
		Rooms: map[int]*Room{
			1: {
				RoomId:      1,
				Name:        "人机练习",
				Desc:        "和机器人练手，随时可以补齐三个人",
				AllowRobot:  true,
				BotLevel:    defaultBotLevel(),
				EntranceFee: 500,
				Tables:      make(map[TableId]*Table),
			},
			2: {
				RoomId:      2,
				Name:        "多人对战",
				Desc:        "和好友开一桌，人不够也能加机器人",
				AllowRobot:  true,
				BotLevel:    defaultBotLevel(),
				EntranceFee: 500,
				Tables:      make(map[TableId]*Table),
			},
		},
	}
)

type RoomId int

type RoomManager struct {
	Lock       sync.RWMutex
	Rooms      map[int]*Room
	TableIdInc TableId
}

type Room struct {
	RoomId      RoomId
	Name        string
	Desc        string
	Lock        sync.RWMutex
	AllowRobot  bool     //允许玩家手动添加机器人补齐人数
	BotLevel    botLevel //本房间机器人的默认难度
	Tables      map[TableId]*Table
	EntranceFee int
}

//房间列表页要用的房间概况
type RoomInfo struct {
	RoomId      int
	Name        string
	Desc        string
	AllowRobot  bool
	Tables      int //牌桌数
	Players     int //桌上的真人
	EntranceFee int //底注
}

//当前所有房间的概况，按房间号排序
func RoomsInfo() []RoomInfo {
	roomManager.Lock.RLock()
	defer roomManager.Lock.RUnlock()
	infos := make([]RoomInfo, 0, len(roomManager.Rooms))
	for _, room := range roomManager.Rooms {
		room.Lock.RLock()
		info := RoomInfo{
			RoomId:      int(room.RoomId),
			Name:        room.Name,
			Desc:        room.Desc,
			AllowRobot:  room.AllowRobot,
			Tables:      len(room.Tables),
			EntranceFee: room.EntranceFee,
		}
		for _, table := range room.Tables {
			table.Lock.RLock()
			info.Players += table.humanCount()
			table.Lock.RUnlock()
		}
		room.Lock.RUnlock()
		infos = append(infos, info)
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].RoomId < infos[j].RoomId })
	return infos
}

//房间是否存在
func RoomExists(roomId int) bool {
	roomManager.Lock.RLock()
	defer roomManager.Lock.RUnlock()
	_, ok := roomManager.Rooms[roomId]
	return ok
}

//新建牌桌
func (r *Room) newTable(client *Client) (table *Table) {
	roomManager.Lock.Lock()
	defer roomManager.Lock.Unlock()

	r.Lock.Lock()
	defer r.Lock.Unlock()
	roomManager.TableIdInc = roomManager.TableIdInc + 1
	table = &Table{
		TableId:      roomManager.TableIdInc,
		Creator:      client,
		TableClients: make(map[UserId]*Client, 3),
		GameManage: &GameManage{
			FirstCallScore: client,
			Multiple:       1,
			LastShotPoker:  make([]int, 0),
			Pokers:         make([]int, 0, 54),
		},
	}
	r.Tables[table.TableId] = table
	logs.Debug("create new table ok! allow robot :%v", r.AllowRobot)
	return
}

//func init()  {
//	go func() {		//压测
//		time.Sleep(time.Second * 3)
//		for i:=0;i<1;i++{
//			client := &Client{
//				Room:       roomManager.Rooms[1],
//				HandPokers: make([]int, 0, 21),
//				UserInfo: &UserInfo{
//					UserId:   UserId(rand.Intn(10000)),
//					Username: "ROBOT-0",
//					Coin:     10000,
//				},
//				IsRobot:  true,
//				toRobot: make(chan []interface{}, 3),
//				toServer: make(chan []interface{}, 3),
//			}
//			go client.runRobot()
//			table := client.Room.newTable(client)
//			table.joinTable(client)
//		}
//	}()
//}
