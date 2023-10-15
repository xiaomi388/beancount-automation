package link

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/template"

	"github.com/plaid/plaid-go/plaid"
	"github.com/sirupsen/logrus"
)

func generateAuthPage(linkToken string) error {
	tmpl, err := template.New("link.yaml").Parse(linkHTML)
	if err != nil {
		return fmt.Errorf("failed to generate template: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "*.html")
	if err != nil {
		return fmt.Errorf("failed to create tmp file: %w", err)

	}

	data := struct{ LinkToken string }{linkToken}
	if err := tmpl.Execute(tmpFile, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	if err := exec.Command("open", tmpFile.Name()).Run(); err != nil {
		return fmt.Errorf("failed to open generated auth page: %w", err)
	}

	return nil
}

func exchangeAccessToken(ctx context.Context, c *plaid.APIClient, publicToken string) (string, error) {
	exchangePublicTokenReq := plaid.NewItemPublicTokenExchangeRequest(publicToken)
	exchangePublicTokenResp, _, err := c.PlaidApi.ItemPublicTokenExchange(ctx).ItemPublicTokenExchangeRequest(*exchangePublicTokenReq).Execute()

	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}

	return exchangePublicTokenResp.GetAccessToken(), nil
}

func readPublicToken() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter publc token: ")
	text, err := reader.ReadString('\n')
	text = strings.ReplaceAll(text, "\n", "")
	if err != nil {
		return "", err
	}

	return text, nil
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
