package main

import (
	"net/http"
	"strconv"

	"github.com/knadh/listmonk/models"
	"github.com/labstack/echo/v4"
)

// GetWebhookEndpoints handles GET /api/webhooks and returns all registered outbound webhooks.
func (a *App) GetWebhookEndpoints(c echo.Context) error {
	eps, err := a.core.GetWebhookEndpoints()
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{Data: eps})
}

// CreateWebhookEndpoint handles POST /api/webhooks and registers a new outbound webhook.
func (a *App) CreateWebhookEndpoint(c echo.Context) error {
	var req models.WebhookEndpoint
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if req.URL == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "URL is required")
	}

	ep, err := a.core.CreateWebhookEndpoint(req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{Data: ep})
}

// UpdateWebhookEndpoint handles PUT /api/webhooks/:id and updates an existing outbound webhook.
func (a *App) UpdateWebhookEndpoint(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid ID")
	}

	var req models.WebhookEndpoint
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	ep, err := a.core.UpdateWebhookEndpoint(id, req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{Data: ep})
}

// DeleteWebhookEndpoint handles DELETE /api/webhooks/:id and deletes an outbound webhook by ID.
func (a *App) DeleteWebhookEndpoint(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid ID")
	}

	if err := a.core.DeleteWebhookEndpoint(id); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{Data: true})
}

// GetWebhookLogs returns paginated webhook delivery logs.
func (a *App) GetWebhookLogs(c echo.Context) error {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(c.QueryParam("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	offset := (page - 1) * perPage

	logs, err := a.core.GetWebhookLogs(offset, perPage)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{Data: logs})
}

// TestWebhookEndpoint handles POST /api/webhooks/test and executes a test webhook call.
func (a *App) TestWebhookEndpoint(c echo.Context) error {
	var req struct {
		URL       string `json:"url"`
		Secret    string `json:"secret"`
		EventType string `json:"event_type"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if err := a.core.TestWebhookEndpoint(req.URL, req.Secret, req.EventType); err != nil {
		return err
	}

	return c.JSON(http.StatusOK, okResp{Data: true})
}
