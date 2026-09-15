package api

import (
	"log/slog"
	"testing"
	"time"

	"github.com/bowlinedev/bowline/contracttest"
	"github.com/bowlinedev/bowline/examples/ledger/ledger"
)

func TestConsumers(t *testing.T) {
	contracttest.VerifyConsumers(t, New(ledger.NewStore(time.Now), slog.Default(), "").Router(), "../contracts/consumers",
		contracttest.WithContract(Contract),
		contracttest.WithSetup(func(testing.TB) {}),
	)
}
