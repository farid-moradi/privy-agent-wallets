package wallet

import (
	"encoding/json"
	"fmt"
	"os"
)

// State is written by setup and read on every run.
type State struct {
	EVMWalletID  string `json:"evm_wallet_id"`
	EVMAddress   string `json:"evm_address"`
	SolWalletID  string `json:"sol_wallet_id"`
	SolAddress   string `json:"sol_address"`
	EVMPolicyID  string `json:"evm_policy_id"`
	TreasuryAddr string `json:"treasury_addr"`
}

func (s State) Save(path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func LoadState(path string) (State, error) {
	var s State
	data, err := os.ReadFile(path)
	if err != nil {
		return s, fmt.Errorf("read %s (did you run setup?): %w", path, err)
	}
	return s, json.Unmarshal(data, &s)
}
