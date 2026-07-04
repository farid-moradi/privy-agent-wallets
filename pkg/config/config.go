// Package config resolves the runtime environment: which network the agent
// operates on and what the spending guardrails are.
package config

import (
	"fmt"
	"os"
)

// Network identifies a chain for Privy (CAIP-2) and for direct RPC reads.
type Network struct {
	Name   string
	CAIP2  string
	RPCURL string
}

var networks = map[string]Network{
	"dev":        {Name: "Base Sepolia (a testnet)", CAIP2: "eip155:84532", RPCURL: "https://sepolia.base.org"},
	"production": {Name: "Base mainnet", CAIP2: "eip155:8453", RPCURL: "https://mainnet.base.org"},
}

type Config struct {
	Env          string
	Network      Network
	TreasuryAddr string
	MaxWeiPerTx  string
	StateFile    string
}

const (
	// Defaults apply in dev only. Production must configure its own treasury.
	devTreasuryAddr   = "0x1B9aB6d9E9dBcD98BA47649aB014fC0eb4D435f1"
	defaultMaxWeiPerTx = "5000000000000000" // 0.005 ETH
)

// Load reads APP_ENV (dev|production, default dev) plus optional
// TREASURY_ADDR, MAX_WEI_PER_TX, and RPC_URL overrides.
func Load() (Config, error) {
	env := envOr("APP_ENV", "dev")
	net, ok := networks[env]
	if !ok {
		return Config{}, fmt.Errorf("unknown APP_ENV %q (want dev or production)", env)
	}
	if rpc := os.Getenv("RPC_URL"); rpc != "" {
		net.RPCURL = rpc
	}

	cfg := Config{
		Env:          env,
		Network:      net,
		TreasuryAddr: os.Getenv("TREASURY_ADDR"),
		MaxWeiPerTx:  envOr("MAX_WEI_PER_TX", defaultMaxWeiPerTx),
		StateFile:    "wallets." + env + ".json",
	}
	if cfg.TreasuryAddr == "" {
		if env == "production" {
			return Config{}, fmt.Errorf("TREASURY_ADDR is required in production; refusing to default a mainnet recipient")
		}
		cfg.TreasuryAddr = devTreasuryAddr
	}
	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
