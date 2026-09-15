package api

import (
	"context"
	"errors"

	"github.com/bowlinedev/bowline"
	"github.com/bowlinedev/bowline/examples/ledger/ledger"
)

type GetCustomerInput struct {
	ID int64 `json:"id" validate:"required"`
}

type SearchCustomersInput struct {
	Query string `json:"query" validate:"required,min=2"`
}

func (a *API) customers() *bowline.Router {
	return bowline.NewRouter(
		bowline.Query("get", a.getCustomer),
		bowline.Query("search", a.searchCustomers, bowline.Sensitive()),
	)
}

func (a *API) getCustomer(ctx context.Context, in GetCustomerInput) (ledger.Customer, error) {
	c, err := a.store.Customer(in.ID)
	if errors.Is(err, ledger.ErrNotFound) {
		return ledger.Customer{}, bowline.Errorf(bowline.NotFound, "customer %d not found", in.ID)
	}
	return c, err
}

func (a *API) searchCustomers(ctx context.Context, in SearchCustomersInput) (ledger.Page[ledger.Customer], error) {
	return ledger.Page[ledger.Customer]{Items: a.store.SearchCustomers(in.Query)}, nil
}
