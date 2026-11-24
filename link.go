package phajay

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

type PaymentLinkRequest struct {
	Amount      float64 `json:"amount"`
	Description string  `json:"description"`
	OrderNo     string  `json:"orderNo,omitempty"`
	Tag1        string  `json:"tag1,omitempty"`
	Tag2        string  `json:"tag2,omitempty"`
	Tag3        string  `json:"tag3,omitempty"`
}

type PaymentLinkResponse struct {
	Message     string `json:"message"`
	RedirectURL string `json:"redirectURL"`
	OrderNo     string `json:"orderNo,omitempty"`
}

func (c *Client) CreatePaymentLink(ctx context.Context, req PaymentLinkRequest) (*PaymentLinkResponse, error) {
	if req.Amount <= 0 {
		return nil, errors.New("phajay: amount must be greater than 0")
	}
	if strings.TrimSpace(req.Description) == "" {
		return nil, errors.New("phajay: description is required")
	}

	var res PaymentLinkResponse
	if err := c.do(ctx, http.MethodPost, "/link/payment-link", authBasic, req, &res); err != nil {
		return nil, err
	}
	return &res, nil
}
