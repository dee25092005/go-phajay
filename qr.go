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
// Link is a deeplink that opens the paying bank's own mobile app (BCEL uses
// onepay://). It may be empty, for example for IB, which has no app deeplink.
type QRResponse struct {
	Message       string `json:"message"`
	TransactionID string `json:"transactionId"`
	QRCode        string `json:"qrCode"`
	Link          string `json:"link"`
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
