# go-phajay

Lightweight Go SDK for the PhaJay Payment Gateway (API v1), with an optional
Echo v4 webhook adapter.

The core package (`github.com/dee25092005/go-phajay`) uses only the Go
standard library. The Echo adapter lives in the subpackage
`github.com/dee25092005/go-phajay/echo`; Echo is only used if you import that
subpackage.

---

## How It Works

```text
  [Your Go / Echo Backend]
            |
            |  1. phajay.NewFromEnv()
            |     (reads PHAJAY_SECRET_KEY + PHAJAY_ENV: test | production | mock)
            v
  [go-phajay Client]
            |
            |-- Validation guard: in "test"/"mock" mode blocks amount outside 1-999
            |                     and Lao text; blocks Lao/Thai on BCEL;
            |                     APB description must be under 25 characters
            |-- Route prefix:     "/test" only when PHAJAY_ENV=mock
            |-- Auth header:      "secretKey" for QR, "Basic" for Link and Status
            |
            |  2. GenerateQR() / CreatePaymentLink()
            v
  [PhaJay Gateway API] -> returns transactionId (or linkCode) + qrCode / link
            |
            |  3. Customer pays in a bank app (onepay:// for BCEL) or the PhaJay app (mock)
            v
  [Payment confirmation, two paths]
            |-- Push: PhaJay POSTs to your webhook -> phajayecho.WebhookHandler
            |         verifies X-Signature (HMAC-SHA256, base64) on the raw body
            '-- Pull: pg.CheckStatus(ctx, idOrLinkCode)
                      check status.Data.IsPaid (Data.ID() falls back to LinkCode)
```

## Supported Banks

| Constant | Bank | Notes |
| --- | --- | --- |
| `phajay.BankBCEL` | BCEL (BCEL One) | Returns an `onepay://` link. Lao and Thai characters are blocked in the description. |
| `phajay.BankJDB` | Joint Development Bank | Standard QR |
| `phajay.BankLDB` | Lao Development Bank | Standard QR |
| `phajay.BankIB` | Indochina Bank | Standard QR |
| `phajay.BankSTB` | ST Bank Laos | Standard QR |
| `phajay.BankAPB` | Agricultural Promotion Bank | Description must be shorter than 25 characters |
| `phajay.BankMMoney` | M MoneyX | Standard QR |

## Environments

| PHAJAY_ENV | Key | Endpoints | Rules |
| --- | --- | --- | --- |
| `test` (alias `dev`) | pre-KYC test key | standard | amount 1-999, no Lao text |
| `production` (default) | KYC-approved secret key | standard | any amount above 0 |
| `mock` (alias `sandbox`) | test key | `/test/*` simulator (needs the PhaJay testing APK) | amount 1-999, no Lao text |

Any unrecognised value is treated as `production`, so check the spelling.

## Installation

```bash
go get github.com/dee25092005/go-phajay@v1.0.2
```

## Usage

### Step 1: Environment variables

```dotenv
PHAJAY_ENV=test
PHAJAY_SECRET_KEY=your_test_or_live_secret_key
```

Optional: `PHAJAY_BASE_URL` overrides the default gateway URL.

### Step 2: Create a payment

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

    // Option A: bank QR
    qr, err := pg.GenerateQR(context.Background(), phajay.BankBCEL, phajay.QRRequest{
        Amount:      10,
        Description: "ORDER-1001",
        OrderNo:     "ORDER-1001",
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("QR Link:", qr.Link, "TxnID:", qr.TransactionID)

    // Option B: hosted payment link
    link, err := pg.CreatePaymentLink(context.Background(), phajay.PaymentLinkRequest{
        Amount:      500,
        Description: "ORDER-1002",
        OrderNo:     "ORDER-1002",
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("Checkout URL:", link.RedirectURL, "LinkCode:", link.LinkCode)
}
```

### Step 3: Verify the payment

```go
// 1. Webhook in Echo v4 (verifies X-Signature automatically)
e.POST("/webhook/phajay", phajayecho.WebhookHandler(pg, func(c echo.Context, event phajay.WebhookPayload) error {
    if event.IsPaid() {
        // Mark the order paid using event.OrderNo / event.TransactionID / event.PaidAmount()
    }
    return c.JSON(http.StatusOK, map[string]string{"message": "OK"})
}))

// 2. Fallback status check
status, err := pg.CheckStatus(ctx, qr.TransactionID)
if err == nil && status.Data.IsPaid {
    // Order confirmed paid
}
```

Without Echo, call `pg.ParseWebhook(rawBody, r.Header.Get(phajay.SignatureHeader))`
with the raw request body. It returns `phajay.ErrInvalidSignature` when the
signature does not match.

## Errors

Gateway errors are returned as `*phajay.APIError` (fields `StatusCode`,
`Message`, `Body`). Use `errors.As` to inspect them. `CheckStatus` returns an
`APIError` with status 404 when the gateway reports `PAYMENT_NOT_FOUND`.

## Testing

```bash
go test ./...
```

The tests run against a local `httptest` server and never call the real gateway.

## License

See `LICENSE`.
