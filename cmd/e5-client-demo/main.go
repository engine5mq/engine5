package main

import (
	"engine5/internal/common"
	"flag"
	"fmt"
)

func main() {
	cfg := common.GetClientConfig()
	host := flag.String("host", cfg.Host, "Engine5 host")
	port := flag.Int("port", cfg.Port, "Engine5 port")
	instanceID := flag.String("instance-id", cfg.InstanceID, "client instance id")
	instanceGroup := flag.String("instance-group", cfg.InstanceGroup, "client instance group")
	subject := flag.String("subject", cfg.Subject, "subject to listen/send")
	tlsEnabled := flag.Bool("tls", cfg.TLSEnabled, "enable TLS")
	insecure := flag.Bool("insecure", false, "skip TLS certificate verification")
	requestDemo := flag.Bool("request", false, "send a demo request after connect")
	flag.Parse()

	fmt.Println("e5-client-demo: Go client SDK is not present in this repository.")
	fmt.Printf("config => host=%s port=%d instance-id=%s instance-group=%s subject=%s tls=%v insecure=%v request=%v\n",
		*host, *port, *instanceID, *instanceGroup, *subject, *tlsEnabled, *insecure, *requestDemo)
}
