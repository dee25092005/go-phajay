package phajayecho

import (
	"bytes"
	"io"
	"net/http"

	"github.com/dee25092005/go-phajay"
	"github.com/labstack/echo/v4"
)

type HandlerFunc func(c echo.Context, event phajay.WebhookPayload) error

func WebhookHandler(client *phajay.Client, fn HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {

		rawBody, err := io.ReadAll(io.LimitReader(c.Request().Body, 1<<20))
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "Failed to read request body"})
		}
		c.Request().Body = io.NopCloser(bytes.NewReader(rawBody))
		event, err := client.ParseWebhook(rawBody, c.Request().Header.Get(phajay.SignatureHeader))
		if err != nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{"message": "Invalid signature"})
		}
		return fn(c, *event)
	}
}
