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

func TestGenerateQR_EnvModesAndRules(t *testing.T) {
	var gotPath, gotSecretHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotSecretHeader = r.Header.Get("secretKey")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":"SUCCESSFULLY","transactionId":"TXN-UUID-123","qrCode":"000201...","link":"onepay://qr/000201..."}`))
	}))
	defer srv.Close()

	// 1. EnvTest hits standard /payment/generate-bcel-qr (returns onepay://)
	testClient, err := phajay.New(phajay.Config{
		SecretKey: "test_key_123",
		Env:       phajay.EnvTest,
		BaseURL:   srv.URL,
	})
	if err != nil {
		t.Fatal(err)
	}

	res, err := testClient.GenerateQR(context.Background(), phajay.BankBCEL, phajay.QRRequest{
		Amount:      999,
		Description: "ORDER-1001",
		OrderNo:     "ORDER-1001",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/payment/generate-bcel-qr" || gotSecretHeader != "test_key_123" || res.Link != "onepay://qr/000201..." {
		t.Fatalf("unexpected test response: path=%s res=%+v", gotPath, res)
	}

	// 2. EnvTest rejects amount > 999
	if _, err := testClient.GenerateQR(context.Background(), phajay.BankBCEL, phajay.QRRequest{
		Amount:      1000,
		Description: "ORDER-1002",
	}); err == nil {
		t.Fatal("expected error for amount > 999 in test mode")
	}

	// 3. EnvTest rejects Lao characters even on JDB
	if _, err := testClient.GenerateQR(context.Background(), phajay.BankJDB, phajay.QRRequest{
		Amount:      10,
		Description: "ຊື້ປຶ້ມ",
	}); err == nil {
		t.Fatal("expected error for Lao characters in test mode")
	}

	// 4. EnvProduction allows amount > 999 and Lao text on JDB, while still blocking Lao on BCEL
	prodClient, _ := phajay.New(phajay.Config{
		SecretKey: "live_secret_123",
		Env:       phajay.EnvProduction,
		BaseURL:   srv.URL,
	})
	if _, err := prodClient.GenerateQR(context.Background(), phajay.BankJDB, phajay.QRRequest{
		Amount:      50000,
		Description: "ຊື້ປຶ້ມ",
	}); err != nil {
		t.Fatalf("expected production JDB with Lao text and 50000 LAK to succeed, got: %v", err)
	}
	if _, err := prodClient.GenerateQR(context.Background(), phajay.BankBCEL, phajay.QRRequest{
		Amount:      50000,
		Description: "ຊື້ປຶ້ມ",
	}); err == nil {
		t.Fatal("expected BCEL to reject Lao text even in production")
	}

	// 5. EnvMock routes to /test/payment/generate-bcel-qr
	mockClient, _ := phajay.New(phajay.Config{
		SecretKey: "test_key_123",
		Env:       phajay.EnvMock,
		BaseURL:   srv.URL,
	})
	if _, err := mockClient.GenerateQR(context.Background(), phajay.BankBCEL, phajay.QRRequest{
		Amount:      10,
		Description: "ORDER-1001",
	}); err != nil || gotPath != "/test/payment/generate-bcel-qr" {
		t.Fatalf("expected mock path /test/payment/generate-bcel-qr, got %s (err=%v)", gotPath, err)
	}
}

func TestPaymentLinkAndStatus_BasicAuth(t *testing.T) {
	var gotAuth, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"message":"SUCCESSFULLY","redirectURL":"https://payment-link.phajay.co?amount=500&linkCode=abc-uuid","orderNo":"ORD-1"}`))
			return
		}
		_, _ = w.Write([]byte(`{"message":"SUCCESSFULLY","data":{"transactionId":"","linkCode":"abc-uuid","status":"WAITING","isPaid":false,"amount":500,"currency":"LAK"}}`))
	}))
	defer srv.Close()

	client, _ := phajay.New(phajay.Config{
		SecretKey: "test_key_123",
		Env:       phajay.EnvTest,
		BaseURL:   srv.URL,
	})

	expectedAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("test_key_123"))

	linkRes, err := client.CreatePaymentLink(context.Background(), phajay.PaymentLinkRequest{
		Amount:      500,
		Description: "Book Order",
		OrderNo:     "ORD-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/link/payment-link" || gotAuth != expectedAuth || linkRes.LinkCode != "abc-uuid" {
		t.Fatalf("unexpected payment link call: path=%s auth=%s res=%+v", gotPath, gotAuth, linkRes)
	}

	statusRes, err := client.CheckStatus(context.Background(), linkRes.LinkCode)
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/payment/check-transaction/status/abc-uuid" || statusRes.Data.ID() != "abc-uuid" {
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

	badReq := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(`{"status":"PAYMENT_COMPLETED"}`))
	badReq.Header.Set("Content-Type", "application/json")
	badReq.Header.Set(phajay.SignatureHeader, "forged-sig")
	badRec := httptest.NewRecorder()
	e.ServeHTTP(badRec, badReq)
	if badRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for bad signature, got %d", badRec.Code)
	}

	raw := []byte(`{"message":"SUCCESS","refNo":580960503,"transactionId":"TXN-77","status":"PAYMENT_COMPLETED","txnAmount":100,"paymentMethod":"BCEL"}`)
	goodReq := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(raw))
	goodReq.Header.Set("Content-Type", "application/json")
	goodReq.Header.Set(phajay.SignatureHeader, signPayload(raw, secret))
	goodRec := httptest.NewRecorder()
	e.ServeHTTP(goodRec, goodReq)

	if goodRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid signature, got %d", goodRec.Code)
	}
	if !captured.IsPaid() || captured.TransactionID != "TXN-77" || captured.PaidAmount() != 100 || captured.Bank() != "BCEL" {
		t.Fatalf("unexpected captured payload: %+v", captured)
	}
}
