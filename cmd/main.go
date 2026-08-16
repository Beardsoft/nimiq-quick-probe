package main

import (
	"log"
	"net/http"

	"github.com/Beardsoft/nimiq-quick-probe/internal/probe"
)

func main() {
	cfg, err := probe.LoadConfig()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	client := probe.NewRPCClient(cfg.RPCURL, cfg.RPCTimeout)
	handler := probe.NewHandler(cfg, client)

	log.Printf("listening on %s, rpc %s", cfg.ListenAddr, cfg.RPCURL)
	if err := http.ListenAndServe(cfg.ListenAddr, handler); err != nil {
		log.Fatal(err)
	}
}
