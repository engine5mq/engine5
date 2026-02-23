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
const net = __importStar(require("net"));
const tls = __importStar(require("node:tls"));
const msgpack_1 = require("@msgpack/msgpack");
const dynamic_queue_1 = require("./dynamic-queue");
const rxjs_1 = require("rxjs");
class Engine5Connection {
    constructor(connectOptions) {
        var _a;
        this.connectOptions = connectOptions;
        this.tcpClient = new net.Socket();
        this.connectionStatus = "CLOSED";
        this.connectionStatusSubject = new rxjs_1.ReplaySubject(1);
        this.listeningSubjectCallbacks = {};
        this.ongoingRequestsToComplete = {};
        this.queue = new dynamic_queue_1.DynamicQueue();
        this.reconnectOnFail = true;
        this.tcpClientEventsRegistered = false;
        this.reconnectInterval = null;
        this.tlsEnabled = false;
        this.host = connectOptions.host;
        this.port = connectOptions.port;
        this.instanceGroup = connectOptions.instanceGroup;
        this.instanceId = connectOptions.instanceId;
        this.tlsEnabled = (_a = connectOptions.tlsEnabled) !== null && _a !== void 0 ? _a : false;
        this.tlsOptions = connectOptions.tlsOptions;
        this.connectionStatusSubject.next("CLOSED");
        this.queue.push(() => __awaiter(this, void 0, void 0, function* () {
            yield this.runAtWhenConnected(() => {
                // Initialize connection preparation
            });
        }));
        this.startReconnectTimer();
    }
    startReconnectTimer() {
        this.reconnectInterval = setInterval(() => {
            if (this.reconnectOnFail && this.connectionStatus === "CLOSED") {
                console.info("Attempting to reconnect...");
                this.init().catch((error) => {
                    console.error("Reconnection failed:", error);
                });
            }
        }, 5000);
    }
    runAtWhenConnected(action) {
        return new Promise((resolve, reject) => {
            if (this.connectionStatus === "CONNECTED") {
                try {
                    const result = action();
                    if (result instanceof Promise) {
                        result.then(resolve).catch(reject);
                    }
                    else {
                        resolve(result);
                    }
                }
                catch (error) {
                    reject(error);
                }
                return;
            }
            const subscription = this.connectionStatusSubject.subscribe((status) => __awaiter(this, void 0, void 0, function* () {
                if (status === "CONNECTED") {
                    subscription.unsubscribe();
                    try {
                        const result = yield action();
                        resolve(result);
                    }
                    catch (error) {
                        reject(error);
                    }
                }
            }));
        });
    }
    writePayload(payload) {
        return __awaiter(this, void 0, void 0, function* () {
            return new Promise((resolve, reject) => {
                this.queue.push(() => {
                    try {
                        const msgpackData = Buffer.from((0, msgpack_1.encode)(payload));
                        const lengthPrefix = Buffer.alloc(4);
                        lengthPrefix.writeUInt32BE(msgpackData.length, 0);
                        const fullMessage = Buffer.concat([lengthPrefix, msgpackData]);
                        this.tcpClient.write(fullMessage, (error) => {
                            if (error) {
                                console.error("Failed to write payload:", error);
                                reject(error);
                            }
                            else {
                                resolve(this);
                            }
                        });
                    }
                    catch (error) {
                        console.error("Error encoding payload:", error);
                        reject(error);
                    }
                });
            });
        });
    }
    listen(subject, callback) {
        return __awaiter(this, void 0, void 0, function* () {
            if (!subject) {
                throw new Error("Subject cannot be empty");
            }
            console.info(`Listening to subject: ${subject}`);
            this.queue.push(() => __awaiter(this, void 0, void 0, function* () {
                try {
                    yield this.writeListenCommand(subject);
                    const callbacks = this.listeningSubjectCallbacks[subject] || [];
                    callbacks.push(callback);
                    this.listeningSubjectCallbacks[subject] = callbacks;
                }
                catch (error) {
                    console.error(`Failed to listen to subject ${subject}:`, error);
                    throw error;
                }
            }));
        });
    }
    writeListenCommand(subject) {
        return __awaiter(this, void 0, void 0, function* () {
            yield this.writePayload({
                Command: "LISTEN",
                Subject: subject,
                MessageId: this.generateMessageId(),
            });
        });
    }
    generateMessageId() {
        return `${Date.now()}_${Math.floor(Math.random() * 1000000)}`;
    }
    sendRequest(subject, data) {
        return __awaiter(this, void 0, void 0, function* () {
            if (!subject) {
                throw new Error("Subject cannot be empty");
            }
            const messageId = this.generateMessageId();
            if (this.connectionStatus !== "CONNECTED") {
                yield this.init();
            }
            yield this.writePayload({
                Command: "REQUEST",
                Subject: subject,
                Content: this.stringifyData(data),
                MessageId: messageId,
            });
            return new Promise((resolve, reject) => {
                const timeout = setTimeout(() => {
                    delete this.ongoingRequestsToComplete[messageId];
                    reject(new Error(`Request timeout for subject: ${subject}`));
                }, 30000); // 30 second timeout
                this.ongoingRequestsToComplete[messageId] = (response) => {
                    clearTimeout(timeout);
                    try {
                        const result = response.Content
                            ? this.parseData(response.Content)
                            : undefined;
                        resolve(result);
                    }
                    catch (error) {
                        reject(error);
                    }
                };
            });
        });
    }
    sendEvent(subject, data) {
        return __awaiter(this, void 0, void 0, function* () {
            if (!subject) {
                throw new Error("Subject cannot be empty");
            }
            if (this.connectionStatus !== "CONNECTED") {
                throw new Error("Not connected to the server");
            }
            yield this.writePayload({
                Command: "EVENT",
                Subject: subject,
                Content: this.stringifyData(data),
            });
        });
    }
    init() {
        return __awaiter(this, void 0, void 0, function* () {
            return new Promise((ok, fail) => {
                if (this.connectionStatus == "CLOSED") {
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
        var _a, _b, _c, _d, _e, _f, _g;
        this.connectionStatusSubject.next("CONNECTING");
        this.connectionStatus = "CONNECTING";
        console.info("Connecting to server");
        if (this.tlsEnabled) {
            const tlsSocket = tls.connect({
                host: this.host,
                port: Number(this.port),
                ca: (_a = this.tlsOptions) === null || _a === void 0 ? void 0 : _a.ca,
                cert: (_b = this.tlsOptions) === null || _b === void 0 ? void 0 : _b.cert,
                key: (_c = this.tlsOptions) === null || _c === void 0 ? void 0 : _c.key,
                servername: (_e = (_d = this.tlsOptions) === null || _d === void 0 ? void 0 : _d.servername) !== null && _e !== void 0 ? _e : this.host,
                rejectUnauthorized: (_g = (_f = this.tlsOptions) === null || _f === void 0 ? void 0 : _f.rejectUnauthorized) !== null && _g !== void 0 ? _g : true,
            });
            this.tcpClient = tlsSocket;
            this.tcpClientEventsRegistered = false;
            this.registerEvents(tlsSocket, ok);
            tlsSocket.once("secureConnect", () => {
                this.startConnection();
            });
            return;
        }
        this.tcpClient = new net.Socket();
        this.tcpClientEventsRegistered = false;
        const client = this.tcpClient;
        this.registerEvents(client, ok);
        client.connect({
            host: this.host,
            port: Number(this.port),
        }, () => {
            this.startConnection();
        });
    }
    registerEvents(client, ok) {
        if (this.tcpClientEventsRegistered)
            return;
        let currentBuff = [];
        let sizeBytes = [];
        let incomingLength = 0;
        client.on("data", (data) => {
            this.queue.push(() => {
                let offset = 0;
                while (offset < data.length) {
                    if (sizeBytes.length < 4) {
                        // Read size prefix bytes
                        sizeBytes.push(data[offset]);
                        offset++;
                        if (sizeBytes.length === 4) {
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
                        if (currentBuff.length === incomingLength) {
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
            });
        });
        client.on("error", (err) => {
            console.error(`Error occured ${err}`);
        });
        client.on("close", () => __awaiter(this, void 0, void 0, function* () {
            this.connectionStatus = "CLOSED";
            this.connectionStatusSubject.next("CLOSED");
            console.log("Connection closed");
        }));
        client.on("session", () => {
            console.info("TLS session established");
        });
        this.tcpClientEventsRegistered = true;
    }
    startConnection() {
        var _a;
        // if (this.connectionStatus != "CONNECTING") return;
        if (this.tlsEnabled) {
            const tlsSocket = this.tcpClient;
            if (tlsSocket.authorizationError) {
                if (((_a = this.tlsOptions) === null || _a === void 0 ? void 0 : _a.rejectUnauthorized) === false) {
                    console.warn("TLS authorization warning (ignored by configuration): " +
                        tlsSocket.authorizationError);
                }
                else {
                    console.error("TLS authorization error: " + tlsSocket.authorizationError);
                    this.tcpClient.destroy();
                    return;
                }
            }
        }
        this.writePayload({
            Command: "CONNECT",
            InstanceId: this.instanceId || "",
            InstanceGroup: this.instanceGroup || this.instanceId,
        });
        const alreadyListeningSubjects = Object.keys(this.listeningSubjectCallbacks);
        for (let alsIndex = 0; alsIndex < alreadyListeningSubjects.length; alsIndex++) {
            const als = alreadyListeningSubjects[alsIndex];
            this.writeListenCommand(als)
                .then(() => console.info("Listening subject again: " + als))
                .catch(console.error);
        }
    }
    processIncomingData(data, promiseResolveFunc) {
        return __awaiter(this, void 0, void 0, function* () {
            const decoded = (0, msgpack_1.decode)(data);
            // console.info(decoded)
            if (decoded.Command == "CONNECT_SUCCESS") {
                this.connectionStatus = "CONNECTED";
                this.connectionStatusSubject.next("CONNECTED");
                this.instanceId = decoded.InstanceId;
                this.instanceGroup = decoded.InstanceGroup;
                promiseResolveFunc === null || promiseResolveFunc === void 0 ? void 0 : promiseResolveFunc(this);
                // this.reconnectOnFail = true;
                console.info("Connected Successfully");
            }
            else if (decoded.Command == "EVENT") {
                console.info("Event recieved", decoded.Subject);
                this.processReceivedEvent(decoded);
            }
            else if (decoded.Command == "REQUEST") {
                console.info("Request recieved: ", decoded.Subject);
                try {
                    const ac = yield this.listeningSubjectCallbacks[decoded.Subject][0](this.parseData(decoded.Content));
                    // this.ongoingRequestsToComplete[decoded.MessageId!](ac)
                    yield this.writePayload({
                        Command: "RESPONSE",
                        Content: this.stringifyData(ac),
                        MessageId: this.generateMessageId(),
                        Subject: decoded.Subject,
                        ResponseOfMessageId: decoded.MessageId,
                    });
                }
                catch (ex) {
                    console.error(ex);
                }
            }
            else if (decoded.Command == "RESPONSE") {
                this.ongoingRequestsToComplete[decoded.ResponseOfMessageId](decoded);
            }
        });
    }
    parseData(dataString) {
        if (dataString === "undefined" || dataString === "") {
            return undefined;
        }
        try {
            return JSON.parse(dataString);
        }
        catch (error) {
            console.error("Failed to parse JSON data:", error);
            return dataString; // Return original string if parsing fails
        }
    }
    stringifyData(data) {
        if (data === undefined) {
            return "undefined";
        }
        try {
            return JSON.stringify(data);
        }
        catch (error) {
            console.error("Failed to stringify data:", error);
            return String(data);
        }
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
            console.info("Closing Engine5 connection...");
            this.reconnectOnFail = false;
            if (this.reconnectInterval) {
                clearInterval(this.reconnectInterval);
                this.reconnectInterval = null;
            }
            try {
                if (this.connectionStatus === "CONNECTED") {
                    yield this.writePayload({ Command: "CLOSE" });
                }
            }
            catch (error) {
                console.error("Error during close:", error);
            }
            finally {
                this.tcpClient.destroy();
                this.connectionStatus = "CLOSED";
                this.connectionStatusSubject.next("CLOSED");
            }
        });
    }
    static create(connectOptions) {
        const { host, port, instanceGroup = "default-group", instanceId = "default-id", } = connectOptions;
        const key = `${instanceGroup}(${instanceId})@${host}:${port}`;
        if (!this.globalE5Connections[key]) {
            const nk = new Engine5Connection(connectOptions);
            this.globalE5Connections[key] = nk;
        }
        return this.globalE5Connections[key];
    }
}
exports.Engine5Connection = Engine5Connection;
Engine5Connection.globalE5Connections = {};
