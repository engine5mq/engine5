"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.DynamicQueue = void 0;
// Import stylesheets
const rxjs_1 = require("rxjs");
/**
 * DynamicQueue holds observables and starts in a order.
 * Another observables can be dynamically added and it will be runned when its turn
 */
class DynamicQueue {
    /**
     * Constructor: Startup
     * DynamicQueue holds observables and starts in a order.
     * Another observables can be dynamically added and it will be runned when its turn
     */
    constructor() {
        /**
         * Tasks are holding with a key and output subject
         */
        this._tasks = [];
        /**
         * When it is running, it is true
         */
        this._busy = false;
        /**
         * When it is runned its get changed immediately and notifies its subscribers
         */
        this.busyChange = new rxjs_1.ReplaySubject(1);
        this.setBusy(false);
    }
    /**
     * Busy field as public
     */
    get busy() {
        return this._busy;
    }
    /**
     * set busy field internally
     */
    setBusy(b) {
        this._busy = b;
        this.busyChange.next(b);
    }
    /**
     * A new task will be runned when its turn
     * @param newTask - a task
     * @returns a information with subscriber and key
     */
    push(newTask_) {
        let newTask;
        if (newTask_ instanceof Function) {
            newTask = new rxjs_1.Observable((subscriber) => {
                try {
                    const result = newTask_();
                    if (result instanceof Promise) {
                        result
                            .then((a) => subscriber.next(a))
                            .catch((e) => subscriber.error(e));
                    }
                    else {
                        subscriber.next(result);
                    }
                }
                catch (error) {
                    subscriber.error(error);
                }
                subscriber.complete();
            });
        }
        else {
            newTask = (0, rxjs_1.from)(newTask_);
        }
        const task = {
            key: this._tasks.length,
            actualTask: newTask,
            outputSubject: new rxjs_1.Subject(),
        };
        this._tasks.push(task);
        //when it is running, it is never touched. otherwise, it will get started
        if (!this.busy) {
            this.setBusy(true);
            this.runFirst();
        }
        return {
            key: task.key,
            output: task.outputSubject.asObservable(),
        };
    }
    /**
     * Pulls a task and runs the task. When its over, calls itself for new.
     */
    runFirst() {
        var _a;
        const firstStart = (_a = this._tasks.splice(0, 1)) === null || _a === void 0 ? void 0 : _a[0];
        if (firstStart) {
            firstStart.actualTask.subscribe({
                next: (a) => {
                    firstStart.outputSubject.next(a);
                },
                error: (error) => {
                    firstStart.outputSubject.error(error);
                },
                complete: () => {
                    firstStart.outputSubject.complete();
                    this.runFirst();
                },
            });
        }
        else {
            this.setBusy(false);
        }
    }
}
exports.DynamicQueue = DynamicQueue;
