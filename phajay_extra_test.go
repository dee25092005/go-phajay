package phajay_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dee25092005/go-phajay"
)

func newStubServer(status int, body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

func TestNew_RequiresSecretKey(t *testing.T) {
	if _, err := phajay.New(phajay.Config{}); err == nil {
		t.Fatal("expected error for empty secret key")
	}
	if _, err := phajay.New(phajay.Config{SecretKey: "   "}); err == nil {
		t.Fatal("expected error for whitespace-only secret key")
	}
}

func TestNewFromEnv_EnvAliases(t *testing.T) {
	cases := map[string]phajay.Env{
		"test":       phajay.EnvTest,
		"dev":        phajay.EnvTest,
		"mock":       phajay.EnvMock,
		"sandbox":    phajay.EnvMock,
		"production": phajay.EnvProduction,
		"":           phajay.EnvProduction,
	}
	for raw, want := range cases {
		t.Setenv("PHAJAY_SECRET_KEY", "key")
		t.Setenv("PHAJAY_ENV", raw)
		c, err := phajay.NewFromEnv()
		if err != nil {
			t.Fatalf("env %q: unexpected error: %v", raw, err)
		}
		if c.Env() != want {
			t.Fatalf("env %q: got %q, want %q", raw, c.Env(), want)
		}
	}
}

func TestGenerateQR_ValidationRules(t *testing.T) {
	srv := newStubServer(http.StatusOK, `{"message":"SUCCESSFULLY","transactionId":"T1"}`)
	defer srv.Close()

	prod, _ := phajay.New(phajay.Config{SecretKey: "k", Env: phajay.EnvProduction, BaseURL: srv.URL})
	test, _ := phajay.New(phajay.Config{SecretKey: "k", Env: phajay.EnvTest, BaseURL: srv.URL})
	ctx := context.Background()

	// Banks that are still in development are rejected
	for _, b := range []string{"stb", "apb", "m-money"} {
		if _, err := prod.GenerateQR(ctx, phajay.Bank(b), phajay.QRRequest{Amount: 10, Description: "x"}); err == nil {
			t.Fatalf("bank %q is not supported yet and should be rejected", b)
		}
	}

	// Supported banks pass
	for _, b := range []phajay.Bank{phajay.BankBCEL, phajay.BankJDB, phajay.BankLDB, phajay.BankIB} {
		if _, err := prod.GenerateQR(ctx, b, phajay.QRRequest{Amount: 10, Description: "x"}); err != nil {
			t.Fatalf("bank %q should be supported: %v", b, err)
		}
	}

	// Amount and description basics
	if _, err := prod.GenerateQR(ctx, phajay.BankJDB, phajay.QRRequest{Amount: 0, Description: "x"}); err == nil {
		t.Fatal("amount 0 should be rejected")
	}
	if _, err := prod.GenerateQR(ctx, phajay.BankJDB, phajay.QRRequest{Amount: 10, Description: "   "}); err == nil {
		t.Fatal("blank description should be rejected")
	}

	// Unsupported bank
	if _, err := prod.GenerateQR(ctx, phajay.Bank("xyz"), phajay.QRRequest{Amount: 10, Description: "x"}); err == nil {
		t.Fatal("unsupported bank should be rejected")
	}

	// Test mode lower bound
	if _, err := test.GenerateQR(ctx, phajay.BankJDB, phajay.QRRequest{Amount: 0.5, Description: "x"}); err == nil {
		t.Fatal("test mode amount below 1 should be rejected")
	}
}

func TestAPIError_FromErrorResponses(t *testing.T) {
	srv := newStubServer(http.StatusUnauthorized, `{"message":"Unauthorized"}`)
	defer srv.Close()

	c, _ := phajay.New(phajay.Config{SecretKey: "bad", Env: phajay.EnvProduction, BaseURL: srv.URL})
	_, err := c.GenerateQR(context.Background(), phajay.BankJDB, phajay.QRRequest{Amount: 10, Description: "x"})

	var apiErr *phajay.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized || apiErr.Message != "Unauthorized" {
		t.Fatalf("unexpected APIError: %+v", apiErr)
	}
	if apiErr.Error() != "phajay: Unauthorized (status 401)" {
		t.Fatalf("unexpected error string: %q", apiErr.Error())
	}

	srv500 := newStubServer(http.StatusInternalServerError, ``)
	defer srv500.Close()
	c500, _ := phajay.New(phajay.Config{SecretKey: "k", Env: phajay.EnvProduction, BaseURL: srv500.URL})
	_, err = c500.GenerateQR(context.Background(), phajay.BankJDB, phajay.QRRequest{Amount: 10, Description: "x"})
	if !errors.As(err, &apiErr) || apiErr.Error() != "phajay: request failed with status 500" {
		t.Fatalf("unexpected 500 error: %v", err)
	}
}

func TestCheckStatus_NotFoundAndEmptyID(t *testing.T) {
	srv := newStubServer(http.StatusOK, `{"message":"PAYMENT_NOT_FOUND","data":{}}`)
	defer srv.Close()

	c, _ := phajay.New(phajay.Config{SecretKey: "k", Env: phajay.EnvTest, BaseURL: srv.URL})

	_, err := c.CheckStatus(context.Background(), "missing-id")
	var apiErr *phajay.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 APIError for PAYMENT_NOT_FOUND, got: %v", err)
	}

	if _, err := c.CheckStatus(context.Background(), "  "); err == nil {
		t.Fatal("expected error for empty id")
	}
}

func TestCreatePaymentLink_MockPath(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":"SUCCESSFULLY","redirectURL":"https://x.test?linkCode=mock-1"}`))
	}))
	defer srv.Close()

	c, _ := phajay.New(phajay.Config{SecretKey: "k", Env: phajay.EnvMock, BaseURL: srv.URL})
	res, err := c.CreatePaymentLink(context.Background(), phajay.PaymentLinkRequest{Amount: 10, Description: "ORDER-1"})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/test/payment/get-payment-link" || res.LinkCode != "mock-1" {
		t.Fatalf("unexpected mock link call: path=%s res=%+v", gotPath, res)
	}
}

func TestParseWebhook_SignatureAndFields(t *testing.T) {
	secret := "whsec"
	c, _ := phajay.New(phajay.Config{SecretKey: secret})

	// QR callback: refNo is a number, production fields
	qrRaw := []byte(`{"refNo":580960503,"transactionId":"TXN-1","status":"PAYMENT_COMPLETED","txnAmount":100,"paymentMethod":"BCEL"}`)
	p, err := c.ParseWebhook(qrRaw, signPayload(qrRaw, secret))
	if err != nil {
		t.Fatal(err)
	}
	if string(p.RefNo) != "580960503" || !p.IsPaid() || p.PaidAmount() != 100 || p.Bank() != "BCEL" {
		t.Fatalf("unexpected QR payload: %+v", p)
	}

	// Payment Link callback: refNo is a string, sandbox fallbacks (amount, origin)
	linkRaw := []byte(`{"refNo":"abc","linkCode":"L1","status":"WAITING","amount":50,"origin":"JDB"}`)
	p, err = c.ParseWebhook(linkRaw, signPayload(linkRaw, secret))
	if err != nil {
		t.Fatal(err)
	}
	if string(p.RefNo) != `"abc"` || p.IsPaid() || p.PaidAmount() != 50 || p.Bank() != "JDB" {
		t.Fatalf("unexpected link payload: %+v", p)
	}

	// Bad signature
	if _, err := c.ParseWebhook(qrRaw, "forged"); !errors.Is(err, phajay.ErrInvalidSignature) {
		t.Fatalf("expected ErrInvalidSignature, got %v", err)
	}

	// Valid signature but malformed JSON
	bad := []byte(`{not json`)
	if _, err := c.ParseWebhook(bad, signPayload(bad, secret)); err == nil || errors.Is(err, phajay.ErrInvalidSignature) {
		t.Fatalf("expected decode error, got %v", err)
	}
}

func TestVerifyWebhookSignature_EmptyInputs(t *testing.T) {
	body := []byte(`{"a":1}`)
	if phajay.VerifyWebhookSignature(body, "", "secret") {
		t.Fatal("empty signature must fail")
	}
	if phajay.VerifyWebhookSignature(body, signPayload(body, ""), "") {
		t.Fatal("empty secret must fail")
	}
	if !phajay.VerifyWebhookSignature(body, signPayload(body, "secret"), "secret") {
		t.Fatal("valid signature must pass")
	}
}
