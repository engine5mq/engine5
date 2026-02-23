import { exec } from "child_process";
import { Engine5Connection } from "./connection";
import * as fs from "fs";
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
const senderConnection = new Engine5Connection({
  ...e5connectionOpt,
  instanceId: "sender-client",
});
const listenerConnection = new Engine5Connection({
  ...e5connectionOpt,
  instanceId: "listener-client",
});

senderConnection.init().then(() => {
  listenerConnection.init().then(() => {
    listenerConnection
      .listen("test.subject", (data: any) => {
        console.log("Received data on test.subject:", data);
        exec("kdialog --msgbox 'Event received: " + JSON.stringify(data) + "'");
      })
      .then(() => {
        senderConnection.sendEvent("test.subject", {
          message: "Hello, Engine5!",
        });

        console.log("Listener is set up and listening on test.subject");
      });
  });
});
