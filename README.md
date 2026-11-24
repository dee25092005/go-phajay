# go-phajay

[![Version](https://img.shields.io/github/v/tag/dee25092005/go-phajay?label=version&sort=semver)](https://github.com/dee25092005/go-phajay/tags)
[![Go Reference](https://pkg.go.dev/badge/github.com/dee25092005/go-phajay.svg)](https://pkg.go.dev/github.com/dee25092005/go-phajay)
[![Go Report Card](https://goreportcard.com/badge/github.com/dee25092005/go-phajay)](https://goreportcard.com/report/github.com/dee25092005/go-phajay)
![Views](https://gitviews.com/repo/dee25092005/go-phajay.svg?label=views)
[![License](https://img.shields.io/github/license/dee25092005/go-phajay)](https://github.com/dee25092005/go-phajay/blob/main/LICENSE)

Lightweight, zero-dependency Go SDK for **PhaJay Payment Gateway (API v1)** with an optional **Echo v4** webhook adapter.

---

## What is `go-phajay`?

`go-phajay` lets your Go backend accept QR payments and hosted payment links across major banks and wallets in **Lao PDR** using a single, idiomatic client:

- **Zero Core Dependencies:** The root `phajay` package uses only Go's standard library (`net/http`, `crypto/hmac`, `crypto/sha256`, `encoding/json`).
- **One-Flag Sandbox/Production Switch:** Set `PHAJAY_ENV=sandbox` or `PHAJAY_ENV=production`—the client automatically routes QR requests to `/test/...` in sandbox mode without changing base URLs in code.
- **Automatic Auth Handling:** Abstracts PhaJay's split authentication (`secretKey` header for QR generation vs. `Basic Auth` for Payment Links and Status Checks).
- **Tamper-Proof Echo v4 Webhooks:** The `phajay/echo` adapter reads raw request bytes before JSON binding and verifies `X-Signature` (`HMAC-SHA256`) in constant time.

---

## Supported Banks & Wallets

| Constant            | Code      | Institution Name                             | Payment Link | Generate QR | QR Sandbox |
| :------------------ | :-------- | :------------------------------------------- | :----------: | :---------: | :--------: |
| `phajay.BankBCEL`   | `bcel`    | Banque pour le Commerce Extérieur Lao Public |     Yes      |     Yes     |    Yes     |
| `phajay.BankJDB`    | `jdb`     | Joint Development Bank                       |     Yes      |     Yes     |    Yes     |
| `phajay.BankLDB`    | `ldb`     | Lao Development Bank                         |     Yes      |     Yes     |    Yes     |
| `phajay.BankIB`     | `ib`      | Indochina Bank                               |     Yes      |     Yes     |    Yes     |
| `phajay.BankSTB`    | `stb`     | ST Bank Laos                                 |     Yes      |     Yes     |     —      |
| `phajay.BankAPB`    | `apb`     | Agricultural Promotion Bank                  |     Yes      |     Yes     |     —      |
| `phajay.BankMMoney` | `m-money` | M MoneyX Wallet                              |      —       |     Yes     |     —      |

---

## How It Works (Package Flow)

```text
  [Your Go / Echo Backend]
            │
            │  1. phajay.NewFromEnv()
            │     (reads PHAJAY_ENV + PHAJAY_SECRET_KEY)
            ▼
  [go-phajay Client (c.do)]
            │
            ├──► Sandbox mode? ──► auto-prefixes "/test" on QR routes
            ├──► Auth header?  ──► auto-picks "secretKey" (QR) or "Basic" (Link/Status)
            │
            │  2. GenerateQR() / CreatePaymentLink()
            ▼
  [PhaJay Gateway API] ──► returns transactionId + qrCode / redirectURL
            │
            │  3. Customer scans QR in Bank App (or PhaJay App in sandbox)
            ▼
  [Payment Confirmation — 2 Safe Paths]
            ├──► Primary (Push): PhaJay POSTs /webhook ──► phajayecho.WebhookHandler
            │      └─ verifies X-Signature (HMAC-SHA256) on raw bytes before decoding JSON
            └──► Fallback (Pull): pg.CheckStatus(ctx, transactionId)
                   └─ checks status.Data.IsPaid == true
```

---

## Installation

```bash
go get github.com/dee25092005/go-phajay
```

---

## Easy 3-Step Usage

### Step 1: Set Environment Variables (`.env`)

Switch between **Sandbox** and **Production** by changing only two values in your environment—never edit URLs in your Go code:

```env
PHAJAY_ENV=sandbox
PHAJAY_SECRET_KEY=your_dev_or_live_secret_key
```

> **Tip:** When using PowerShell on Windows with bcrypt-style keys (`$2a$10$...`), wrap the key in **single quotes** (`'...'`) so PowerShell does not expand `$2` or `$10` as variables.

### Step 2: Initialize Client & Create a Payment

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/dee25092005/go-phajay"
)

func main() {
	pg, err := phajay.NewFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	// Option A: Generate Bank QR (auto-routes to /test/payment/generate-bcel-qr in sandbox)
	qr, err := pg.GenerateQR(context.Background(), phajay.BankBCEL, phajay.QRRequest{
		Amount:      15000,
		Description: "ORDER-1001",
		OrderNo:     "ORDER-1001",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("QR String:", qr.QRCode)
	fmt.Println("Transaction ID:", qr.TransactionID)

	// Option B: Generate Hosted Payment Link
	link, err := pg.CreatePaymentLink(context.Background(), phajay.PaymentLinkRequest{
		Amount:      15000,
		Description: "ORDER-1001",
		OrderNo:     "ORDER-1001",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Checkout URL:", link.RedirectURL)
}
```

### Step 3: Verify Payment (Webhook + Fallback Check)

```go
package main

import (
	"log"
	"net/http"

	"github.com/dee25092005/go-phajay"
	phajayecho "github.com/dee25092005/go-phajay/echo"
	"github.com/labstack/echo/v4"
)

func registerRoutes(e *echo.Echo, pg *phajay.Client) {
	// 1. Real-time Webhook in Echo v4 (verifies X-Signature automatically)
	e.POST("/webhook/phajay", phajayecho.WebhookHandler(pg, func(c echo.Context, event phajay.WebhookPayload) error {
		if event.IsPaid() {
			log.Printf("Payment confirmed! Order=%s Txn=%s Amount=%.0f Bank=%s",
				event.OrderNo, event.TransactionID, event.TxnAmount, event.PaymentMethod)
			// Mark order paid in your database
		}
		return c.JSON(http.StatusOK, map[string]string{"message": "OK"})
	}))

	// 2. Manual / Fallback Status Check (if a webhook was missed or delayed)
	e.GET("/pay/status/:id", func(c echo.Context) error {
		status, err := pg.CheckStatus(c.Request().Context(), c.Param("id"))
		if err != nil {
			return c.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, status)
	})
}
```

---

## Package Structure

```text
go-phajay/
├── phajay.go           # Client, Config, NewFromEnv(), and shared HTTP request helper
├── qr.go               # GenerateQR() for BCEL, JDB, LDB, IB, STB, APB, M MoneyX
├── link.go             # CreatePaymentLink()
├── status.go           # CheckStatus() transaction verification
├── webhook.go          # Constant-time HMAC-SHA256 VerifySignature() & WebhookPayload
├── phajay_test.go      # Unit tests using stdlib net/http/httptest
├── echo/
│   └── webhook.go      # Echo v4 raw-body webhook handler
└── examples/
    └── echo/
        └── main.go     # Runnable Echo server example
```

## License

MIT License — see [LICENSE](LICENSE) for details.
