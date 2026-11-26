package handler

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"chawy-erp-api/internal/delivery/http/dto"
	usecaseStockOut "chawy-erp-api/internal/usecase/stockout"
	appErrors "chawy-erp-api/pkg/errors"
	"chawy-erp-api/pkg/response"

	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp"
)

// StockOutHandler serves /api/v1/stock-outs for the daily stock-out count.
type StockOutHandler struct {
	uc     usecaseStockOut.Usecase
	broker *usecaseStockOut.Broker
	// heartbeat keeps proxies from closing an idle stream.
	heartbeat time.Duration
}

// NewStockOutHandler builds the handler. broker may be nil (no stream).
func NewStockOutHandler(uc usecaseStockOut.Usecase, broker *usecaseStockOut.Broker) *StockOutHandler {
	return &StockOutHandler{uc: uc, broker: broker, heartbeat: 20 * time.Second}
}

// Stream serves GET /stock/stream (alias /stock-outs/stream) as Server-Sent
// Events: one "change" event whenever a batch is saved, voided or reviewed,
// or the settings change. Screens reload the summary on each event and keep
// polling every few seconds as a fallback. The client reads it with fetch
// (so the Bearer token stays in the Authorization header, not the URL).
func (h *StockOutHandler) Stream(c *fiber.Ctx) error {
	if h.broker == nil {
		return response.Error(c, fiber.StatusNotImplemented, "STREAM_UNAVAILABLE", "live updates are not enabled")
	}
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")
	events, cancel := h.broker.Subscribe()
	heartbeat := h.heartbeat
	c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
		defer cancel()
		fmt.Fprint(w, "retry: 5000\n: connected\n\n")
		if w.Flush() != nil {
			return
		}
		tick := time.NewTicker(heartbeat)
		defer tick.Stop()
		for {
			select {
			case e, ok := <-events:
				if !ok {
					return
				}
				data, _ := json.Marshal(e)
				fmt.Fprintf(w, "event: change\ndata: %s\n\n", data)
			case <-tick.C:
				fmt.Fprint(w, ": ping\n\n")
			}
			if w.Flush() != nil {
				return
			}
		}
	}))
	return nil
}

func stockOutActor(c *fiber.Ctx) usecaseStockOut.Actor {
	id, _ := c.Locals("userID").(uint)
	role, _ := c.Locals("role").(string)
	email, _ := c.Locals("email").(string)
	return usecaseStockOut.Actor{ID: id, Role: role, Email: email}
}

func (h *StockOutHandler) fail(c *fiber.Ctx, err error) error {
	var appErr *appErrors.AppError
	if errors.As(err, &appErr) {
		return response.Error(c, appErr.StatusCode, appErr.Code, appErr.Message)
	}
	return response.InternalServerError(c, "Stock-out request failed")
}

func stockOutConfig(v *usecaseStockOut.ConfigView) fiber.Map {
	return fiber.Map{"config": dto.StockOutConfigFromDomain(v.Config), "skuCodes": v.SKUCodes, "updatedAt": v.UpdatedAt, "updatedBy": v.UpdatedBy}
}

func batchID(c *fiber.Ctx) (uint, error) {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil || id == 0 {
		return 0, appErrors.NewAppError("VALIDATION_ERROR", "Invalid batch id", 400)
	}
	return uint(id), nil
}

// GetConfig serves GET /stock-outs/config.
func (h *StockOutHandler) GetConfig(c *fiber.Ctx) error {
	v, err := h.uc.GetConfig(c.UserContext())
	if err != nil {
		return h.fail(c, err)
	}
	return response.OK(c, stockOutConfig(v))
}

// SaveConfig serves PUT /stock-outs/config (owner).
func (h *StockOutHandler) SaveConfig(c *fiber.Ctx) error {
	var req dto.StockOutConfigDTO
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid JSON body", "VALIDATION_ERROR")
	}
	v, err := h.uc.SaveConfig(c.UserContext(), req.ToDomain(), stockOutActor(c).Email)
	if err != nil {
		return h.fail(c, err)
	}
	return response.OK(c, stockOutConfig(v), "Stock-out settings saved")
}

// Summary serves GET /stock-outs/summary?date=YYYY-MM-DD: per box, out that
// day, on hand and the lot being taken from.
func (h *StockOutHandler) Summary(c *fiber.Ctx) error {
	s, err := h.uc.Summary(c.UserContext(), c.Query("date"))
	if err != nil {
		return h.fail(c, err)
	}
	return response.OK(c, s)
}

// History serves GET /stock-outs/history?from=&to=.
func (h *StockOutHandler) History(c *fiber.Ctx) error {
	rows, err := h.uc.History(c.UserContext(), c.Query("from"), c.Query("to"))
	if err != nil {
		return h.fail(c, err)
	}
	return response.OK(c, rows)
}

// List serves GET /stock-outs?date=YYYY-MM-DD.
func (h *StockOutHandler) List(c *fiber.Ctx) error {
	batches, err := h.uc.ListBatches(c.UserContext(), c.Query("date"))
	if err != nil {
		return h.fail(c, err)
	}
	out := make([]dto.StockOutBatchDTO, 0, len(batches))
	for i := range batches {
		out = append(out, dto.StockOutBatchFromDomain(&batches[i]))
	}
	return response.OK(c, out)
}

// Create serves POST /stock-outs. A replayed clientId answers 200 with the
// first batch; a new batch answers 201.
func (h *StockOutHandler) Create(c *fiber.Ctx) error {
	var req dto.StockOutBatchRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid JSON body", "VALIDATION_ERROR")
	}
	in := usecaseStockOut.BatchInput{ClientID: req.ClientID, CountedAt: req.CreatedAt, Actor: stockOutActor(c)}
	for _, l := range req.Lines {
		in.Lines = append(in.Lines, usecaseStockOut.CountLine{SKUID: l.SKUID, Qty: l.Qty})
	}
	res, err := h.uc.CreateBatch(c.UserContext(), in)
	if err != nil {
		return h.fail(c, err)
	}
	body := fiber.Map{"batch": dto.StockOutBatchFromDomain(res.Batch), "replayed": res.Replayed}
	if res.Replayed {
		return response.OK(c, body, "Batch already recorded")
	}
	return response.Created(c, body, "Batch recorded")
}

// Void serves POST /stock-outs/:id/void.
func (h *StockOutHandler) Void(c *fiber.Ctx) error {
	id, err := batchID(c)
	if err != nil {
		return h.fail(c, err)
	}
	b, err := h.uc.VoidBatch(c.UserContext(), id, stockOutActor(c))
	if err != nil {
		return h.fail(c, err)
	}
	return response.OK(c, dto.StockOutBatchFromDomain(b), "Batch voided")
}

// Review serves POST /stock-outs/:id/review (owner).
func (h *StockOutHandler) Review(c *fiber.Ctx) error {
	id, err := batchID(c)
	if err != nil {
		return h.fail(c, err)
	}
	var req dto.StockOutReviewRequest
	if len(c.Body()) > 0 {
		if err := c.BodyParser(&req); err != nil {
			return response.BadRequest(c, "Invalid JSON body", "VALIDATION_ERROR")
		}
	}
	b, err := h.uc.ReviewBatch(c.UserContext(), id, stockOutActor(c), req.Note)
	if err != nil {
		return h.fail(c, err)
	}
	return response.OK(c, dto.StockOutBatchFromDomain(b), "Batch reviewed")
}
