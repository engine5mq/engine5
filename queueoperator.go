package main;

type voidy func void();
var GlobalTaskQueue = make(chan func())


// Worker
func loopGlobalTaskQueue() {
	for task := range taskQueue {
		task()
	}
}

func addToGlobalTaskQueue() {

}

// Task gönderme
taskQueue <- func() { fmt.Println("Okuma") }
taskQueue <- func() { fmt.Println("Yazma") }
