package phajay

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const SignatureHeader = "X-Signature"

var ErrInvalidSignature = errors.New("phajay: invalid webhook signature")

type WebhookPayload struct {
	Message        string          `json:"message"`
	RefNo          json.RawMessage `json:"refNo"`
	ExReferenceNo  string          `json:"exReferenceNo"`
	MerchantName   string          `json:"merchantName"`
	Memo           string          `json:"memo"`
	Description    string          `json:"description"`
	Remark         string          `json:"remark"`
	PaymentMethod  string          `json:"paymentMethod"`
	TxnDateTime    string          `json:"txnDateTime"`
	TxnAmount      float64         `json:"txnAmount"`
	BillNumber     string          `json:"billNumber"`
	SourceAccount  string          `json:"sourceAccount"`
	SourceName     string          `json:"sourceName"`
	SourceCurrency string          `json:"sourceCurrency"`
	PaymentID      string          `json:"paymentId"`
	LinkCode       string          `json:"linkCode"`
	TransactionID  string          `json:"transactionId"`
	UserID         string          `json:"userId"`
	OrderNo        string          `json:"orderNo"`
	SuccessURL     string          `json:"successURL"`
	Status         string          `json:"status"`
	Tag1           string          `json:"tag1"`
	Tag2           string          `json:"tag2"`
	Tag3           string          `json:"tag3"`
	Tag4           string          `json:"tag4"`
	Tag5           string          `json:"tag5"`
	Tag6           string          `json:"tag6"`
}

func (p WebhookPayload) IsPaid() bool {
	return p.Status == StatusPaymentCompleted
}

func (c *Client) VerifySignature(rawBody []byte, signature string) bool {
	return VerifyWebhookSignature(rawBody, signature, c.secretKey)
}

func VerifyWebhookSignature(rawBody []byte, signature, secretKey string) bool {
	sig := strings.TrimSpace(signature)
	if sig == "" || secretKey == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write(rawBody)
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(sig))
}

func (c *Client) ParseWebhook(rawBody []byte, signature string) (*WebhookPayload, error) {
	if !c.VerifySignature(rawBody, signature) {
		return nil, ErrInvalidSignature
	}
	var payload WebhookPayload
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return nil, fmt.Errorf("phajay: decode webhook payload: %w", err)
	}
	return &payload, nil
}
