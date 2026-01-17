"use strict";
var __createBinding = (this && this.__createBinding) || (Object.create ? (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    var desc = Object.getOwnPropertyDescriptor(m, k);
    if (!desc || ("get" in desc ? !m.__esModule : desc.writable || desc.configurable)) {
      desc = { enumerable: true, get: function() { return m[k]; } };
    }
    Object.defineProperty(o, k2, desc);
}) : (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    o[k2] = m[k];
}));
var __setModuleDefault = (this && this.__setModuleDefault) || (Object.create ? (function(o, v) {
    Object.defineProperty(o, "default", { enumerable: true, value: v });
}) : function(o, v) {
    o["default"] = v;
});
var __importStar = (this && this.__importStar) || (function () {
    var ownKeys = function(o) {
        ownKeys = Object.getOwnPropertyNames || function (o) {
            var ar = [];
            for (var k in o) if (Object.prototype.hasOwnProperty.call(o, k)) ar[ar.length] = k;
            return ar;
        };
        return ownKeys(o);
    };
    return function (mod) {
        if (mod && mod.__esModule) return mod;
        var result = {};
        if (mod != null) for (var k = ownKeys(mod), i = 0; i < k.length; i++) if (k[i] !== "default") __createBinding(result, mod, k[i]);
        __setModuleDefault(result, mod);
        return result;
    };
})();
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
Object.defineProperty(exports, "__esModule", { value: true });
exports.Engine5Connection = void 0;
// import net = require("net"); // import 'net' module
const net = __importStar(require("net"));
const msgpack_1 = require("@msgpack/msgpack");
const dynamic_queue_1 = require("@ubs-platform/dynamic-queue");
const rxjs_1 = require("rxjs");
class Engine5Connection {
    // onGoingRequests: Map<string, ((a: any) => any)[]> = new Map();
    constructor(host, port, instanceGroup, instanceId) {
        this.host = host;
        this.port = port;
        this.instanceGroup = instanceGroup;
        this.instanceId = instanceId;
        this.tcpClient = new net.Socket();
        this.connectionStatus = 'CLOSED';
        this.connectionStatusSubject = new rxjs_1.ReplaySubject(1);
        this.listeningSubjectCallbacks = {};
        this.ongoingRequestsToComplete = {};
        this.queue = new dynamic_queue_1.DynamicQueue();
        this.reconnectOnFail = true;
        this.tcpClientEventsRegistered = false;
        this.connectionStatusSubject.next('CLOSED');
        this.queue.push(() => __awaiter(this, void 0, void 0, function* () {
            yield this.runAtWhenConnected(() => {
                'This is for prevent add some events or requests before connection';
            });
        }));
        setInterval(() => {
            if (this.reconnectOnFail && this.connectionStatus == 'CLOSED') {
                console.info('Trying to establish a connection');
                this.init()
                    .then()
                    .catch((e) => console.error(e));
            }
        }, 5000);
    }
    runAtWhenConnected(ac) {
        let completed = false;
        return new Promise((ok, fail) => {
            let subscription = null;
            subscription = this.connectionStatusSubject.subscribe((a) => __awaiter(this, void 0, void 0, function* () {
                if (!completed && a == 'CONNECTED') {
                    completed = true;
                    subscription === null || subscription === void 0 ? void 0 : subscription.unsubscribe();
                    try {
                        const result = yield ac();
                        ok(result);
                    }
                    catch (e) {
                        fail(e);
                    }
                }
                else if (completed) {
                    subscription === null || subscription === void 0 ? void 0 : subscription.unsubscribe();
                }
            }));
        });
    }
    writePayload(p) {
        return __awaiter(this, void 0, void 0, function* () {
            return new Promise((ok, fail) => {
                this.queue.push(() => {
                    try {
                        // First 4 bytes are for length
                        const msgpackData = Buffer.from((0, msgpack_1.encode)(p));
                        const lengthPrefix = Buffer.alloc(4);
                        lengthPrefix.writeUInt32BE(msgpackData.length, 0);
                        const full = Buffer.concat([lengthPrefix, msgpackData]);
                        this.tcpClient.write(full, (e) => {
                            if (e)
                                fail(e);
                            else
                                ok(this);
                        });
                    }
                    catch (error) {
                        console.error(error);
                    }
                });
            });
        });
    }
    listen(subject, cb) {
        return __awaiter(this, void 0, void 0, function* () {
            console.info('Listening Subject: ' + subject);
            this.queue.push(() => __awaiter(this, void 0, void 0, function* () {
                yield this.writeListenCommand(subject);
                const ls = this.listeningSubjectCallbacks[subject] || [];
                ls.push(cb);
                this.listeningSubjectCallbacks[subject] = ls;
            }));
        });
    }
    writeListenCommand(subject) {
        return __awaiter(this, void 0, void 0, function* () {
            yield this.writePayload({
                Command: 'LISTEN',
                Subject: subject,
                MessageId: this.messageIdGenerate(),
            });
        });
    }
    messageIdGenerate() {
        return Date.now() + '_' + (Math.random() * 100000).toFixed();
    }
    sendRequest(subject, data) {
        return __awaiter(this, void 0, void 0, function* () {
            const messageId = this.messageIdGenerate();
            if (!(this.connectionStatus == 'CONNECTED')) {
                yield this.init();
            }
            yield this.writePayload({
                Command: 'REQUEST',
                Subject: subject,
                Content: this.stringifyData(data),
                MessageId: messageId,
            });
            return new Promise((ok, fail) => {
                this.ongoingRequestsToComplete[messageId] = (response) => {
                    if (response.Content) {
                        const jsonObj = this.parseData(response.Content);
                        ok(jsonObj);
                    }
                    else {
                        ok(undefined);
                    }
                };
            });
        });
    }
    sendEvent(subject, data) {
        return __awaiter(this, void 0, void 0, function* () {
            // this.runAtWhenConnected(async () => {
            // })
            // this.queue.push(() => {
            //     return new Promise((ok) => {
            //         this.connectionReady.subscribe((a) => {
            //             exec(`kdialog --msgbox "${a}"`)
            //             ok(null)
            //         })
            //     })
            // })
            yield this.writePayload({
                Command: 'EVENT',
                Subject: subject,
                Content: this.stringifyData(data),
            });
        });
    }
    // async sendEventStr(subject: string, data: string) {
    //   await this.writePayload({
    //     Command: "LISTEN",
    //     Subject: subject,
    //     Content: data,
    //   });
    // }
    init() {
        return __awaiter(this, void 0, void 0, function* () {
            return new Promise((ok, fail) => {
                if (this.connectionStatus == 'CLOSED') {
                    this._init((v) => {
                        ok(v);
                    });
                }
                else {
                    this.runAtWhenConnected(() => {
                        ok(this);
                    });
                }
            });
        });
    }
    _init(ok) {
        this.connectionStatusSubject.next('CONNECTING');
        this.connectionStatus = 'CONNECTING';
        console.info('Connecting to server');
        const client = this.tcpClient;
        this.registerEvents(client, ok);
        client.connect(parseInt(this.port), this.host, () => {
            //   client.write("I am Chappie");
            this.startConnection();
        });
    }
    registerEvents(client, ok) {
        if (this.tcpClientEventsRegistered)
            return;
        let currentBuff = [];
        let sizeBytes = [];
        let incomingLength = 0;
        let sizePrefixBuffer = null;
        client.on('data', (data) => {
            // console.info("Gelen data", data);
            this.queue.push(() => {
                let offset = 0;
                while (offset < data.length) {
                    if (sizeBytes.length < 4) {
                        // Read size prefix bytes
                        sizeBytes.push(data[offset]);
                        offset++;
                        if (sizeBytes.length == 4) {
                            incomingLength = Buffer.from(sizeBytes).readUInt32BE(0);
                        }
                    }
                    else {
                        // Read message bytes
                        const bytesNeeded = incomingLength - currentBuff.length;
                        const bytesAvailable = data.length - offset;
                        const bytesToRead = Math.min(bytesNeeded, bytesAvailable);
                        currentBuff.push(...data.subarray(offset, offset + bytesToRead));
                        offset += bytesToRead;
                        if (currentBuff.length == incomingLength) {
                            // We have a complete message
                            const messageBuffer = Buffer.from(currentBuff);
                            this.processIncomingData(messageBuffer, ok);
                            // Reset for next message
                            sizeBytes = [];
                            currentBuff = [];
                            incomingLength = 0;
                        }
                    }
                }
                // while
                // if (sizeBytes.length < 4) {
                //     // Read size prefix bytes
                //     for (let i = 0; i < data.length && sizeBytes.length < 4; i++) {
                // let offset = 0;
                while (offset < data.length) {
                    if (sizePrefixBuffer === null) {
                        // Read size prefix
                        sizePrefixBuffer = data.slice(offset, offset + 4);
                        offset += 4;
                    }
                    const messageSize = sizePrefixBuffer.readUInt32BE(0);
                    if (data.length - offset >= messageSize) {
                        // We have a complete message
                        const messageBuffer = data.slice(offset, offset + messageSize);
                        this.processIncomingData(messageBuffer, ok);
                        offset += messageSize;
                        sizePrefixBuffer = null; // Reset for next message
                    }
                    else {
                        // Not enough data for a complete message
                        break;
                    }
                }
                // let newBuffData = [...currentBuff, ...data];
                // let splitByteIndex = newBuffData.indexOf(4);
                // while (splitByteIndex > -1) {
                //     const popped = newBuffData.slice(0, splitByteIndex);
                //     this.processIncomingData(popped, ok);
                //     newBuffData = newBuffData.slice(splitByteIndex + 1);
                //     splitByteIndex = newBuffData.indexOf(4);
                // }
                // currentBuff = newBuffData;
            });
        });
        client.on('error', (err) => {
            console.error(`Error occured ${err}`);
        });
        client.on('close', () => __awaiter(this, void 0, void 0, function* () {
            this.connectionStatus = 'CLOSED';
            this.connectionStatusSubject.next('CLOSED');
            console.log('Connection closed');
        }));
        this.tcpClientEventsRegistered = true;
    }
    startConnection() {
        this.writePayload({
            Command: 'CONNECT',
            InstanceId: this.instanceId || '',
            InstanceGroup: this.instanceGroup || this.instanceId,
        });
        const alreadyListeningSubjects = Object.keys(this.listeningSubjectCallbacks);
        for (let alsIndex = 0; alsIndex < alreadyListeningSubjects.length; alsIndex++) {
            const als = alreadyListeningSubjects[alsIndex];
            this.writeListenCommand(als)
                .then(() => console.info('Listening subject again: ' + als))
                .catch(console.error);
        }
    }
    processIncomingData(data, promiseResolveFunc) {
        return __awaiter(this, void 0, void 0, function* () {
            const decoded = (0, msgpack_1.decode)(data);
            // console.info(decoded)
            if (decoded.Command == 'CONNECT_SUCCESS') {
                this.connectionStatus = 'CONNECTED';
                this.connectionStatusSubject.next('CONNECTED');
                this.instanceId = decoded.InstanceId;
                this.instanceGroup = decoded.InstanceGroup;
                promiseResolveFunc === null || promiseResolveFunc === void 0 ? void 0 : promiseResolveFunc(this);
                // this.reconnectOnFail = true;
                console.info('Connected Successfully');
            }
            else if (decoded.Command == 'EVENT') {
                console.info('Event recieved', decoded.Subject);
                this.processReceivedEvent(decoded);
            }
            else if (decoded.Command == 'REQUEST') {
                console.info('Request recieved: ', decoded.Subject);
                try {
                    const ac = yield this.listeningSubjectCallbacks[decoded.Subject][0](this.parseData(decoded.Content));
                    // this.ongoingRequestsToComplete[decoded.MessageId!](ac)
                    yield this.writePayload({
                        Command: 'RESPONSE',
                        Content: this.stringifyData(ac),
                        MessageId: this.messageIdGenerate(),
                        Subject: decoded.Subject,
                        ResponseOfMessageId: decoded.MessageId,
                    });
                }
                catch (ex) {
                    console.error(ex);
                }
            }
            else if (decoded.Command == 'RESPONSE') {
                this.ongoingRequestsToComplete[decoded.ResponseOfMessageId](decoded);
            }
        });
    }
    parseData(dataString) {
        if (dataString[0] == 'undefined')
            return undefined;
        return JSON.parse(dataString);
    }
    stringifyData(ac) {
        let a = JSON.stringify(ac);
        // // her 1000 karakterde bir bölelim
        // const chunkSize = 1000;
        // const chunks: string[] = [];
        // if (a?.length) {
        //     for (let i = 0; i < a.length; i += chunkSize) {
        //         chunks.push(a.substring(i, i + chunkSize));
        //     }
        // } else {
        //     chunks.push('undefined');
        // }
        // stringleri bölmek şu anda '4' karakteri sorununa çözüm değil. Ancak ileride farklı bir protokole geçildiğinde sorun olmayacak.
        return a;
    }
    processReceivedEvent(decoded) {
        const cbs = this.listeningSubjectCallbacks[decoded.Subject] || [];
        for (let callbackIndex = 0; callbackIndex < cbs.length; callbackIndex++) {
            const callback = cbs[callbackIndex];
            callback(this.parseData(decoded.Content));
        }
    }
    close() {
        return __awaiter(this, void 0, void 0, function* () {
            console.info('E5JSCL - Connection is about to be closed');
            this.reconnectOnFail = false;
            yield this.writePayload({ Command: 'CLOSE' });
        });
    }
    static create(host, port, instanceGroup, instanceId) {
        const key = `${instanceGroup}(${instanceId})@${host}:${port}`;
        if (!this.globalE5Connections[key]) {
            const nk = new Engine5Connection(host, port, instanceGroup, instanceId);
            this.globalE5Connections[key] = nk;
        }
        return this.globalE5Connections[key];
    }
}
exports.Engine5Connection = Engine5Connection;
Engine5Connection.globalE5Connections = {};
