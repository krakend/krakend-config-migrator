package main

import (
	"context"
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"golang.org/x/sync/errgroup"

	migrator "github.com/devopsfaith/krakend-config-migrator"
)

var rules = [][]string{
	{"github_com/devopsfaith/krakend/transport/http/server/handler", "plugin/http-server"},
	{"github.com/devopsfaith/krakend/transport/http/client/executor", "plugin/http-client"},
	{"github.com/devopsfaith/krakend/proxy/plugin/response", "plugin/response-modifier"},
	{"github.com/devopsfaith/krakend/proxy/plugin/request", "plugin/request-modifier"},
	{"github.com/devopsfaith/krakend/proxy", "proxy"}, // e.g: Shadow proxy
	{"github_com/luraproject/lura/router/gin", "router"},

	{"github.com/devopsfaith/krakend-ratelimit/juju/router", "qos/ratelimit/router"},
	{"github.com/devopsfaith/krakend-ratelimit/juju/proxy", "qos/ratelimit/proxy"},
	{"github.com/devopsfaith/krakend-httpcache", "qos/http-cache"},
	{"github.com/devopsfaith/krakend-circuitbreaker/gobreaker", "qos/circuit-breaker"},

	{"github.com/devopsfaith/krakend-oauth2-clientcredentials", "auth/client-credentials"},
	{"github.com/devopsfaith/krakend-jose/validator", "auth/validator"},
	{"github.com/devopsfaith/krakend-jose/signer", "auth/signer"},
	{"github_com/devopsfaith/bloomfilter", "auth/revoker"},

	{"github_com/devopsfaith/krakend-botdetector", "security/bot-detector"},
	{"github_com/devopsfaith/krakend-httpsecure", "security/http"},
	{"github_com/devopsfaith/krakend-cors", "security/cors"},

	{"github.com/devopsfaith/krakend-cel", "validation/cel"},
	{"github.com/devopsfaith/krakend-jsonschema", "validation/json-schema"},

	{"github.com/devopsfaith/krakend-amqp/consume", "backend/amqp/consumer"},
	{"github.com/devopsfaith/krakend-amqp/produce", "backend/amqp/producer"},
	{"github.com/devopsfaith/krakend-lambda", "backend/lambda"},
	{"github.com/devopsfaith/krakend-pubsub/publisher", "backend/pubsub/publisher"},
	{"github.com/devopsfaith/krakend-pubsub/subscriber", "backend/pubsub/susbcriber"},
	{"github.com/devopsfaith/krakend/transport/http/client/graphql", "backend/graphql"},
	{"github.com/devopsfaith/krakend/http", "backend/http"}, //e.g: detailed_errors

	{"github_com/devopsfaith/krakend-gelf", "telemetry/gelf"},
	{"github_com/devopsfaith/krakend-gologging", "telemetry/logging"},
	{"github_com/devopsfaith/krakend-logstash", "telemetry/logstash"},
	{"github_com/devopsfaith/krakend-metrics", "telemetry/metrics"},
	{"github_com/letgoapp/krakend-influx", "telemetry/influx"},
	{"github_com/devopsfaith/krakend-influx", "telemetry/influx"},
	{"github_com/devopsfaith/krakend-opencensus", "telemetry/opencensus"},

	{"github.com/devopsfaith/krakend-lua/router", "modifier/lua-endpoint"},
	{"github.com/devopsfaith/krakend-lua/proxy", "modifier/lua-proxy"},
	{"github.com/devopsfaith/krakend-lua/proxy/backend", "modifier/lua-backend"},
	{"github.com/devopsfaith/krakend-martian", "modifier/martian"},

	// Enterprise
	{"github_com/devopsfaith/krakend-swagger", "generator/openapi"},
	{"github_com/devopsfaith/krakend-apikeys", "auth/api-keys"},
	{"github_com/devopsfaith/krakend-instana", "telemetry/instana"},
	{"github.com/devopsfaith/krakend-websocket", "websocket"},

	// Unused by CE:
	// github.com/devopsfaith/krakend-ratelimit/rate/router
	// github.com/devopsfaith/krakend-ratelimit/rate/proxy
	// github.com/devopsfaith/krakend-circuitbreaker/eapache

	// Deprecations
	{"whitelist", "allow"},
	{"blacklist", "deny"},
	{"github.com/devopsfaith/krakend-etcd", "---- THE ETCD COMPONENT IS NO LONGER SUPPORTED ----"},
	{"github.com/devopsfaith/krakend-consul", "---- THE BLOOMFILTER-CONSUL INTEGRATION IS NO LONGER SUPPORTED ----"},

	// krakend-jose
	{"propagate-claims", "propagate_claims"},
	{"jwk-url", "jwk_url"},
	{"keys-to-sign", "keys_to_sign"},

	// circuit breaker
	{"maxErrors", "max_errors"},
	{"logStatusChange", "log_status_change"},

	// Botdetector
	{"Denylist", "deny"},
	{"Allowlist", "allow"},
	{"Patterns", "patterns"},
	{"CacheSize", "cache_size"},
}

var defaultConcurrency = runtime.GOMAXPROCS(-1)

func main() {
	patterns := flag.String("p", "*.json,*.tmpl", "patterns to use to contain the file modification")
	mapping := flag.String("m", "", "path to the custom mapping definition")
	concurrency := flag.Int("c", defaultConcurrency, "concurrency level")
	flag.Parse()

	targets := flag.Args()
	if len(targets) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	if *mapping != "" {
		if r, err := parseCfg(*mapping); err == nil {
			rules = r
			log.Println("using customized rules from", *mapping)
		}
	}

	log.Println("ready to scan", targets)

	patternsToUse := strings.Split(*patterns, ",")

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		select {
		case sig := <-sigs:
			log.Println("Signal intercepted:", sig)
			cancel()
		case <-ctx.Done():
		}
	}()

	g, ctx := errgroup.WithContext(ctx)

	for _, target := range targets {
		target := target
		g.Go(func() error {
			provider := migrator.NewProvider(target, *concurrency, patternsToUse...)
			persister := migrator.NewPersister(*concurrency)
			migrator.NewRuleWorker(rules, provider.Out, persister.In, *concurrency)

			persister.Persist()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
	log.Println("work done")
}

func parseCfg(path string) ([][]string, error) {
	rules := [][]string{}

	rawCfg, err := ioutil.ReadFile(path)
	if err != nil {
		return rules, err
	}

	err = json.Unmarshal(rawCfg, &rules)
	return rules, err
}
