import { Engine5Connection } from "./connection";

const connection = new Engine5Connection("localhost", 3535);
connection.init().then(() => {
  connection.sendEvent("test.subject", { message: "Hello, Engine5!" });
});
