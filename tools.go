package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"

	privy "github.com/privy-io/go-sdk"
)

const (
	baseSepoliaCAIP2 = "eip155:84532"
	baseSepoliaRPC   = "https://sepolia.base.org"
)

// getETHPrice asks CoinGecko's free API for the spot price.
func getETHPrice(ctx context.Context) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.coingecko.com/api/v3/simple/price?ids=ethereum&vs_currencies=usd", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		Ethereum struct {
			USD float64 `json:"usd"`
		} `json:"ethereum"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return fmt.Sprintf("ETH is $%.2f", out.Ethereum.USD), nil
}

// getBalance does a raw eth_getBalance against the public Base Sepolia RPC.
func getBalance(ctx context.Context, addr string) (string, error) {
	body := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"eth_getBalance","params":["%s","latest"]}`, addr)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, baseSepoliaRPC, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		Result string `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.Error != nil {
		return "", fmt.Errorf("rpc: %s", out.Error.Message)
	}
	wei, ok := new(big.Int).SetString(strings.TrimPrefix(out.Result, "0x"), 16)
	if !ok {
		return "", fmt.Errorf("bad balance %q", out.Result)
	}
	return fmt.Sprintf("balance: %s ETH", weiToETH(wei)), nil
}

// sendETH asks Privy to sign and broadcast a transfer from the agent's wallet.
// The policy attached to the wallet is evaluated inside Privy's TEE before
// signing — a violating transaction comes back as an error, not a signature.
func sendETH(ctx context.Context, client *privy.PrivyClient, walletID, to, amountETH string) (string, error) {
	wei, err := ethToWei(amountETH)
	if err != nil {
		return "", err
	}
	resp, err := client.Wallets.Ethereum.SendTransaction(ctx, walletID, baseSepoliaCAIP2,
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
