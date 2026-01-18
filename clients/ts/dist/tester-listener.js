"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const connection_1 = require("./connection");
const connection = new connection_1.Engine5Connection("localhost", 3535);
connection.init().then(() => {
    connection.listen("test.subject", (data) => {
        console.log("Received data on test.subject:", data);
    });
});
