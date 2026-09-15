package ledger

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/bowlinedev/bowline"
)

type Money struct {
	Cents    int64
	Currency string
}

var _ = bowline.WireAs[Money, string]()

func (m Money) String() string {
	sign := ""
	cents := m.Cents
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	return fmt.Sprintf("%s %s%d.%02d", m.Currency, sign, cents/100, cents%100)
}

func (m Money) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.String())
}

func (m *Money) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	currency, amount, ok := strings.Cut(s, " ")
	if !ok || len(currency) != 3 {
		return fmt.Errorf("money %q: want \"CUR 12.34\"", s)
	}
	negative := strings.HasPrefix(amount, "-")
	amount = strings.TrimPrefix(amount, "-")
	whole, frac, ok := strings.Cut(amount, ".")
	if !ok || len(frac) != 2 {
		return fmt.Errorf("money %q: want two decimal places", s)
	}
	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return fmt.Errorf("money %q: %w", s, err)
	}
	f, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		return fmt.Errorf("money %q: %w", s, err)
	}
	cents := w*100 + f
	if negative {
		cents = -cents
	}
	*m = Money{Cents: cents, Currency: currency}
	return nil
}

func (m Money) Add(o Money) Money {
	return Money{Cents: m.Cents + o.Cents, Currency: m.Currency}
}

func (m Money) Times(n int32) Money {
	return Money{Cents: m.Cents * int64(n), Currency: m.Currency}
}
