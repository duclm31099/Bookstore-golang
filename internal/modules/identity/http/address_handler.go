package http

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/app/command"
	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/app/dto"
	identity_err "github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain/error"
	"github.com/duclm99/bookstore-backend-v2/internal/platform/httpx"
	"github.com/gin-gonic/gin"
)

// 1. Dependency Inversion: Hợp đồng cho logic Ghi (Command)
type AddressUseCase interface {
	AddAddress(ctx context.Context, cmd command.AddAddressCommand) (int64, error)
	UpdateAddress(ctx context.Context, cmd command.UpdateAddressCommand) error
	DeleteAddress(ctx context.Context, cmd command.DeleteAddressCommand) error
}

// 2. Dependency Inversion: Hợp đồng cho logic Đọc (Query)
type AddressQueryUseCase interface {
	ListAddresses(ctx context.Context, userID int64) ([]*dto.AddressOutput, error)
}

type AddressHandler struct {
	addressUC AddressUseCase
	queryUC   AddressQueryUseCase
}

func NewAddressHandler(addressUC AddressUseCase, queryUC AddressQueryUseCase) *AddressHandler {
	return &AddressHandler{
		addressUC: addressUC,
		queryUC:   queryUC,
	}
}

// --- Helpers (Nên đưa ra một file base_handler.go dùng chung cho cả module) ---

func writeAddressError(c *gin.Context, operation string, err error) {
	switch {
	case errors.Is(err, identity_err.ErrAddressNotFound):
		httpx.Error(c, http.StatusNotFound, "ADDRESS_NOT_FOUND", "address not found")
	default:
		// Ghi log lỗi thật ở đây: logger.Error("Address operation failed", zap.Error(err))
		httpx.Error(c, http.StatusInternalServerError, operation, "internal server error")
	}
}

// --- Handlers ---

func (h *AddressHandler) ListAddresses(c *gin.Context) {
	userID, ok := getAuthUser(c)
	if !ok {
		return
	}

	out, err := h.queryUC.ListAddresses(c.Request.Context(), userID)
	if err != nil {
		writeAddressError(c, "LIST_ADDRESSES_FAILED", err)
		return
	}
	httpx.Success(c, http.StatusOK, "OK", out)
}

func (h *AddressHandler) AddAddress(c *gin.Context) {
	userID, ok := getAuthUser(c)
	if !ok {
		return
	}

	var req AddressRequest
	if !bindJSON(c, &req) {
		return
	}

	id, err := h.addressUC.AddAddress(c.Request.Context(), command.AddAddressCommand{
		UserID:       userID,
		Line1:        req.Line1,
		Line2:        req.Line2,
		ProvinceCode: req.ProvinceCode,
		DistrictCode: req.DistrictCode,
		WardCode:     req.WardCode,
		PostalCode:   req.PostalCode,
		CountryCode:  req.CountryCode,
		IsDefault:    req.IsDefault,
	})
	if err != nil {
		writeAddressError(c, "ADD_ADDRESS_FAILED", err)
		return
	}
	httpx.Success(c, http.StatusCreated, "ADDRESS_CREATED", gin.H{"id": id})
}

func (h *AddressHandler) UpdateAddress(c *gin.Context) {
	userID, ok := getAuthUser(c)
	if !ok {
		return
	}

	addrID, err := strconv.ParseInt(c.Param("address_id"), 10, 64)
	if err != nil || addrID <= 0 {
		httpx.Error(c, http.StatusBadRequest, "INVALID_ADDRESS_ID", "address_id must be a valid integer")
		return
	}

	var req AddressRequest
	if !bindJSON(c, &req) {
		return
	}

	if err := h.addressUC.UpdateAddress(c.Request.Context(), command.UpdateAddressCommand{
		ID:           addrID,
		UserID:       userID,
		Line1:        req.Line1,
		Line2:        req.Line2,
		ProvinceCode: req.ProvinceCode,
		DistrictCode: req.DistrictCode,
		WardCode:     req.WardCode,
		PostalCode:   req.PostalCode,
		CountryCode:  req.CountryCode,
		IsDefault:    req.IsDefault,
	}); err != nil {
		writeAddressError(c, "UPDATE_ADDRESS_FAILED", err)
		return
	}
	httpx.Success(c, http.StatusOK, "ADDRESS_UPDATED", nil)
}

func (h *AddressHandler) DeleteAddress(c *gin.Context) {
	userID, ok := getAuthUser(c)
	if !ok {
		return
	}

	addrID, err := strconv.ParseInt(c.Param("address_id"), 10, 64)
	if err != nil || addrID <= 0 {
		httpx.Error(c, http.StatusBadRequest, "INVALID_ADDRESS_ID", "address_id must be a valid integer")
		return
	}

	if err := h.addressUC.DeleteAddress(c.Request.Context(), command.DeleteAddressCommand{
		ID:     addrID,
		UserID: userID,
	}); err != nil {
		writeAddressError(c, "DELETE_ADDRESS_FAILED", err)
		return
	}
	httpx.Success(c, http.StatusOK, "ADDRESS_DELETED", nil)
}
