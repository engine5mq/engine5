package main

import "github.com/google/uuid"

type VoidMethod func()

type QueueTicket struct {
	voidMethod VoidMethod
	completed  bool
	id         string
}

var GlobalTaskQueue = make(chan *QueueTicket)

// Worker
func loopGlobalTaskQueue() {
	for {
		select {
		case _, ok := <-GlobalTaskQueue:
			if ok {
				for task := range GlobalTaskQueue {
					task.voidMethod()
					task.completed = true
				}
			}

		}
	}

}

func addToGlobalTaskQueue(voidMethod VoidMethod) {
	id := uuid.NewString()
	ticket := QueueTicket{voidMethod: voidMethod, completed: false, id: id}
	GlobalTaskQueue <- &ticket
}

func addAndWaitToGlobalTaskQueue(voidMethod VoidMethod) {
	id := uuid.NewString()
	ticket := QueueTicket{voidMethod: voidMethod, completed: false, id: id}
	GlobalTaskQueue <- &ticket
	for {
		if ticket.completed {
			break
		}

	}
}
