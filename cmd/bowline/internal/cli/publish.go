package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bowlinedev/bowline/contract"
	"github.com/bowlinedev/bowline/registry"
)

const publishUsage = "usage: bowline publish --registry URL --service NAME [--tag main] [--ref SHA] [--contract PATH] [--token T]\n       bowline publish --registry URL --consumer NAME --provider SERVICE --usage PATH [--token T]"

func Publish(opts Options, args []string) int {
	flags := flag.NewFlagSet("publish", flag.ContinueOnError)
	flags.SetOutput(opts.Stderr)
	registryURL := flags.String("registry", "", "base URL of the registry")
	service := flags.String("service", "", "service whose contract to publish")
	tag := flags.String("tag", "main", "tag to move to the published version")
	ref := flags.String("ref", "", "source revision the contract was built from")
	contractPath := flags.String("contract", "", "contract document to publish; generated from the module when absent")
	consumer := flags.String("consumer", "", "consumer whose usage to publish")
	provider := flags.String("provider", "", "service the consumer calls")
	usage := flags.String("usage", "", "recorded consumer usage file")
	token := flags.String("token", "", "bearer token; defaults to BOWLINE_REGISTRY_TOKEN")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *registryURL == "" || flags.NArg() > 0 || (*service == "") == (*consumer == "") {
		fmt.Fprintln(opts.Stderr, publishUsage)
		return 2
	}
	if *token == "" {
		if tokens := envTokens(); len(tokens) > 0 {
			*token = tokens[0]
		}
	}
	client := &registry.Client{URL: *registryURL, Token: *token}
	if *consumer != "" {
		return publishConsumer(opts, client, *consumer, *provider, *usage)
	}
	return publishService(opts, client, *service, *tag, *ref, *contractPath)
}

func publishService(opts Options, client *registry.Client, service, tag, ref, contractPath string) int {
	doc, err := serviceDocument(opts, contractPath)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	if doc == nil {
		return 1
	}
	result, err := client.Publish(context.Background(), service, doc, ref, tag)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	state := "already published"
	if result.Created {
		state = "published"
	}
	fmt.Fprintf(opts.Stdout, "%s %s %s", state, service, result.Hash)
	if tag != "" {
		fmt.Fprintf(opts.Stdout, " tagged %s", tag)
	}
	fmt.Fprintln(opts.Stdout)
	if result.Impact != nil && !result.Impact.OK {
		fmt.Fprint(opts.Stderr, registry.FormatText(result.Impact))
	}
	return 0
}

func serviceDocument(opts Options, contractPath string) (*contract.Document, error) {
	if contractPath != "" {
		path := contractPath
		if !filepath.IsAbs(path) {
			path = filepath.Join(opts.Dir, filepath.FromSlash(path))
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		return contract.Parse(data)
	}
	cfg, err := LoadConfig(opts.Dir)
	if err != nil {
		return nil, err
	}
	files, diags, err := Produce(opts)
	if err != nil {
		return nil, err
	}
	if len(diags) > 0 {
		printDiagnostics(opts.Stderr, diags)
		return nil, nil
	}
	return contract.Parse(files[cfg.Contract])
}

func publishConsumer(opts Options, client *registry.Client, consumer, provider, usagePath string) int {
	if provider == "" || usagePath == "" {
		fmt.Fprintln(opts.Stderr, publishUsage)
		return 2
	}
	path := usagePath
	if !filepath.IsAbs(path) {
		path = filepath.Join(opts.Dir, filepath.FromSlash(path))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	if !json.Valid(data) {
		fmt.Fprintf(opts.Stderr, "bowline: %s is not JSON\n", usagePath)
		return 1
	}
	record := registry.Consumer{
		Consumer:   consumer,
		Provider:   provider,
		RecordedAt: time.Now().UTC(),
		Usage:      data,
	}
	if err := client.PublishConsumer(context.Background(), provider, record); err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	fmt.Fprintf(opts.Stdout, "published consumer %s of %s\n", consumer, provider)
	return 0
}
