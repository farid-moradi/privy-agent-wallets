package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
