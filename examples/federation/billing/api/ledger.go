package api

import (
	"net/http"

	"github.com/bowlinedev/bowline/examples/ledger/ledgerclient"
	"github.com/bowlinedev/bowline/signing"
)

func LedgerClient(url, keyID string, secret []byte) *ledgerclient.Client {
	if keyID == "" || len(secret) == 0 {
		return ledgerclient.New(url)
	}
	return ledgerclient.New(url, ledgerclient.WithHTTPClient(&http.Client{
		Transport: &signing.Transport{KeyID: keyID, Secret: secret},
	}))
}
