package main

import (
	"log"
	"net/http"

	"github.com/dee25092005/go-phajay"
	phajayecho "github.com/dee25092005/go-phajay/echo"
	"github.com/labstack/echo/v4"
)

func main() {
	// Reads PHAJAY_SECRET_KEY and PHAJAY_ENV=sandbox|production from environment
	pg, err := phajay.NewFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	e := echo.New()

	e.POST("/pay/qr", func(c echo.Context) error {
		qr, err := pg.GenerateQR(c.Request().Context(), phajay.BankBCEL, phajay.QRRequest{
			Amount:      10,
			Description: "ORDER-1001",
			OrderNo:     "ORDER-1001",
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
