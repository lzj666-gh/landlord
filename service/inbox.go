package service

import "sync"

//机器人的消息收件箱。牌桌是在持有锁的情况下给机器人发消息的，
//如果直接往有界 channel 里塞，队列满了就会卡住持锁的 goroutine，
//而机器人又需要同一把锁才能继续处理，双方互等。这里用不设上限的队列，
//发消息永远不阻塞。
type robotInbox struct {
	mu     sync.Mutex
	cond   *sync.Cond
	msgs   [][]interface{}
	closed bool
}

func newRobotInbox() *robotInbox {
	inbox := &robotInbox{}
	inbox.cond = sync.NewCond(&inbox.mu)
	return inbox
}

func (inbox *robotInbox) push(msg []interface{}) {
	inbox.mu.Lock()
	if !inbox.closed {
		inbox.msgs = append(inbox.msgs, msg)
		inbox.cond.Signal()
	}
	inbox.mu.Unlock()
}

//取一条消息，收件箱关闭且取空后返回 false
func (inbox *robotInbox) pop() ([]interface{}, bool) {
	inbox.mu.Lock()
	defer inbox.mu.Unlock()
	for len(inbox.msgs) == 0 && !inbox.closed {
		inbox.cond.Wait()
	}
	if len(inbox.msgs) == 0 {
		return nil, false
	}
	msg := inbox.msgs[0]
	inbox.msgs = inbox.msgs[1:]
	return msg, true
}

func (inbox *robotInbox) close() {
	inbox.mu.Lock()
	inbox.closed = true
	inbox.cond.Broadcast()
	inbox.mu.Unlock()
}
