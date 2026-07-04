// Package agent runs a Claude tool-use loop over a policy-guarded wallet.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	anthropic "github.com/anthropics/anthropic-sdk-go"

	"github.com/farid-moradi/privy-agent-wallets/pkg/wallet"
)

const systemPrompt = `You are an accounts-payable agent for a small engineering team.
You manage a petty-cash wallet on Base Sepolia (a testnet).
You can check the ETH price, check the wallet balance, and send payments.
Company policy: payments go to the team treasury only, and only small amounts.
Before paying, sanity-check the amount against the balance. Report exactly what
happened, including failures — never claim a payment succeeded without a tx hash.`

// Run drives the agent loop until Claude stops asking for tools.
func Run(ctx context.Context, w *wallet.Client, s wallet.State, task string) error {
	llm := anthropic.NewClient() // reads ANTHROPIC_API_KEY

	tools := []anthropic.ToolUnionParam{
		{OfTool: &anthropic.ToolParam{
			Name:        "get_eth_price",
			Description: anthropic.String("Current ETH price in USD from CoinGecko."),
			InputSchema: anthropic.ToolInputSchemaParam{Properties: map[string]any{}},
		}},
		{OfTool: &anthropic.ToolParam{
			Name:        "get_balance",
			Description: anthropic.String("Current balance of the petty-cash wallet in ETH."),
			InputSchema: anthropic.ToolInputSchemaParam{Properties: map[string]any{}},
		}},
		{OfTool: &anthropic.ToolParam{
			Name: "send_eth",
			Description: anthropic.String(
				"Send ETH from the petty-cash wallet. Call this only after deciding a payment is due. " +
					"Returns the transaction hash on success."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]any{
					"to":         map[string]any{"type": "string", "description": "Recipient 0x address"},
					"amount_eth": map[string]any{"type": "string", "description": "Amount in ETH, e.g. \"0.001\""},
				},
				Required: []string{"to", "amount_eth"},
			},
		}},
	}

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(
			fmt.Sprintf("Wallet address: %s\nTreasury address: %s\n\nTask: %s",
				s.EVMAddress, s.TreasuryAddr, task))),
	}

	for {
		resp, err := llm.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     anthropic.ModelClaudeOpus4_8,
			MaxTokens: 2048,
			System:    []anthropic.TextBlockParam{{Text: systemPrompt}},
			Messages:  messages,
			Tools:     tools,
		})
		if err != nil {
			return fmt.Errorf("anthropic: %w", err)
		}
		messages = append(messages, resp.ToParam())

		var results []anthropic.ContentBlockParamUnion
		for _, block := range resp.Content {
			switch b := block.AsAny().(type) {
			case anthropic.TextBlock:
				fmt.Printf("\nagent> %s\n", b.Text)
			case anthropic.ToolUseBlock:
				out, toolErr := dispatch(ctx, w, s, b.Name, []byte(b.JSON.Input.Raw()))
				if toolErr != nil {
					// Surface the failure to the model instead of crashing — a
					// policy DENY from Privy lands here, and the agent should
					// read it and report back.
					log.Printf("tool %s error: %v", b.Name, toolErr)
					results = append(results, anthropic.NewToolResultBlock(b.ID, toolErr.Error(), true))
				} else {
					log.Printf("tool %s -> %s", b.Name, out)
					results = append(results, anthropic.NewToolResultBlock(b.ID, out, false))
				}
			}
		}

		if resp.StopReason != anthropic.StopReasonToolUse {
			return nil
		}
		messages = append(messages, anthropic.NewUserMessage(results...))
	}
}

func dispatch(ctx context.Context, w *wallet.Client, s wallet.State, name string, input []byte) (string, error) {
	switch name {
	case "get_eth_price":
		return getETHPrice(ctx)
	case "get_balance":
		return wallet.GetBalance(ctx, s.EVMAddress)
	case "send_eth":
		var in struct {
			To        string `json:"to"`
			AmountETH string `json:"amount_eth"`
		}
		if err := json.Unmarshal(input, &in); err != nil {
			return "", err
		}
		return w.SendETH(ctx, s.EVMWalletID, wallet.BaseSepoliaCAIP2, in.To, in.AmountETH)
	}
	return "", fmt.Errorf("unknown tool %q", name)
}
