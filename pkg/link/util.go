package link

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/plaid/plaid-go/plaid"
	"github.com/sirupsen/logrus"
)

func displayLinkPage(linkURL string) error {
	cmd := exec.Command("open", linkURL)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to open link page: %w", err)
	}
	return nil
}

func waitForToken(ctx context.Context, ts *tokenServer) (string, error) {
	token, err := ts.waitForToken(ctx)
	if err == nil && token != "" {
		return token, nil
	}

	fmt.Print("Enter public token: ")
	reader := bufio.NewReader(os.Stdin)
	text, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(text), nil
}

func exchangeAccessToken(ctx context.Context, c *plaid.APIClient, publicToken string) (string, error) {
	exchangePublicTokenReq := plaid.NewItemPublicTokenExchangeRequest(publicToken)
	exchangePublicTokenResp, _, err := c.PlaidApi.ItemPublicTokenExchange(ctx).ItemPublicTokenExchangeRequest(*exchangePublicTokenReq).Execute()

	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}

	return exchangePublicTokenResp.GetAccessToken(), nil
}

func createLinkToken(ctx context.Context, c *plaid.APIClient, pd *plaid.Products, accessToken *string) (string, error) {
	user := plaid.LinkTokenCreateRequestUser{
		ClientUserId: "USERID",
	}
	request := plaid.NewLinkTokenCreateRequest(
		"Beancount Automation",
		"en",
		[]plaid.CountryCode{plaid.COUNTRYCODE_US},
		user,
	)

	if pd != nil {
		request.SetProducts([]plaid.Products{*pd})
	}
	if accessToken != nil {
		request.SetAccessToken(*accessToken)
	}

	resp, httpResp, err := c.PlaidApi.LinkTokenCreate(ctx).LinkTokenCreateRequest(*request).Execute()
	if err != nil {
		logrus.Debug(httpResp.Body)
		return "", err
	}

	linkToken := resp.GetLinkToken()
	return linkToken, nil
}

func launchLinkFlow(ctx context.Context, linkToken string) (string, error) {
	ts := newTokenServer(linkToken)
	linkURL, err := ts.start(ctx)
	if err != nil {
		return "", err
	}

	if err := displayLinkPage(linkURL); err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	token, err := waitForToken(ctx, ts)
	ts.shutdown(context.Background())
	if err != nil {
		return "", err
	}

	if token == "" {
		return "", errors.New("empty public token")
	}

	return token, nil
}
