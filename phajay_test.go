package phajay_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dee25092005/go-phajay"
	phajayecho "github.com/dee25092005/go-phajay/echo"
	"github.com/labstack/echo/v4"
)

func signPayload(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func TestGenerateQR_SandboxVsProdAndAuth(t *testing.T) {
	var gotPath, gotSecretHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotSecretHeader = r.Header.Get("secretKey")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":"SUCCESS","transactionId":"TXN123","qrCode":"000201...","link":"onepay://qr/..."}`))
	}))
	defer srv.Close()

	sandboxClient, err := phajay.New(phajay.Config{
		SecretKey: "dev_secret_123",
		IsSandbox: true,
		BaseURL:   srv.URL,
	})
	if err != nil {
		t.Fatal(err)
	}

	res, err := sandboxClient.GenerateQR(context.Background(), phajay.BankBCEL, phajay.QRRequest{
		Amount:      10000,
		Description: "ORDER-001",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/test/payment/generate-bcel-qr" {
		t.Fatalf("expected sandbox path /test/payment/generate-bcel-qr, got %s", gotPath)
	}
	if gotSecretHeader != "dev_secret_123" {
		t.Fatalf("expected secretKey header, got %s", gotSecretHeader)
	}
	if res.TransactionID != "TXN123" {
		t.Fatalf("unexpected transactionId: %s", res.TransactionID)
	}
}

func TestPaymentLinkAndStatus_BasicAuth(t *testing.T) {
	var gotAuth, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"message":"SUCCESSFULLY","redirectURL":"https://payment-link.phajay.co?linkCode=abc","orderNo":"ORD-1"}`))
			return
		}
		_, _ = w.Write([]byte(`{"message":"SUCCESSFULLY","data":{"transactionId":"TXN-9","status":"PAYMENT_COMPLETED","isPaid":true,"amount":50000,"currency":"LAK"}}`))
	}))
	defer srv.Close()

	client, _ := phajay.New(phajay.Config{
		SecretKey: "live_secret",
		BaseURL:   srv.URL,
	})

	expectedAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("live_secret"))

	linkRes, err := client.CreatePaymentLink(context.Background(), phajay.PaymentLinkRequest{
		Amount:      50000,
		Description: "Book Order",
		OrderNo:     "ORD-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/link/payment-link" || gotAuth != expectedAuth || linkRes.RedirectURL == "" {
		t.Fatalf("unexpected payment link call: path=%s auth=%s res=%+v", gotPath, gotAuth, linkRes)
	}

	statusRes, err := client.CheckStatus(context.Background(), "TXN-9")
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/payment/check-transaction/status/TXN-9" || !statusRes.Data.IsPaid {
		t.Fatalf("unexpected status call: path=%s res=%+v", gotPath, statusRes)
	}
}

func TestEchoWebhookHandler(t *testing.T) {
	secret := "webhook_secret_key"
	client, _ := phajay.New(phajay.Config{SecretKey: secret})

	e := echo.New()
	var captured phajay.WebhookPayload
	e.POST("/webhook", phajayecho.WebhookHandler(client, func(c echo.Context, event phajay.WebhookPayload) error {
		captured = event
		return c.JSON(http.StatusOK, map[string]string{"message": "OK"})
	}))

	// 1. Invalid signature must return 401
	badReq := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(`{"status":"PAYMENT_COMPLETED"}`))
	badReq.Header.Set("Content-Type", "application/json")
	badReq.Header.Set(phajay.SignatureHeader, "forged-sig")
	badRec := httptest.NewRecorder()
	e.ServeHTTP(badRec, badReq)
	if badRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for bad signature, got %d", badRec.Code)
	}

	// 2. Valid signature (with numeric refNo from QR callback) must return 200
	raw := []byte(`{"message":"SUCCESS","refNo":580960503,"transactionId":"TXN-77","status":"PAYMENT_COMPLETED","txnAmount":10000,"paymentMethod":"BCEL"}`)
	goodReq := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(raw))
	goodReq.Header.Set("Content-Type", "application/json")
	goodReq.Header.Set(phajay.SignatureHeader, signPayload(raw, secret))
	goodRec := httptest.NewRecorder()
	e.ServeHTTP(goodRec, goodReq)

	if goodRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid signature, got %d", goodRec.Code)
	}
	if !captured.IsPaid() || captured.TransactionID != "TXN-77" {
		t.Fatalf("unexpected captured payload: %+v", captured)
	}
}
