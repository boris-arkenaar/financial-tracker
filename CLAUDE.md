# Financial Tracker - Project Handoff Document

## Project Overview

A Go application that fetches financial data from Moneybird, calculates family budget, generates visualizations, and sends daily reports via Telegram.

**Repository:** `boris-arkenaar/financial-tracker`  
**Current Status:** MVP deployed and running daily on Fly.io

---

## What's Currently Working

### Core Functionality

- ✅ Fetches transaction data from Moneybird API with pagination (7-day chunks)
- ✅ Handles two types of categorization:
  - **Ledger Account Bookings**: Direct categorizations (expenses are negative)
  - **Payments**: Linked to documents/invoices
    - `SalesInvoice` → mapped to "Omzet" (revenue)
    - `Document` → fetched via batch API to get ledger accounts from document details
- ✅ Aggregates all transactions by ledger account
- ✅ Groups by root categories (walks up parent hierarchy)
- ✅ Budget calculation: `Revenue excl. VAT - 30% Income Tax + Business Expenses`
- ✅ Pie chart visualization showing:
  - Family expense categories (sorted by amount, largest first)
  - Remaining budget (gray) or Over Budget (red)
- ✅ Daily Telegram report with budget summary and category breakdown
- ✅ Automated deployment on Fly.io via GitHub Actions (9 AM UTC daily)

### Key Features

- **Manual revenue override**: `-revenue=12850.20` flag for months where invoice paid in previous month
- **Proper VAT handling**: 21% Dutch VAT rate
- **Root category grouping**: Sums both direct amounts AND all subcategory amounts
- **Smart document fetching**: Batch API (purchase_invoices + receipts) to avoid rate limits

---

## Technical Architecture

### Tech Stack

- **Language**: Go 1.25
- **Visualization**: `github.com/wcharczuk/go-chart/v2`
- **APIs**: Moneybird API, Telegram Bot API
- **Hosting**: Fly.io (Docker deployment)
- **Scheduling**: GitHub Actions (`.github/workflows/daily-report.yml`)

### Project Structure

```
financial-tracker/
├── main.go                           # Main application
├── go.mod & go.sum                   # Dependencies
├── Dockerfile                        # Fly.io deployment
├── fly.toml                          # Fly.io configuration
├── .github/workflows/daily-report.yml # Scheduling (9 AM UTC daily)
├── .env                              # Local secrets (NOT committed)
└── .gitignore
```

### Environment Variables

Required secrets (set in Fly.io and locally in `.env`):

- `MONEYBIRD_API_TOKEN` - API access to Moneybird
- `TELEGRAM_BOT_TOKEN` - Bot token from BotFather
- `TELEGRAM_CHAT_ID` - Target chat for reports

### Key API Endpoints Used

**Moneybird:**

- `GET /ledger_accounts.json` - Fetch all accounts (once per run)
- `GET /financial_mutations.json?filter=period:YYYY-MM-DD..YYYY-MM-DD` - Fetch transactions
- `POST /documents/purchase_invoices/synchronization.json` - Batch fetch invoices
- `POST /documents/receipts/synchronization.json` - Batch fetch receipts

**Telegram:**

- `POST https://api.telegram.org/bot{token}/sendPhoto` - Send chart with caption

---

## Data Flow

### 1. Fetch Ledger Accounts

```
GET /ledger_accounts → Build accountMap[ID]LedgerAccount
```

### 2. Fetch Transactions (Paginated)

```
For each 7-day chunk in month:
  GET /financial_mutations → Append to allMutations[]
```

### 3. Collect Document IDs

```
For each payment where InvoiceType == "Document":
  Add payment.InvoiceID to uniqueDocIDs
```

### 4. Batch Fetch Documents

```
POST /documents/purchase_invoices/synchronization (all IDs)
POST /documents/receipts/synchronization (all IDs)
→ Build documentCache[ID][]DocumentDetail
```

### 5. Aggregate by Ledger Account

```
For each mutation:
  For each booking:
    totals[booking.LedgerAccountID] += booking.Price
  For each payment:
    If SalesInvoice: totals[omzetAccountID] += amount
    If Document:
      For each detail in documentCache:
        totals[detail.LedgerAccountID] -= detail.Price  // Note: subtract!
```

### 6. Calculate Budget

```
If manual revenue flag:
  totalRevenue = flag value

revenueExclVAT = totalRevenue / 1.21
incomeTax = revenueExclVAT * 0.30
familyBudget = revenueExclVAT - incomeTax + totalBusinessExpenses
```

### 7. Group by Root Categories

```
For each equity account:
  Walk up parent chain to find root
  rootTotals[root.Name] += total
Sort by amount (largest expense first)
```

### 8. Generate & Send

```
Create pie chart (PNG)
Format Telegram message (HTML)
Send via Telegram API
```

---

## Important Technical Details

### Transaction Categorization Logic

The tricky part is that Moneybird has two ways transactions get categorized:

1. **Ledger Account Bookings** (direct categorization in UI)

   - These are straightforward: use `booking.LedgerAccountID` and `booking.Price`
   - Prices are already negative for expenses

2. **Payments** (linked to invoices/documents)
   - **Problem**: The `payment.LedgerAccountID` points to the bank account, NOT the expense category
   - **Solution**: Use the `invoice_id` to fetch the actual document, then use `detail.LedgerAccountID` from the document's line items
   - **Important**: Document detail prices are positive, so we SUBTRACT them (`totals[id] -= amount`)

### Why Batch Document Fetching?

- Individual document fetches hit rate limits (429 errors)
- Batch synchronization endpoint handles all documents in 2 API calls
- Cache prevents duplicate fetches for documents referenced multiple times

### Root Category Calculation

Categories can have both direct expenses AND subcategory expenses:

```
Privé Woning: €-100 (direct)
├── Hypotheek: €-1127.74
└── VVE: €-335.04
TOTAL: €-1562.78 (sum of all three)
```

Implementation walks up parent chain and sums at root level.

---

## Current Deployment

### Fly.io Configuration

```toml
# fly.toml
app = "financial-tracker"
primary_region = "ams"  # Amsterdam

[build]
  # Uses Dockerfile

[env]
  # Secrets injected via flyctl secrets
```

### GitHub Actions Workflow

```yaml
# .github/workflows/daily-report.yml
schedule:
  - cron: "0 9 * * *" # 9 AM UTC daily
```

### Manual Operations

```bash
# Local testing
go run main.go

# With manual revenue
go run main.go -revenue=12850.20

# Deploy to Fly.io
flyctl deploy

# Set secrets
flyctl secrets set KEY=value

# Trigger GitHub Action manually
# (via Actions tab in GitHub)
```

---

## Known Limitations & Issues

### Current Limitations

1. **No change detection** - Sends report daily even if no new transactions
2. **No data persistence** - Each run is stateless, can't compare to previous
3. **Only current month** - Can't generate reports for previous months
4. **Single pie chart** - Doesn't show budget vs actual side-by-side
5. **No drill-down** - Can't see subcategory details in Telegram

### Edge Cases Handled

- ✅ Months with no revenue (use `-revenue` flag)
- ✅ Over-budget scenarios (red slice in chart)
- ✅ Multiple document types (purchase invoices + receipts)
- ✅ Rate limiting (batch fetching, 7-day chunks)
- ✅ Parent-child account relationships

### Edge Cases NOT Handled

- ❌ Multiple invoices in same month (currently sums all revenue)
- ❌ Mid-month invoice payment timing
- ❌ Non-21% VAT rates
- ❌ Multiple currencies

---

## Planned Improvements

### High Priority

1. **Data persistence & change detection**

   - Store monthly summaries in SQLite/PostgreSQL
   - Only send Telegram update when changes detected
   - Use Moneybird's change detection endpoints
   - Track: `last_processed_transaction_id`, `last_updated_at`

2. **Historical month support**

   - Add `-month=YYYY-MM` flag to process previous months
   - Useful when updating past administration after month-end
   - Store historical data for comparison

3. **Better visualization**
   - Two charts: Budget allocation vs Actual spending
   - Bar chart option for better absolute value comparison
   - Show trend vs previous month

### Medium Priority

4. **Month-to-month comparison**

   - "Spending up/down X% vs last month"
   - Category-level trends
   - Year-to-date totals

5. **Web view with HTMX or Next.js**

   - Interactive drill-down into subcategories
   - Monthly calendar view
   - Export options (PDF, CSV)

6. **Smarter baseline budgets**
   - Configurable baseline for months without income
   - Budget templates/presets
   - Automatic adjustment based on rolling average

### Low Priority

7. **Multiple VAT rates** - Handle different rates per transaction
8. **Multi-currency support** - Handle foreign transactions
9. **Category budgets** - Set limits per category
10. **Alerts** - Notify when approaching category limits

---

## Development Guidelines

### Code Style

- Keep functions focused and well-named
- Comment complex business logic (especially Moneybird quirks)
- Error handling: log but don't crash on non-critical errors
- Use meaningful variable names (no `totals` reuse for different things)

### Testing Approach

- Test with real Moneybird data (no mocking needed yet)
- Use `-revenue` flag for consistent test scenarios
- Keep test outputs in `.gitignore` (JSON, PNG files)
- Manual testing via `go run main.go` before deploy

### Performance Considerations

- Batch document fetching is critical (don't revert to individual calls)
- 50ms delay between chunks helps with rate limiting
- Document cache prevents duplicate API calls
- Consider caching ledger accounts (they rarely change)

---

## Common Issues & Solutions

### Issue: "Too many financial mutations"

**Cause:** Trying to fetch entire month at once  
**Solution:** Already implemented - 7-day chunking in `GetFinancialMutations`

### Issue: Revenue not showing up

**Cause:** `SalesInvoice` payment ledger account points to bank, not revenue  
**Solution:** Already handled - hardcoded mapping to `omzetAccountID`

### Issue: Document expenses missing

**Cause:** Need to fetch document details to get ledger accounts  
**Solution:** Already implemented - batch document fetching

### Issue: Wrong totals in root categories

**Cause:** Not summing subcategories properly  
**Solution:** Walk parent chain and sum at root level

### Issue: Rate limiting (429 errors)

**Cause:** Too many individual document fetches  
**Solution:** Use batch synchronization endpoints

---

## Moneybird API Quirks

### Important Gotchas

1. **Invoice dates ≠ Payment dates**

   - Invoice created in February
   - Money arrives in April
   - Use transaction date, not invoice date

2. **Financial mutations response structure**

   - `ledger_account_bookings[]` - direct categorizations
   - `payments[]` - linked to documents
   - BOTH can exist on same transaction

3. **Payment ledger_account_id**

   - Points to BANK account, not expense category
   - Must fetch document to get real categorization

4. **Document detail prices**

   - Are POSITIVE (unlike booking prices which are negative)
   - Must SUBTRACT from totals

5. **Rate limiting**
   - Individual document GETs hit limits quickly
   - Batch synchronization endpoints are generous

---

## Example Commands

### Local Development

```bash
# Run for current month
go run main.go

# Run with manual revenue (for months where invoice paid previous month)
go run main.go -revenue=12850.20

# Test without sending to Telegram
# (comment out sendToTelegram section temporarily)
```

### Deployment

```bash
# Deploy to Fly.io
flyctl deploy

# View logs
flyctl logs

# SSH into container
flyctl ssh console

# Set/update secrets
flyctl secrets set MONEYBIRD_API_TOKEN="new-token"
```

### Git Workflow

```bash
# After making changes
git add .
git commit -m "Descriptive message"
git push

# GitHub Action will auto-deploy on next scheduled run
# Or trigger manually via Actions tab
```

---

## Resources

### Documentation

- [Moneybird API Docs](https://developer.moneybird.com/)
- [Telegram Bot API](https://core.telegram.org/bots/api)
- [Fly.io Go Docs](https://fly.io/docs/languages-and-frameworks/golang/)
- [go-chart Library](https://github.com/wcharczuk/go-chart)

### Key Moneybird Endpoints Docs

- [Financial Mutations](https://developer.moneybird.com/api/financial_mutations/)
- [Documents](https://developer.moneybird.com/api/documents/)
- [Ledger Accounts](https://developer.moneybird.com/api/ledger_accounts/)

---

## Questions for Future Development

When continuing development, consider:

1. **Database choice**: SQLite (simple) or PostgreSQL (scalable)?
2. **Change detection**: Poll Moneybird or use webhooks?
3. **Web frontend**: HTMX (simple) or Next.js (rich)?
4. **Multi-user**: Keep single-user or support multiple families?
5. **Notifications**: Just Telegram or add email/SMS?

---

## Contact & Context

**Use Case:** Freelancer in Netherlands tracking family budget  
**Accounting:** Self-managed via Moneybird  
**Payment Flow:** Invoice → Client pays → Split: VAT (21%), Income Tax (30%), Business Expenses, then Family Budget

**Special Requirements:**

- Easy access for family members (via Telegram)
- Real-time daily updates preferred
- Dutch tax/VAT structure
- Distinction between business and family expenses critical

---

## Success Criteria

The MVP is considered successful if:

- ✅ Daily Telegram report arrives at 9 AM
- ✅ Budget calculation is accurate (matches manual calculation)
- ✅ All transactions are categorized correctly
- ✅ Chart is clear and intuitive
- ✅ No manual intervention needed for normal months

Next phase success criteria:

- Reports only when transactions change
- Can generate report for any past month
- Can compare month-to-month trends
- Historical data persisted reliably

---

## Final Notes

This project started as a conversation about better financial visibility for a freelance family. The core insight was that standard accounting reports (Moneybird's balance sheet) don't reflect actual cash flow - invoices are dated when sent, not when paid.

The solution focuses on **transaction dates** (when money moves) rather than invoice dates (when work was billed), giving a real-time view of available family budget.

Key principle: **Simplicity over features.** Each feature was added only when needed, keeping the codebase maintainable and focused on the core use case.

---

**Document Version:** 1.0  
**Last Updated:** 2024-11-16  
**Status:** MVP Complete & Deployed
