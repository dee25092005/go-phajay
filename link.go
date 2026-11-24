package phajay

import (
	"context"
	"net/http"
	"net/url"
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
	LinkCode    string `json:"linkCode,omitempty"`
	OrderNo     string `json:"orderNo,omitempty"`
}

func (c *Client) CreatePaymentLink(ctx context.Context, req PaymentLinkRequest) (*PaymentLinkResponse, error) {
	if err := c.validatePayment(req.Amount, req.Description, ""); err != nil {
		return nil, err
	}

	path := "/link/payment-link"
	if c.env == EnvMock {
		path = "/test/payment/get-payment-link"
	}

	var res PaymentLinkResponse
	if err := c.do(ctx, http.MethodPost, path, authBasic, req, &res); err != nil {
		return nil, err
	}

	if res.LinkCode == "" && res.RedirectURL != "" {
		if u, err := url.Parse(strings.TrimSpace(res.RedirectURL)); err == nil {
			res.LinkCode = u.Query().Get("linkCode")
		}
	}
	return &res, nil
}
