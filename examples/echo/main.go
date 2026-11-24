package main

import (
	"log"
	"net/http"
	"strings"

	"github.com/dee25092005/go-phajay"
	phajayecho "github.com/dee25092005/go-phajay/echo"
	"github.com/labstack/echo/v4"
)

type qrInput struct {
	Bank        string  `json:"bank"` // bcel, jdb, ldb, ib (default: bcel)
	Amount      float64 `json:"amount"`
	Description string  `json:"description"`
	OrderNo     string  `json:"orderNo"`
}

func main() {
	// Reads PHAJAY_SECRET_KEY and PHAJAY_ENV=test|production|mock from environment
	pg, err := phajay.NewFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	e := echo.New()

	// POST /pay/qr  {"bank":"jdb","amount":10,"description":"ORDER-1001","orderNo":"ORDER-1001"}
	// The bank in the request decides which PhaJay endpoint is called, so the
	// returned link is that bank's own app deeplink (onepay:// only for BCEL).
	e.POST("/pay/qr", func(c echo.Context) error {
		var in qrInput
		if err := c.Bind(&in); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		}

		bank := phajay.Bank(strings.ToLower(strings.TrimSpace(in.Bank)))
		if bank == "" {
			bank = phajay.BankBCEL
		}

		qr, err := pg.GenerateQR(c.Request().Context(), bank, phajay.QRRequest{
			Amount:      in.Amount,
			Description: in.Description,
			OrderNo:     in.OrderNo,
		})
		if err != nil {
			return c.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, qr)
	})

	e.POST("/webhook/phajay", phajayecho.WebhookHandler(pg, func(c echo.Context, event phajay.WebhookPayload) error {
		if event.IsPaid() {
			log.Printf("Paid: txn=%s order=%s amount=%.0f bank=%s",
				event.TransactionID, event.OrderNo, event.TxnAmount, event.PaymentMethod)
		}
		return c.JSON(http.StatusOK, map[string]string{"message": "OK"})
	}))

	log.Fatal(e.Start(":8080"))
}
