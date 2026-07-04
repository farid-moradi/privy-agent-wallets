// A minimal example of the OTHER Privy custody model: a user-owned wallet.
//
// It creates a Privy user from an email address and a wallet whose owner is
// that user. Then it proves the point of ownership: it asks the wallet to
// sign using only the app secret, which fails, because signing requests for
// an owned wallet must be authorized by the owner (in a real product, via the
// user's JWT from your auth flow).
//
// usage: user-wallet-demo alice@example.com
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	privy "github.com/privy-io/go-sdk"
)

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 {
		log.Fatalf("usage: user-wallet-demo <email>")
	}
	email := os.Args[1]

	client := privy.NewPrivyClient(privy.PrivyClientOptions{
		AppID:     os.Getenv("PRIVY_APP_ID"),
		AppSecret: os.Getenv("PRIVY_APP_SECRET"),
	})
	ctx := context.Background()

	// 1. A user, identified by email. In a real product this happens when
	// they sign up; their JWT later authorizes wallet actions.
	user, err := client.Users.New(ctx, privy.UserNewParams{
		LinkedAccounts: []privy.LinkedAccountInputUnion{
			{OfEmail: &privy.LinkedAccountEmailInput{Address: email}},
		},
	})
	if err != nil {
		log.Fatalf("create user: %v", err)
	}
	fmt.Printf("user   %s  (%s)\n", user.ID, email)

	// 2. A wallet owned by that user. The owner is the difference between
	// custody models: with it, the app secret alone cannot sign.
	w, err := client.Wallets.New(ctx, privy.WalletNewParams{
		ChainType: privy.WalletChainTypeEthereum,
		Owner: privy.OwnerInputUnion{
			OfOwnerInputUser: &privy.OwnerInputUser{UserID: user.ID},
		},
	})
	if err != nil {
		log.Fatalf("create wallet: %v", err)
	}
	fmt.Printf("wallet %s  (id %s, owner %s)\n", w.Address, w.ID, w.OwnerID)

	// 3. The proof: the app tries to sign with the user's wallet on its own.
	_, err = client.Wallets.Ethereum.SignMessage(ctx, w.ID, "the app trying to act without its user")
	if err != nil {
		fmt.Printf("\napp-secret-only signing attempt: %v\n", err)
		fmt.Println("(this failure is the feature: only the owner can authorize signing)")
		return
	}
	log.Fatal("unexpected: the app was able to sign with a user-owned wallet")
}
