package phajay

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type Bank string

const (
	BankBCEL   Bank = "bcel"
	BankJDB    Bank = "jdb"
	BankLDB    Bank = "ldb"
	BankIB     Bank = "ib"
	BankSTB    Bank = "stb"
	BankAPB    Bank = "apb"
	BankMMoney Bank = "m-money"
)

func (b Bank) valid() bool {
	switch b {
	case BankBCEL, BankJDB, BankLDB, BankIB, BankSTB, BankAPB, BankMMoney:
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

type QRResponse struct {
	Message       string `json:"message"`
	TransactionID string `json:"transactionId"`
	QRCode        string `json:"qrCode"`
	Link          string `json:"link"`
	BillNumber    string `json:"billNumber,omitempty"`
}

func (c *Client) GenerateQR(ctx context.Context, bank Bank, req QRRequest) (*QRResponse, error) {
	if !bank.valid() {
		return nil, fmt.Errorf("phajay: unsupported bank %q", bank)
	}
	if req.Amount <= 0 {
		return nil, errors.New("phajay: amount must be greater than 0")
	}
	if strings.TrimSpace(req.Description) == "" {
		return nil, errors.New("phajay: description is required")
	}

	prefix := ""
	if c.isSandbox {
		prefix = "/test"
	}
	path := fmt.Sprintf("%s/payment/generate-%s-qr", prefix, bank)

	var res QRResponse
	if err := c.do(ctx, http.MethodPost, path, authSecretHeader, req, &res); err != nil {
		return nil, err
	}
	return &res, nil
}
