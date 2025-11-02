package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	baseURL          = "https://moneybird.com/api/v2"
	administrationID = "341884047915484822"
)

// LedgerAccount represents a Moneybird ledger account
type LedgerAccount struct {
	ID                   string    `json:"id"`
	AdministrationID     string    `json:"administration_id"`
	Name                 string    `json:"name"`
	AccountType          string    `json:"account_type"`
	AccountID            *string   `json:"account_id"`
	ParentID             *string   `json:"parent_id"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
	AllowedDocumentTypes []string  `json:"allowed_document_types"`
	TaxonomyItem         *struct {
		TaxonomyVersion string `json:"taxonomy_version"`
		Code            string `json:"code"`
		Name            string `json:"name"`
		NameEnglish     string `json:"name_english"`
		Reference       string `json:"reference"`
	} `json:"taxonomy_item"`
	FinancialAccountID *string `json:"financial_account_id"`
}

// Client is the Moneybird API client
type Client struct {
	apiToken string
	client   *http.Client
}

// NewClient creates a new Moneybird API client
func NewClient(apiToken string) *Client {
	return &Client{
		apiToken: apiToken,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

// doRequest performs an authenticated API request
func (c *Client) doRequest(method, endpoint string) ([]byte, error) {
	url := fmt.Sprintf("%s/%s/%s", baseURL, administrationID, endpoint)

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// GetLedgerAccounts fetches all ledger accounts
func (c *Client) GetLedgerAccounts() ([]LedgerAccount, error) {
	body, err := c.doRequest("GET", "ledger_accounts.json")
	if err != nil {
		return nil, err
	}

	var accounts []LedgerAccount
	if err := json.Unmarshal(body, &accounts); err != nil {
		return nil, fmt.Errorf("unmarshaling response: %w", err)
	}

	return accounts, nil
}

// loadEnvFile loads environment variables from a file (for local development)
func loadEnvFile(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		// .env file doesn't exist, that's fine for production
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			os.Setenv(key, value)
		}
	}
}

func main() {
	// Load .env file if it exists (for local testing)
	loadEnvFile(".env")

	// Get API token from environment variable
	apiToken := os.Getenv("MONEYBIRD_API_TOKEN")
	if apiToken == "" {
		fmt.Println("Error: MONEYBIRD_API_TOKEN environment variable not set")
		fmt.Println("Usage: export MONEYBIRD_API_TOKEN='your-token-here'")
		os.Exit(1)
	}

	client := NewClient(apiToken)

	fmt.Println("Fetching ledger accounts from Moneybird...")
	accounts, err := client.GetLedgerAccounts()
	if err != nil {
		fmt.Printf("Error fetching accounts: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nFound %d ledger accounts\n\n", len(accounts))

	// Group accounts by type
	accountsByType := make(map[string][]LedgerAccount)
	for _, acc := range accounts {
		accountsByType[acc.AccountType] = append(accountsByType[acc.AccountType], acc)
	}

	// Print summary
	fmt.Println("=== Account Types Summary ===")
	for accountType, accs := range accountsByType {
		fmt.Printf("\n%s (%d accounts):\n", accountType, len(accs))
		for _, acc := range accs {
			parent := "root"
			if acc.ParentID != nil {
				parent = *acc.ParentID
			}
			fmt.Printf("  - %s (ID: %s, Parent: %s)\n", acc.Name, acc.ID, parent)
		}
	}

	// Save detailed JSON for inspection
	detailedJSON, _ := json.MarshalIndent(accounts, "", "  ")
	if err := os.WriteFile("ledger_accounts.json", detailedJSON, 0644); err != nil {
		fmt.Printf("Warning: Could not save detailed JSON: %v\n", err)
	} else {
		fmt.Printf("\n✓ Detailed account data saved to ledger_accounts.json\n")
	}
}
