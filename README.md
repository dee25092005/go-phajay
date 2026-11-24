# go-phajay

[![Version](https://img.shields.io/github/v/tag/dee25092005/go-phajay?label=version&sort=semver)](https://github.com/dee25092005/go-phajay/tags)
[![Go
Reference](https://pkg.go.dev/badge/github.com/dee25092005/go-phajay.svg)](https://pkg.go.dev/github.com/dee25092005/go-phajay)
[![Go Report
Card](https://goreportcard.com/badge/github.com/dee25092005/go-phajay)](https://goreportcard.com/report/github.com/dee25092005/go-phajay)
![Views](https://gitviews.com/repo/dee25092005/go-phajay.svg?label=views)
[![License](https://img.shields.io/github/license/dee25092005/go-phajay)](https://github.com/dee25092005/go-phajay/blob/main/LICENSE)

Lightweight, zero-dependency Go SDK for **PhaJay Payment Gateway (API
v1)** with an optional **Echo v4** webhook adapter.

---

## How It Works (Package Flow)

```text
  [Your Go / Echo Backend]
            │
            │  1. phajay.NewFromEnv()
            │     (reads PHAJAY_SECRET_KEY + PHAJAY_ENV: test | production | mock)
            ▼
  [go-phajay Client (c.do)]
            │
            ├──► Validation guard ──► blocks amount > 999 or Lao text in "test"/"mock" mode,
            │                         blocks Lao/Thai on BCEL, enforces APB desc < 25 chars
            ├──► Route prefix     ──► "/test" only when PHAJAY_ENV=mock; standard routes in test/production
            ├──► Auth header      ──► auto-picks "secretKey" (QR) or "Basic" (Link/Status)
            │
            │  2. GenerateQR() / CreatePaymentLink()
            ▼
  [PhaJay Gateway API] ──► returns transactionId (or linkCode in redirectURL) + qrCode / link
            │
            │  3. Customer pays via Bank App (onepay:// for BCEL) or PhaJay App (mock)
            ▼
  [Payment Confirmation — 2 Safe Paths]
            ├──► Primary (Push): PhaJay POSTs /webhook ──► phajayecho.WebhookHandler
            │      └─ verifies X-Signature (HMAC-SHA256) on raw bytes + normalizes sandbox/prod fields
            └──► Fallback (Pull): pg.CheckStatus(ctx, idOrLinkCode)
                   └─ checks status.Data.IsPaid == true (and Data.ID() falls back to LinkCode)
```

## Supported Banks & Rules

---

**Constant** **Bank** **Notes**

---

`phajay.BankBCEL` BCEL (BCEL One) Returns `onepay://`
link; Lao/Thai
characters blocked in
description

`phajay.BankJDB` Joint Development Bank Standard QR generation
(JDB)

`phajay.BankLDB` Lao Development Bank Standard QR generation
(LDB)

`phajay.BankIB` Indochina Bank (IB) Standard QR generation

`phajay.BankSTB` ST Bank Laos (STB) Standard QR generation

`phajay.BankAPB` Agricultural Promotion Description must be
Bank (APB) shorter than 25
characters

`phajay.BankMMoney` M MoneyX Standard QR generation

---

## Installation

```bash
go get github.com/dee25092005/go-phajay@v1.0.1
```

## Easy 3-Step Usage

### Step 1: Set Environment Variables (`.env`)

```dotenv
# "test"       -> pre-KYC testKey against standard endpoints (real onepay://, amount 1-999, no Lao text)
# "production" -> KYC-approved secretKey against standard endpoints (any amount > 0)
# "mock"       -> offline /test/* simulator (requires PhaJay testing APK)
PHAJAY_ENV=test
PHAJAY_SECRET_KEY=your_test_or_live_secret_key
```

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

    // Option A: Generate Bank QR
    qr, err := pg.GenerateQR(context.Background(), phajay.BankBCEL, phajay.QRRequest{
        Amount:      10,
        Description: "ORDER-1001",
        OrderNo:     "ORDER-1001",
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("QR Link:", qr.Link, "TxnID:", qr.TransactionID)

    // Option B: Generate Hosted Payment Link
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

### Step 3: Verify Payment (Webhook + Fallback Check)

```go
// 1. Real-time Webhook in Echo v4 (verifies X-Signature automatically)
e.POST("/webhook/phajay", phajayecho.WebhookHandler(pg, func(c echo.Context, event phajay.WebhookPayload) error {
    if event.IsPaid() {
        // Mark order paid using event.OrderNo / event.TransactionID / event.PaidAmount()
    }
    return c.JSON(http.StatusOK, map[string]string{"message": "OK"})
}))

// 2. Manual / Fallback Status Check
status, err := pg.CheckStatus(ctx, qr.TransactionID)
if err == nil && status.Data.IsPaid {
    // Order confirmed paid
}
```
