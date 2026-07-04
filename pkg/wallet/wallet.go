// Package wallet wraps Privy server wallets: policy-guarded wallet creation
// and transaction signing for EVM and Solana.
package wallet

import (
	"context"
	"fmt"
	"math/big"

	privy "github.com/privy-io/go-sdk"
)

const (
	// BaseSepoliaCAIP2 selects the chain per transaction; the same EVM wallet
	// works on any EVM chain.
	BaseSepoliaCAIP2 = "eip155:84532"
	baseSepoliaRPC   = "https://sepolia.base.org"
)

// Config bounds what the wallet is allowed to do. The policy built from it is
// enforced inside Privy's TEE, next to the key.
type Config struct {
	// TreasuryAddr is the only address the wallet may pay.
	TreasuryAddr string
	// MaxWeiPerTx caps a single transaction, as a decimal wei string.
	MaxWeiPerTx string
}

// Client is a thin wrapper around the Privy client.
type Client struct {
	privy *privy.PrivyClient
}

func New(appID, appSecret string) *Client {
	return &Client{privy: privy.NewPrivyClient(privy.PrivyClientOptions{
		AppID:     appID,
		AppSecret: appSecret,
	})}
}

// Setup creates the spending policy, an EVM wallet carrying it, and a Solana
// wallet. Privy policies are default-deny: a method with no matching ALLOW
// rule is rejected before anything is signed.
func (c *Client) Setup(ctx context.Context, cfg Config) (State, error) {
	var s State

	policy, err := c.privy.Policies.New(ctx, privy.PolicyNewParams{
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
						Value:       privy.ConditionValueUnion{OfString: privy.String(cfg.TreasuryAddr)},
					}},
					{OfEthereumTransaction: &privy.EthereumTransactionCondition{
						FieldSource: "ethereum_transaction",
						Field:       "value",
						Operator:    "lte",
						Value:       privy.ConditionValueUnion{OfString: privy.String(cfg.MaxWeiPerTx)},
					}},
				},
			},
		},
	})
	if err != nil {
		return s, fmt.Errorf("create policy: %w", err)
	}

	evm, err := c.privy.Wallets.New(ctx, privy.WalletNewParams{
		ChainType: privy.WalletChainTypeEthereum,
		PolicyIDs: privy.PolicyInput{policy.ID},
	})
	if err != nil {
		return s, fmt.Errorf("create evm wallet: %w", err)
	}

	sol, err := c.privy.Wallets.New(ctx, privy.WalletNewParams{
		ChainType: privy.WalletChainTypeSolana,
	})
	if err != nil {
		return s, fmt.Errorf("create solana wallet: %w", err)
	}

	return State{
		EVMWalletID:  evm.ID,
		EVMAddress:   evm.Address,
		SolWalletID:  sol.ID,
		SolAddress:   sol.Address,
		EVMPolicyID:  policy.ID,
		TreasuryAddr: cfg.TreasuryAddr,
	}, nil
}

// SendETH asks Privy to sign and broadcast a transfer from the wallet. The
// attached policy is evaluated inside the TEE before signing; a violating
// transaction comes back as an error, not a signature.
func (c *Client) SendETH(ctx context.Context, walletID, caip2, to, amountETH string) (string, error) {
	wei, err := ethToWei(amountETH)
	if err != nil {
		return "", err
	}
	resp, err := c.privy.Wallets.Ethereum.SendTransaction(ctx, walletID, caip2,
		privy.EthereumSendTransactionRpcInputParams{
			Transaction: privy.UnsignedEthereumTransactionUnion{
				OfUnsignedStandardEthereumTransaction: &privy.UnsignedStandardEthereumTransaction{
					To:    privy.String(to),
					Value: privy.QuantityUnion{OfString: privy.String("0x" + wei.Text(16))},
				},
			},
		})
	if err != nil {
		return "", fmt.Errorf("privy rejected the transaction: %w", err)
	}
	return fmt.Sprintf("sent %s ETH to %s, tx hash %s", amountETH, to, resp.Hash), nil
}

func ethToWei(amount string) (*big.Int, error) {
	f, ok := new(big.Float).SetString(amount)
	if !ok {
		return nil, fmt.Errorf("bad amount %q", amount)
	}
	wei, _ := new(big.Float).Mul(f, big.NewFloat(1e18)).Int(nil)
	if wei.Sign() <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}
	return wei, nil
}

func weiToETH(wei *big.Int) string {
	f := new(big.Float).Quo(new(big.Float).SetInt(wei), big.NewFloat(1e18))
	return f.Text('f', 6)
}
