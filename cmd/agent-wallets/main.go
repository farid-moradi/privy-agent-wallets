// An AI agent with its own wallets, on a leash.
//
// setup:      creates a spending policy + one EVM and one Solana wallet via Privy
// run:        drives a Claude tool-use loop that pays invoices inside the policy's bounds
// force-send: simulates a compromised agent so the policy denial can be observed
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/farid-moradi/privy-agent-wallets/pkg/agent"
	"github.com/farid-moradi/privy-agent-wallets/pkg/wallet"
)

// The one address the agent is allowed to pay, and the per-transaction cap.
// On a real deployment these come from your config, not constants.
const (
	treasuryAddr = "0x1B9aB6d9E9dBcD98BA47649aB014fC0eb4D435f1" // demo recipient
	maxWeiPerTx  = "5000000000000000"                           // 0.005 ETH
	stateFile    = "wallets.json"
)

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 {
		log.Fatalf("usage: agent-wallets <setup|run \"task\"|force-send <to> <amount_eth>>")
	}

	client := wallet.New(mustEnv("PRIVY_APP_ID"), mustEnv("PRIVY_APP_SECRET"))
	ctx := context.Background()

	switch os.Args[1] {
	case "setup":
		s, err := client.Setup(ctx, wallet.Config{
			TreasuryAddr: treasuryAddr,
			MaxWeiPerTx:  maxWeiPerTx,
		})
		if err != nil {
			log.Fatalf("setup: %v", err)
		}
		if err := s.Save(stateFile); err != nil {
			log.Fatalf("setup: %v", err)
		}
		fmt.Printf("policy   %s\n", s.EVMPolicyID)
		fmt.Printf("ethereum %s  (id %s)\n", s.EVMAddress, s.EVMWalletID)
		fmt.Printf("solana   %s  (id %s)\n", s.SolAddress, s.SolWalletID)
		fmt.Printf("\nwrote %s. Fund the EVM wallet with Base Sepolia ETH and run the agent.\n", stateFile)

	case "run":
		if len(os.Args) < 3 {
			log.Fatalf("usage: agent-wallets run \"<task for the agent>\"")
		}
		s := mustState()
		if err := agent.Run(ctx, client, s, strings.Join(os.Args[2:], " ")); err != nil {
			log.Fatalf("run: %v", err)
		}

	case "force-send":
		// Simulates a fully compromised agent: calls the send tool directly,
		// no model judgment in the loop. Only the wallet policy stands between
		// this command and the funds.
		if len(os.Args) < 4 {
			log.Fatalf("usage: agent-wallets force-send <to> <amount_eth>")
		}
		s := mustState()
		out, err := client.SendETH(ctx, s.EVMWalletID, wallet.BaseSepoliaCAIP2, os.Args[2], os.Args[3])
		if err != nil {
			log.Fatalf("%v", err)
		}
		fmt.Println(out)

	default:
		log.Fatalf("unknown command %q", os.Args[1])
	}
}

func mustState() wallet.State {
	s, err := wallet.LoadState(stateFile)
	if err != nil {
		log.Fatal(err)
	}
	return s
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing required env var %s", key)
	}
	return v
}
