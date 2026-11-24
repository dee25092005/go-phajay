package phajay_test

// TestGenerateQR_NativeAppURIPerBank verifies that each bank endpoint returns
// the correct native-app URI scheme from the gateway Link field.
//
// Real PhaJay API uses the phapay:// scheme as the universal deeplink router:
//
//	BCEL → phapay://qr/bcel/<qrCode>
//	LDB  → phapay://qr/ldb/<qrCode>
//	JDB  → phapay://qr/jdb/<qrCode>
//	IB   → "" (empty — IB has no mobile-app deeplink)
//
// The test also confirms that PHAJAY_ENV=sandbox → EnvMock, which routes
// all QR requests under the /test/ prefix.
import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dee25092005/go-phajay"
)

func TestGenerateQR_NativeAppURIPerBank(t *testing.T) {
	// Per-bank stub responses mirroring what the real PhaJay gateway returns.
	bankResponses := map[phajay.Bank]struct {
		body       string
		wantScheme string // expected URI prefix; "" means link must be empty
		wantEmpty  bool
	}{
		phajay.BankBCEL: {
			body:       `{"message":"SUCCESSFULLY","transactionId":"TXN-BCEL","qrCode":"000201BCEL","link":"phapay://qr/bcel/000201BCEL"}`,
			wantScheme: "phapay://",
		},
		phajay.BankLDB: {
			body:       `{"message":"SUCCESSFULLY","transactionId":"TXN-LDB","qrCode":"000201LDB","link":"phapay://qr/ldb/000201LDB"}`,
			wantScheme: "phapay://",
		},
		phajay.BankJDB: {
			body:       `{"message":"SUCCESSFULLY","transactionId":"TXN-JDB","qrCode":"000201JDB","link":"phapay://qr/jdb/000201JDB"}`,
			wantScheme: "phapay://",
		},
		phajay.BankIB: {
			body:      `{"message":"SUCCESSFULLY","transactionId":"TXN-IB","qrCode":"000201IB","link":""}`,
			wantEmpty: true,
		},
	}

	for bank, tc := range bankResponses {
		bank, tc := bank, tc // capture loop vars
		t.Run(string(bank), func(t *testing.T) {
			var capturedPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedPath = r.URL.Path
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			// --- Production mode (no /test prefix) ---
			prodClient, err := phajay.New(phajay.Config{
				SecretKey: "prod_key",
				Env:       phajay.EnvProduction,
				BaseURL:   srv.URL,
			})
			if err != nil {
				t.Fatal(err)
			}

			res, err := prodClient.GenerateQR(context.Background(), bank, phajay.QRRequest{
				Amount:      100,
				Description: "ORDER-PROD",
			})
			if err != nil {
				t.Fatalf("production GenerateQR(%s): %v", bank, err)
			}

			// Verify path: /payment/generate-<bank>-qr
			wantPath := "/payment/generate-" + string(bank) + "-qr"
			if capturedPath != wantPath {
				t.Errorf("prod path: got %q, want %q", capturedPath, wantPath)
			}

			// Verify NativeAppURI helper
			uri := res.NativeAppURI()
			if tc.wantEmpty {
				if uri != "" {
					t.Errorf("bank %s: expected empty NativeAppURI, got %q", bank, uri)
				}
			} else {
				if !strings.HasPrefix(uri, tc.wantScheme) {
					t.Errorf("bank %s: NativeAppURI %q does not start with %q", bank, uri, tc.wantScheme)
				}
			}

			// --- Sandbox / Mock mode (PHAJAY_ENV=sandbox → EnvMock → /test prefix) ---
			sandboxClient, err := phajay.New(phajay.Config{
				SecretKey: "sandbox_key",
				Env:       phajay.EnvMock, // same as PHAJAY_ENV=sandbox
				BaseURL:   srv.URL,
			})
			if err != nil {
				t.Fatal(err)
			}

			_, err = sandboxClient.GenerateQR(context.Background(), bank, phajay.QRRequest{
				Amount:      10,
				Description: "ORDER-SANDBOX",
			})
			if err != nil {
				t.Fatalf("sandbox GenerateQR(%s): %v", bank, err)
			}

			// Verify /test/ prefix is prepended in mock/sandbox mode
			wantSandboxPath := "/test/payment/generate-" + string(bank) + "-qr"
			if capturedPath != wantSandboxPath {
				t.Errorf("sandbox path: got %q, want %q", capturedPath, wantSandboxPath)
			}
		})
	}
}

// TestGenerateQR_SandboxEnvNormalization verifies that the string "sandbox"
// (as set via $env:PHAJAY_ENV='sandbox') normalises to EnvMock, which
// is the value that triggers the /test/ path prefix.
func TestGenerateQR_SandboxEnvNormalization(t *testing.T) {
	t.Setenv("PHAJAY_SECRET_KEY", "$2a$10$/yjThUesmk1rrM.fIk7HHOssL55S3X3U.R7U5747EjYbn9WXr/zSi")
	t.Setenv("PHAJAY_ENV", "sandbox")

	c, err := phajay.NewFromEnv()
	if err != nil {
		t.Fatalf("NewFromEnv: %v", err)
	}
	if c.Env() != phajay.EnvMock {
		t.Errorf("PHAJAY_ENV=sandbox should resolve to EnvMock, got %q", c.Env())
	}
}
