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
	"github.com/xiaomi388/beancount-automation/pkg/config"
	"github.com/xiaomi388/beancount-automation/pkg/plaidclient"
)

func createLinkToken(ctx context.Context, c *plaid.APIClient) (string, error) {
	user := plaid.LinkTokenCreateRequestUser{
		ClientUserId: "USERID",
	}
	request := plaid.NewLinkTokenCreateRequest(
		"Beancount Automation",
		"en",
		[]plaid.CountryCode{plaid.COUNTRYCODE_US},
		user,
	)
	request.SetProducts([]plaid.Products{plaid.PRODUCTS_TRANSACTIONS})
	resp, _, err := c.PlaidApi.LinkTokenCreate(ctx).LinkTokenCreateRequest(*request).Execute()
	if err != nil {
		return "", err
	}

	linkToken := resp.GetLinkToken()
	return linkToken, nil
}

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

func readAccessToken() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter access token: ")
	text, err := reader.ReadString('\n')
	text = strings.ReplaceAll(text, "\n", "")
	if err != nil {
		return "", err
	}

	return text, nil
}

func Link(owner string, institution string) error {
	cfg, err := config.Load(config.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load config file: %w", err)
	}

	if _, ok := cfg.Institution(institution, owner); ok {
		return fmt.Errorf("%s:%s already existed", owner, institution)
	}

	ctx := context.Background()

	c := plaidclient.New(cfg.ClientID, cfg.Secret, cfg.Environment)
	linkToken, err := createLinkToken(ctx, c)
	if err != nil {
		return fmt.Errorf("failed to create link token: %w", err)

	}

	if err := generateAuthPage(linkToken); err != nil {
		return fmt.Errorf("failed to generate auth page: %w", err)
	}

	accessToken, err := readAccessToken()
	fmt.Println(accessToken)
	if err != nil {
		return fmt.Errorf("failed to read access token: %w", err)
	}

	cfg.SetInstitution(config.Institution{
		Name:        institution,
		AccessToken: accessToken,
	}, owner)

	config.Dump(config.ConfigPath, cfg)
	return nil
}
