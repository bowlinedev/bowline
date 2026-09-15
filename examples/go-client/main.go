package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/bowlinedev/bowline"
	"github.com/bowlinedev/bowline/examples/ledger/ledgerclient"
)

type OutstandingInput struct {
	CustomerID int64 `json:"customerId,omitempty"`
}

type Outstanding struct {
	Count    int32  `json:"count"`
	Total    string `json:"total"`
	Currency string `json:"currency"`
}

type CustomerInput struct {
	ID int64 `json:"id" validate:"required"`
}

type CustomerReport struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type Reports struct {
	ledger *ledgerclient.Client
}

func Routes(ledger *ledgerclient.Client) *bowline.Router {
	r := &Reports{ledger: ledger}
	return bowline.NewRouter(
		bowline.Mount("reports", bowline.NewRouter(
			bowline.Query("outstanding", r.outstanding, bowline.Description("Outstanding sums every invoice that is neither paid nor void.")),
			bowline.Query("customer", r.customer, bowline.Description("Customer reports on one ledger customer.")),
		)),
	)
}

func (r *Reports) outstanding(ctx context.Context, in OutstandingInput) (Outstanding, error) {
	page, err := r.ledger.Invoices.List(ctx, ledgerclient.ListInvoicesInput{Limit: 100})
	if err != nil {
		return Outstanding{}, err
	}
	var out Outstanding
	var cents int64
	for _, inv := range page.Items {
		if inv.Status == ledgerclient.StatusPaid || inv.Status == ledgerclient.StatusVoid {
			continue
		}
		if in.CustomerID != 0 && inv.CustomerId != in.CustomerID {
			continue
		}
		currency, amount, err := parseMoney(inv.Total)
		if err != nil {
			return Outstanding{}, bowline.Errorf(bowline.Internal, "invoice %d: %v", inv.ID, err)
		}
		if out.Currency == "" {
			out.Currency = currency
		}
		out.Count++
		cents += amount
	}
	out.Total = fmt.Sprintf("%d.%02d", cents/100, cents%100)
	return out, nil
}

func (r *Reports) customer(ctx context.Context, in CustomerInput) (CustomerReport, error) {
	c, err := r.ledger.Customers.Get(ctx, ledgerclient.GetCustomerInput{ID: in.ID})
	if err != nil {
		var be *bowline.Error
		if errors.As(err, &be) && be.Code == bowline.NotFound {
			return CustomerReport{}, bowline.Errorf(bowline.FailedPrecondition, "customer %d is not in the ledger yet", in.ID)
		}
		return CustomerReport{}, err
	}
	return CustomerReport{ID: c.ID, Name: c.Name, Email: c.Email}, nil
}

func parseMoney(total string) (string, int64, error) {
	currency, amount, ok := strings.Cut(total, " ")
	if !ok {
		return "", 0, fmt.Errorf("malformed money %q", total)
	}
	whole, fraction, _ := strings.Cut(amount, ".")
	units, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return "", 0, fmt.Errorf("malformed money %q", total)
	}
	cents := int64(0)
	if fraction != "" {
		for len(fraction) < 2 {
			fraction += "0"
		}
		cents, err = strconv.ParseInt(fraction[:2], 10, 64)
		if err != nil {
			return "", 0, fmt.Errorf("malformed money %q", total)
		}
	}
	if strings.HasPrefix(whole, "-") {
		cents = -cents
	}
	return currency, units*100 + cents, nil
}

func NewLedgerClient(url, token string) *ledgerclient.Client {
	var opts []ledgerclient.Option
	if token != "" {
		opts = append(opts, ledgerclient.WithHeaders(func(context.Context) (http.Header, error) {
			return http.Header{"Authorization": {"Bearer " + token}}, nil
		}))
	}
	return ledgerclient.New(url, opts...)
}

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8081"
	}
	url := os.Getenv("LEDGER_URL")
	if url == "" {
		url = "http://localhost:8080/api"
	}
	mux := http.NewServeMux()
	mux.Handle("/api/", http.StripPrefix("/api", Routes(NewLedgerClient(url, os.Getenv("LEDGER_TOKEN"))).Handler()))
	log.Printf("reports listening on %s, ledger at %s", addr, url)
	log.Fatal(http.ListenAndServe(addr, mux))
}
