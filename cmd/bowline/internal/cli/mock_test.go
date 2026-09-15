package cli

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMockServesTheLedgerContract(t *testing.T) {
	dir := ledgerProject(t)
	opts, out, errOut := testOptions(dir)
	stop := make(chan struct{})
	ready := make(chan string, 1)
	opts.Stop = stop
	done := make(chan int, 1)
	go func() { done <- Mock(MockOptions{Options: opts, Addr: "127.0.0.1:0", Seed: 1, Ready: ready}) }()
	addr := <-ready
	resp, err := http.Get("http://" + addr + "/api/invoices.get?input=%7B%22id%22%3A3%7D")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(body), `"id":3`) {
		t.Fatalf("status %d body %s", resp.StatusCode, body)
	}
	resp, _ = http.Get("http://" + addr + "/invoices.get?input=%7B%22id%22%3A3%7D")
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("root mount status %d", resp.StatusCode)
	}
	close(stop)
	if code := <-done; code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "procedures at http://"+addr+"/api (generated data)") {
		t.Fatalf("banner %q", out.String())
	}
}

func TestMockFlags(t *testing.T) {
	dir := ledgerProject(t)
	m, err := parseMockFlags([]string{"--addr", ":0", "--seed", "9", "--replay", "--strict", "--no-playground"}, io.Discard)
	if err != nil || m.Addr != ":0" || m.Seed != 9 || !m.Replay || !m.Strict || m.Playground {
		t.Fatalf("%v %+v", err, m)
	}
	for _, args := range [][]string{{"--record", "http://x", "--replay"}, {"--strict"}, {"--bogus"}} {
		if _, err := parseMockFlags(args, io.Discard); err == nil {
			t.Fatalf("%v: expected an error", args)
		}
	}
	opts, _, errOut := testOptions(dir)
	if code := MockCommand(opts, []string{"--replay", "--fixtures", "nowhere"}); code != 1 || !strings.Contains(errOut.String(), "nowhere") {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	opts, _, errOut = testOptions(dir)
	if code := MockCommand(opts, []string{"--record", "not a url"}); code != 2 || !strings.Contains(errOut.String(), "absolute URL") {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	os.Remove(filepath.Join(dir, "bowline.contract.json"))
	opts, _, errOut = testOptions(dir)
	if code := MockCommand(opts, nil); code != 1 || !strings.Contains(errOut.String(), "run bowline gen first") {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
}
