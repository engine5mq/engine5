package main

type TaskQueue struct {
	tasks      chan func()
	taskInsert chan bool
}

func NewTaskQueue(workerCount int) *TaskQueue {
	tq := &TaskQueue{
		tasks: make(chan func(), 100),
	}
	for i := 0; i < workerCount; i++ {
		go tq.worker()
	}
	return tq
}

func (tq *TaskQueue) worker() {
	for {
		for task := range tq.tasks {
			task()
		}
		for range tq.taskInsert {
			break
		}
	}

}

func (tq *TaskQueue) Enqueue(task func()) {
	go func() {
		tq.tasks <- task
	}()
	go func() {
		tq.taskInsert <- true
	}()
}
