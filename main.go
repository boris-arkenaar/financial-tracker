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

// Payment represents a payment linked to an invoice
type Payment struct {
	ID                  string    `json:"id"`
	AdministrationID    string    `json:"administration_id"`
	InvoiceType         string    `json:"invoice_type"`
	InvoiceID           string    `json:"invoice_id"`
	FinancialAccountID  string    `json:"financial_account_id"`
	UserID              string    `json:"user_id"`
	Price               string    `json:"price"`
	PriceBase           string    `json:"price_base"`
	PaymentDate         string    `json:"payment_date"`
	FinancialMutationID string    `json:"financial_mutation_id"`
	LedgerAccountID     string    `json:"ledger_account_id"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// LedgerAccountBooking represents a booking entry within a financial mutation
type LedgerAccountBooking struct {
	ID                  string    `json:"id"`
	AdministrationID    string    `json:"administration_id"`
	FinancialMutationID string    `json:"financial_mutation_id"`
	LedgerAccountID     string    `json:"ledger_account_id"`
	ProjectID           *string   `json:"project_id"`
	Description         string    `json:"description"`
	Price               string    `json:"price"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// FinancialMutation represents a Moneybird financial mutation (transaction)
type FinancialMutation struct {
	ID                    string                 `json:"id"`
	AdministrationID      string                 `json:"administration_id"`
	Amount                string                 `json:"amount"`
	Code                  string                 `json:"code"`
	Date                  string                 `json:"date"`
	Message               string                 `json:"message"`
	ContraAccountName     string                 `json:"contra_account_name"`
	ContraAccountNumber   string                 `json:"contra_account_number"`
	State                 string                 `json:"state"`
	LedgerAccountID       string                 `json:"ledger_account_id"`
	FinancialAccountID    string                 `json:"financial_account_id"`
	Payments              []Payment              `json:"payments"`
	LedgerAccountBookings []LedgerAccountBooking `json:"ledger_account_bookings"`
	CreatedAt             time.Time              `json:"created_at"`
	UpdatedAt             time.Time              `json:"updated_at"`
}

// DocumentDetail represents a line item in a document
type DocumentDetail struct {
	ID              string `json:"id"`
	LedgerAccountID string `json:"ledger_account_id"`
	Price           string `json:"price"`
}

// Document represents a Moneybird document (receipt/invoice)
type Document struct {
	ID      string           `json:"id"`
	Details []DocumentDetail `json:"details"`
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

// GetFinancialMutations fetches financial mutations for a specific period
func (c *Client) GetFinancialMutations(startDate, endDate string) ([]FinancialMutation, error) {
	endpoint := fmt.Sprintf("financial_mutations.json?filter=period:%s..%s", startDate, endDate)
	body, err := c.doRequest("GET", endpoint)
	if err != nil {
		return nil, err
	}

	var mutations []FinancialMutation
	if err := json.Unmarshal(body, &mutations); err != nil {
		return nil, fmt.Errorf("unmarshaling response: %w", err)
	}

	return mutations, nil
}

// GetDocumentsBatch fetches multiple documents at once using the synchronization endpoint
func (c *Client) GetDocumentsBatch(documentIDs []string, docType string) ([]Document, error) {
	if len(documentIDs) == 0 {
		return nil, nil
	}

	endpoint := fmt.Sprintf("documents/%s/synchronization.json", docType)

	requestBody := map[string]interface{}{
		"ids": documentIDs,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	url := fmt.Sprintf("%s/%s/%s", baseURL, administrationID, endpoint)
	req, err := http.NewRequest("POST", url, strings.NewReader(string(jsonData)))
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

	var docs []Document
	if err := json.Unmarshal(body, &docs); err != nil {
		return nil, fmt.Errorf("unmarshaling response: %w", err)
	}

	return docs, nil
}

// loadEnvFile loads environment variables from a file (for local development)
func loadEnvFile(filename string) {
	file, err := os.Open(filename)
	if err != nil {
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
	loadEnvFile(".env")

	apiToken := os.Getenv("MONEYBIRD_API_TOKEN")
	if apiToken == "" {
		fmt.Println("Error: MONEYBIRD_API_TOKEN environment variable not set")
		fmt.Println("Usage: export MONEYBIRD_API_TOKEN='your-token-here'")
		os.Exit(1)
	}

	client := NewClient(apiToken)

	// Get previous month's date range
	now := time.Now()
	firstOfThisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	monthStart := firstOfThisMonth.AddDate(0, -1, 0)
	monthEnd := firstOfThisMonth.AddDate(0, 0, -1)

	fmt.Printf("Fetching financial data for %s...\n\n", monthStart.Format("January 2006"))

	// Fetch ledger accounts
	fmt.Println("1. Fetching ledger accounts...")
	accounts, err := client.GetLedgerAccounts()
	if err != nil {
		fmt.Printf("Error fetching accounts: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("   Found %d ledger accounts\n", len(accounts))

	// Create account lookup map
	accountMap := make(map[string]LedgerAccount)
	for _, acc := range accounts {
		accountMap[acc.ID] = acc
	}

	// Fetch financial mutations in 7-day chunks
	fmt.Printf("\n2. Fetching transactions in chunks...\n")
	var allMutations []FinancialMutation

	currentStart := monthStart
	chunkNum := 1
	for currentStart.Before(monthEnd) || currentStart.Equal(monthEnd) {
		chunkEnd := currentStart.AddDate(0, 0, 6)
		if chunkEnd.After(monthEnd) {
			chunkEnd = monthEnd
		}

		fmt.Printf("   Chunk %d: %s to %s...",
			chunkNum,
			currentStart.Format("2006-01-02"),
			chunkEnd.Format("2006-01-02"))

		mutations, err := client.GetFinancialMutations(
			currentStart.Format("2006-01-02"),
			chunkEnd.Format("2006-01-02"),
		)
		if err != nil {
			fmt.Printf("\n   Error fetching chunk: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf(" %d transactions\n", len(mutations))
		allMutations = append(allMutations, mutations...)

		currentStart = chunkEnd.AddDate(0, 0, 1)
		chunkNum++
	}

	fmt.Printf("   Total: %d transactions\n", len(allMutations))

	// Aggregate by ledger account
	fmt.Println("\n3. Aggregating transactions by category...")
	totals := make(map[string]float64)

	// Find the Omzet (revenue) account ID
	var omzetAccountID string
	for _, acc := range accounts {
		if acc.Name == "Omzet" && acc.AccountType == "revenue" {
			omzetAccountID = acc.ID
			break
		}
	}

	// First pass: collect all unique document IDs
	fmt.Println("   Collecting document IDs...")
	uniqueDocIDs := make(map[string]bool)
	for _, mut := range allMutations {
		for _, payment := range mut.Payments {
			if payment.InvoiceType == "Document" {
				uniqueDocIDs[payment.InvoiceID] = true
			}
		}
	}

	// Fetch documents in batches
	documentCache := make(map[string][]DocumentDetail)
	if len(uniqueDocIDs) > 0 {
		docIDs := make([]string, 0, len(uniqueDocIDs))
		for id := range uniqueDocIDs {
			docIDs = append(docIDs, id)
		}

		fmt.Printf("   Fetching %d unique documents...\n", len(docIDs))

		// Try purchase_invoices first
		purchaseDocs, err := client.GetDocumentsBatch(docIDs, "purchase_invoices")
		if err == nil {
			fmt.Printf("   Found %d purchase invoices\n", len(purchaseDocs))
			for _, doc := range purchaseDocs {
				if len(doc.Details) > 0 {
					documentCache[doc.ID] = doc.Details
				}
			}
		}

		// Try receipts for any remaining
		receiptDocs, err := client.GetDocumentsBatch(docIDs, "receipts")
		if err == nil {
			fmt.Printf("   Found %d receipts\n", len(receiptDocs))
			for _, doc := range receiptDocs {
				if len(doc.Details) > 0 {
					documentCache[doc.ID] = doc.Details
				}
			}
		}

		fmt.Printf("   Successfully mapped %d/%d documents\n", len(documentCache), len(docIDs))
	}

	var paymentsProcessed, bookingsProcessed int

	for _, mut := range allMutations {
		// Process ledger account bookings (direct categorizations)
		for _, booking := range mut.LedgerAccountBookings {
			var amount float64
			fmt.Sscanf(booking.Price, "%f", &amount)
			totals[booking.LedgerAccountID] += amount
			bookingsProcessed++
		}

		// Process payments (linked to documents/invoices)
		for _, payment := range mut.Payments {
			var amount float64
			fmt.Sscanf(payment.Price, "%f", &amount)

			if payment.InvoiceType == "SalesInvoice" {
				// Sales invoices are revenue
				totals[omzetAccountID] += amount
				paymentsProcessed++
			} else if payment.InvoiceType == "Document" {
				// Look up document details in our cache
				if details, ok := documentCache[payment.InvoiceID]; ok {
					// Add each detail to its respective ledger account
					for _, detail := range details {
						if detail.LedgerAccountID != "" {
							var detailAmount float64
							fmt.Sscanf(detail.Price, "%f", &detailAmount)
							// Payment prices are not negative like booking prices
							// (-= instead of +=)
							totals[detail.LedgerAccountID] -= detailAmount
						}
					}
					paymentsProcessed++
				}
				// Skip if document not found
			} else if payment.LedgerAccountID != "" {
				// Other payment types use their ledger account
				totals[payment.LedgerAccountID] += amount
				paymentsProcessed++
			}
		}
	}

	fmt.Printf("   Processed %d bookings and %d payments\n", bookingsProcessed, paymentsProcessed)
	fmt.Printf("   Aggregated into %d categories\n", len(totals))

	// Group by account type
	fmt.Println("\n=== Monthly Summary ===")
	typeGroups := make(map[string]map[string]float64)
	for ledgerID, total := range totals {
		if acc, ok := accountMap[ledgerID]; ok {
			if typeGroups[acc.AccountType] == nil {
				typeGroups[acc.AccountType] = make(map[string]float64)
			}
			typeGroups[acc.AccountType][acc.Name] = total
		}
	}

	// Print equity (family expenses)
	if equityAccounts, ok := typeGroups["equity"]; ok {
		fmt.Println("\nFamily Expenses (Equity accounts):")
		var totalEquity float64
		for name, amount := range equityAccounts {
			fmt.Printf("   %s: €%.2f\n", name, amount)
			totalEquity += amount
		}
		fmt.Printf("   TOTAL: €%.2f\n", totalEquity)
	}

	// Print revenue
	if revenueAccounts, ok := typeGroups["revenue"]; ok {
		fmt.Println("\nRevenue:")
		var totalRevenue float64
		for name, amount := range revenueAccounts {
			fmt.Printf("   %s: €%.2f\n", name, amount)
			totalRevenue += amount
		}
		fmt.Printf("   TOTAL: €%.2f\n", totalRevenue)
	}

	// Print business expenses
	if expenseAccounts, ok := typeGroups["expenses"]; ok {
		fmt.Println("\nBusiness Expenses:")
		var totalExpenses float64
		for name, amount := range expenseAccounts {
			fmt.Printf("   %s: €%.2f\n", name, amount)
			totalExpenses += amount
		}
		fmt.Printf("   TOTAL: €%.2f\n", totalExpenses)
	}

	// Save detailed data
	detailedData := map[string]interface{}{
		"period_start": monthStart.Format("2006-01-02"),
		"period_end":   monthEnd.Format("2006-01-02"),
		"mutations":    allMutations,
		"totals":       typeGroups,
	}

	detailedJSON, _ := json.MarshalIndent(detailedData, "", "  ")
	filename := fmt.Sprintf("financial_data_%s.json", monthStart.Format("2006-01"))
	if err := os.WriteFile(filename, detailedJSON, 0644); err != nil {
		fmt.Printf("\nWarning: Could not save detailed JSON: %v\n", err)
	} else {
		fmt.Printf("\nDetailed data saved to %s\n", filename)
	}
}
