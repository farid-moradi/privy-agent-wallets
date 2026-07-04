package wallet

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
)

// GetBalance does a raw eth_getBalance against the given RPC endpoint.
func GetBalance(ctx context.Context, rpcURL, addr string) (string, error) {
	body := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"eth_getBalance","params":["%s","latest"]}`, addr)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, strings.NewReader(body))
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
