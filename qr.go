package phajay

import (
	"context"
	"fmt"
	"net/http"
)

type Bank string

// Supported banks. STB, APB and M MoneyX are in development and not available yet.
const (
	BankBCEL Bank = "bcel"
	BankJDB  Bank = "jdb"
	BankLDB  Bank = "ldb"
	BankIB   Bank = "ib"
)

func (b Bank) valid() bool {
	switch b {
	case BankBCEL, BankJDB, BankLDB, BankIB:
		return true
	default:
		return false
	}
}

type QRRequest struct {
	Amount      float64 `json:"amount"`
	Description string  `json:"description"`
	OrderNo     string  `json:"orderNo,omitempty"`
	Tag1        string  `json:"tag1,omitempty"`
	Tag2        string  `json:"tag2,omitempty"`
	Tag3        string  `json:"tag3,omitempty"`
}

// QRResponse is the gateway reply for a generated bank QR.
//
// Each bank issues its own native-app URI scheme via the Link field.
// The PhaJay gateway uses the phapay:// scheme as the universal deeplink
// wrapper that routes into each bank's native app:
//
//   - BCEL  → phapay://qr/bcel/<qrCode>
//   - LDB   → phapay://qr/ldb/<qrCode>
//   - JDB   → phapay://qr/jdb/<qrCode>
//   - IB    → "" (empty — IB has no mobile-app deeplink)
//
// Use [QRResponse.NativeAppURI] to get the Link, or fall back to constructing
// the URI yourself when the gateway returns an empty string.
type QRResponse struct {
	Message       string `json:"message"`
	TransactionID string `json:"transactionId"`
	QRCode        string `json:"qrCode"`
	Link          string `json:"link"`
}

// NativeAppURI returns the bank-specific mobile-app deeplink URI from the
// gateway response (the Link field). It is empty for IB which has no app.
func (r *QRResponse) NativeAppURI() string {
	return r.Link
}

func (c *Client) GenerateQR(ctx context.Context, bank Bank, req QRRequest) (*QRResponse, error) {
	if !bank.valid() {
		return nil, fmt.Errorf("phajay: unsupported bank %q", bank)
	}
	if err := c.validatePayment(req.Amount, req.Description, bank); err != nil {
		return nil, err
	}

	prefix := ""
	if c.env == EnvMock {
		prefix = "/test"
	}
	path := fmt.Sprintf("%s/payment/generate-%s-qr", prefix, bank)

	var res QRResponse
	if err := c.do(ctx, http.MethodPost, path, authSecretHeader, req, &res); err != nil {
		return nil, err
	}
	return &res, nil
}
