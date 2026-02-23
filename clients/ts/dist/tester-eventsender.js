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
Object.defineProperty(exports, "__esModule", { value: true });
const child_process_1 = require("child_process");
const connection_1 = require("./connection");
const fs = __importStar(require("fs"));
const e5connectionOpt = {
    host: "localhost",
    port: 3535,
    instanceGroup: "instance-group",
    instanceId: "tester-secret",
    tlsEnabled: true,
    tlsOptions: {
        key: fs.readFileSync("../../certs/client.key"),
        cert: fs.readFileSync("../../certs/client.crt"),
        ca: fs.readFileSync("../../certs/ca.crt"),
    },
};
const senderConnection = new connection_1.Engine5Connection(Object.assign(Object.assign({}, e5connectionOpt), { instanceId: "sender-client" }));
const listenerConnection = new connection_1.Engine5Connection(Object.assign(Object.assign({}, e5connectionOpt), { instanceId: "listener-client" }));
senderConnection.init().then(() => {
    listenerConnection.init().then(() => {
        listenerConnection
            .listen("test.subject", (data) => {
            console.log("Received data on test.subject:", data);
            (0, child_process_1.exec)("kdialog --msgbox 'Event received: " + JSON.stringify(data) + "'");
        })
            .then(() => {
            senderConnection.sendEvent("test.subject", {
                message: "Hello, Engine5!",
            });
            console.log("Listener is set up and listening on test.subject");
        });
    });
});
