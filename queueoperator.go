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
		for task := range GlobalTaskQueue {
			task.voidMethod()
			task.completed = true
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
	var count = 0
	for {
		if ticket.completed {
			break
		} else {
			count++
			if count > 0 && (count%1000) == 0 {
				println("ATTENTION: A task has not been resolved because it is continuing for long")
				break
			}
		}

	}
}

// Task gönderme

// taskQueue <- func() { fmt.Println("Yazma") }
