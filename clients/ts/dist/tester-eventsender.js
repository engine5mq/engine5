"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const connection_1 = require("./connection");
const connection = new connection_1.Engine5Connection("localhost", 3535);
connection.init().then(() => {
    connection.sendEvent("test.subject", { message: "Hello, Engine5!" });
});
