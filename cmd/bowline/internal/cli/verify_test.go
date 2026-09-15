package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const consumerFixture = `{"bowline":"1.2","consumer":"ledger-web","interactions":[
  {"procedure":"invoices.get","method":"GET","input":{"id":3},"response":{"status":200,"body":{"id":3,"total":"USD 1.00"}}}
]}`

func TestVerifyConsumers(t *testing.T) {
	dir := ledgerProject(t)
	opts, _, errOut := testOptions(dir)
	if code := VerifyConsumers(opts, nil); code != 1 || !strings.Contains(errOut.String(), "no consumer contracts under contracts/consumers") {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	os.MkdirAll(filepath.Join(dir, "contracts", "consumers"), 0o755)
	os.WriteFile(filepath.Join(dir, "contracts", "consumers", "ledger-web.json"), []byte(consumerFixture), 0o644)
	opts, out, errOut := testOptions(dir)
	if code := VerifyConsumers(opts, nil); code != 0 || !strings.Contains(out.String(), "ok        1 interactions from 1 consumer(s)") {
		t.Fatalf("exit %d: %s %s", code, out.String(), errOut.String())
	}
	broken := strings.Replace(consumerFixture, `"total":"USD 1.00"`, `"amount":"USD 1.00"`, 1)
	os.MkdirAll(filepath.Join(dir, "other"), 0o755)
	os.WriteFile(filepath.Join(dir, "other", "ledger-web.json"), []byte(broken), 0o644)
	opts, _, errOut = testOptions(dir)
	if code := VerifyConsumers(opts, []string{"other"}); code != 1 || !strings.Contains(errOut.String(), "invoices.get → response.amount: field removed") {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	opts, _, _ = testOptions(dir)
	if code := VerifyConsumers(opts, []string{"--help"}); code != 2 {
		t.Fatalf("usage exit %d", code)
	}
}
