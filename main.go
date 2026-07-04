// An AI agent with its own wallets, on a leash.
//
// setup: creates a spending policy + one EVM and one Solana wallet via Privy
// run:   drives a Claude tool-use loop that can check prices, check balances,
//        and pay invoices from the EVM wallet — inside the policy's bounds.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	privy "github.com/privy-io/go-sdk"
)

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 {
		log.Fatalf("usage: agent-wallets <setup|run \"task\">")
	}

	client := privy.NewPrivyClient(privy.PrivyClientOptions{
		AppID:     mustEnv("PRIVY_APP_ID"),
		AppSecret: mustEnv("PRIVY_APP_SECRET"),
	})
	ctx := context.Background()

	switch os.Args[1] {
	case "setup":
		if err := setup(ctx, client); err != nil {
			log.Fatalf("setup: %v", err)
		}
	case "run":
		if len(os.Args) < 3 {
			log.Fatalf("usage: agent-wallets run \"<task for the agent>\"")
		}
		if err := runAgent(ctx, client, strings.Join(os.Args[2:], " ")); err != nil {
			log.Fatalf("run: %v", err)
		}
	case "force-send":
		// Simulates a fully compromised agent: calls the send tool directly,
		// no model judgment in the loop. Only the wallet policy stands between
		// this command and the funds.
		if len(os.Args) < 4 {
			log.Fatalf("usage: agent-wallets force-send <to> <amount_eth>")
		}
		s, err := loadState()
		if err != nil {
			log.Fatal(err)
		}
		out, err := sendETH(ctx, client, s.EVMWalletID, os.Args[2], os.Args[3])
		if err != nil {
			log.Fatalf("%v", err)
		}
		fmt.Println(out)
	default:
		log.Fatalf("unknown command %q", os.Args[1])
	}
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing required env var %s", key)
	}
	return v
}

func fmtWallet(w *privy.Wallet) string {
	return fmt.Sprintf("%-8s %s  (id %s)", w.ChainType, w.Address, w.ID)
}
