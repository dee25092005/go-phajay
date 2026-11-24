package phajay

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
)

const StatusPaymentCompleted = "PAYMENT_COMPLETED"

type TransactionStatusResponse struct {
	Message string          `json:"message"`
	Data    TransactionData `json:"data"`
}

type TransactionData struct {
	TransactionID string  `json:"transactionId"`
	LinkCode      *string `json:"linkCode"`
	OrderNo       *string `json:"orderNo"`
	Status        string  `json:"status"`
	IsPaid        bool    `json:"isPaid"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	Description   string  `json:"description"`
	PaymentMethod string  `json:"paymentMethod"`
	PaymentTime   *string `json:"paymentTime"`
	CreatedAt     string  `json:"createdAt"`
}

func (c *Client) CheckStatus(ctx context.Context, transactionID string) (*TransactionStatusResponse, error) {
	id := strings.TrimSpace(transactionID)
	if id == "" {
		return nil, errors.New("phajay: transactionID is required")
	}

	path := "/payment/check-transaction/status/" + url.PathEscape(id)
	var res TransactionStatusResponse
	if err := c.do(ctx, http.MethodGet, path, authBasic, nil, &res); err != nil {
		return nil, err
	}
	if res.Message == "PAYMENT_NOT_FOUND" {
		return nil, &APIError{StatusCode: http.StatusNotFound, Message: res.Message}
	}
	return &res, nil
}
