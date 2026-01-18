import { Engine5Connection } from "./connection";

const connection = new Engine5Connection("localhost", 3535);
connection.init().then(() => {
    connection.listen("test.subject", (data: any) => {
        console.log("Received data on test.subject:", data);
    });
});