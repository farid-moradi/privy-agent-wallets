package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	privy "github.com/privy-io/go-sdk"
)

// The one address the agent is allowed to pay, and the per-transaction cap.
// On a real deployment these come from your config, not constants.
const (
	treasuryAddr = "0x1B9aB6d9E9dBcD98BA47649aB014fC0eb4D435f1" // demo recipient
	maxWeiPerTx  = "5000000000000000"                           // 0.005 ETH
)

// state is written by `setup` and read by `run`.
type state struct {
	EVMWalletID   string `json:"evm_wallet_id"`
	EVMAddress    string `json:"evm_address"`
	SolWalletID   string `json:"sol_wallet_id"`
	SolAddress    string `json:"sol_address"`
	EVMPolicyID   string `json:"evm_policy_id"`
	TreasuryAddr  string `json:"treasury_addr"`
}

const stateFile = "wallets.json"

func setup(ctx context.Context, client *privy.PrivyClient) error {
	// 1. A policy: allow small payments to the treasury, deny everything else.
	// Privy policies are default-deny — a method with no matching ALLOW rule
	// is rejected inside the TEE before anything is signed.
	policy, err := client.Policies.New(ctx, privy.PolicyNewParams{
		Version:   privy.PolicyNewParamsVersion1_0,
		ChainType: privy.WalletChainTypeEthereum,
		Name:      "petty-cash guardrails",
		Rules: []privy.PolicyNewParamsRule{
			{
				Name:   "small payments to the treasury only",
				Method: privy.PolicyMethodEthSendTransaction,
				Action: privy.PolicyActionAllow,
				Conditions: []privy.PolicyConditionUnion{
					{OfEthereumTransaction: &privy.EthereumTransactionCondition{
						FieldSource: "ethereum_transaction",
						Field:       "to",
						Operator:    "eq",
						Value:       privy.ConditionValueUnion{OfString: privy.String(treasuryAddr)},
					}},
					{OfEthereumTransaction: &privy.EthereumTransactionCondition{
						FieldSource: "ethereum_transaction",
						Field:       "value",
						Operator:    "lte",
						Value:       privy.ConditionValueUnion{OfString: privy.String(maxWeiPerTx)},
					}},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("create policy: %w", err)
	}
	fmt.Printf("policy   %s  (id %s)\n", policy.Name, policy.ID)

	// 2. An EVM wallet with the policy attached at birth.
	evm, err := client.Wallets.New(ctx, privy.WalletNewParams{
		ChainType: privy.WalletChainTypeEthereum,
		PolicyIDs: privy.PolicyInput{policy.ID},
	})
	if err != nil {
		return fmt.Errorf("create evm wallet: %w", err)
	}
	fmt.Println(fmtWallet(evm))

	// 3. A Solana wallet — same API, different chain type.
	sol, err := client.Wallets.New(ctx, privy.WalletNewParams{
		ChainType: privy.WalletChainTypeSolana,
	})
	if err != nil {
		return fmt.Errorf("create solana wallet: %w", err)
	}
	fmt.Println(fmtWallet(sol))

	s := state{
		EVMWalletID:  evm.ID,
		EVMAddress:   evm.Address,
		SolWalletID:  sol.ID,
		SolAddress:   sol.Address,
		EVMPolicyID:  policy.ID,
		TreasuryAddr: treasuryAddr,
	}
	data, _ := json.MarshalIndent(s, "", "  ")
	if err := os.WriteFile(stateFile, data, 0o600); err != nil {
		return err
	}
	fmt.Printf("\nwrote %s — fund the EVM wallet with Base Sepolia ETH and run the agent\n", stateFile)
	return nil
}

func loadState() (state, error) {
	var s state
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return s, fmt.Errorf("read %s (did you run setup?): %w", stateFile, err)
	}
	return s, json.Unmarshal(data, &s)
}
