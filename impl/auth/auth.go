package auth

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	log "github.com/sirupsen/logrus"
)

// provider formalizes the supported providers.
type provider int

// tokenGetter is a function that returns a token and an error.
type tokenGetter func(string) (string, error)

// Defined providers
const (
	unknownProvider provider = iota
	ecrProvider
)

// providers converts a string to the typed provider.
var providers = map[string]provider{
	"ecr": ecrProvider,
}

// providerTostr converts typed provider to string.
var providerTostr = map[provider]string{
	ecrProvider: "ecr",
}

// tokenProvider has all the configuration to support token refresh.
type tokenProvider struct {
	// allows concurrent read/write access to the token value by pullers and the
	// token refresher goroutine.
	sync.RWMutex
	// for error logging.
	providerStr string
	// provider options like foo=bar,bin=baz
	providerOpts string
	// the function that gets the token.
	getter tokenGetter
	// the last time the token was retrieved.
	lastTokenGet time.Time
	// token returned by the token getter function.
	token string
	// token refresh period.
	expiry time.Duration
}

// tokenProviders has every token provider initialized by a call to the
// Init function.
var tokenProviders = make(map[provider]*tokenProvider)

// IsInitialized checks to see if the passed provider has already been initialized.
func IsInitialized(providerStr string) (bool, error) {
	p, err := toProvider(providerStr)
	if err != nil {
		return false, err
	}
	_, ok := tokenProviders[p]
	return ok, nil
}

// Init sets up a provider, gets an initial token value from the provider to verify
// that a token can actually be gotten (to support fail early), and then starts a
// goroutine to refresh the token according to the passed expiration. If expiration is
// empty then 12 hours is the default.
func Init(providerStr string, options string, expiry string) error {
	p, err := toProvider(providerStr)
	if err != nil {
		return err
	}
	_, ok := tokenProviders[p]
	if ok {
		return fmt.Errorf("provider already initialized: %s", providerStr)
	}
	if expiry == "" {
		expiry = "12h"
	}
	parsedExpiry, err := time.ParseDuration(expiry)
	if err != nil {
		return err
	}
	getter, err := getTokenGetter(p)
	if err != nil {
		return err
	}
	// make sure we can actually get the token
	token, err := getter(options)
	if err != nil {
		return err
	}
	// initialize the provider
	tokenProviders[p] = &tokenProvider{
		providerStr:  providerTostr[p],
		providerOpts: options,
		getter:       getter,
		lastTokenGet: time.Now(),
		token:        token,
		expiry:       parsedExpiry,
	}
	go tokenRefresher(tokenProviders[p])
	return nil
}

// GetToken gets the current token value (which is being asynchronously refreshed
// by the token provider.)
func GetToken(providerStr string) (string, error) {
	p, err := toProvider(providerStr)
	if err != nil {
		return "", err
	}
	tp, ok := tokenProviders[p]
	if !ok {
		return "", fmt.Errorf("provider not initialized: %s", providerStr)
	}
	tp.RLock()
	defer tp.RUnlock()
	return tp.token, nil
}

// tokenRefresher is intended to be run as a goroutine. It creates a time ticker
// according to the refresh interval in the passed token provider struct. On each
// tick of the ticker it calls the token getter function in the struct and updates
// the token in the struct from the token getter return value.
func tokenRefresher(tp *tokenProvider) {
	ticker := time.NewTicker(tp.expiry)
	defer ticker.Stop()
	for {
		<-ticker.C
		func() {
			log.Debugf("getting new token for provider %q", tp.providerStr)
			token, err := tp.getter(tp.providerOpts)
			if err != nil {
				log.Errorf("error getting token for provider %q: %s", tp.providerStr, err)
				return
			}
			tp.Lock()
			defer tp.Unlock()
			tp.token = token
			tp.lastTokenGet = time.Now()
		}()
	}
}

// toProvider validates the passed provider string (like "ECR") and returns the matching
// provider type values.
func toProvider(providerStr string) (provider, error) {
	p, ok := providers[strings.ToLower(providerStr)]
	if !ok {
		return unknownProvider, fmt.Errorf("unknown provider: %s", providerStr)
	}
	return p, nil
}

// getTokenGetter gets the token retrieval function for the passed provider.
func getTokenGetter(p provider) (tokenGetter, error) {
	switch p {
	case ecrProvider:
		return getECRToken, nil
	}
	// should never happen
	return nil, fmt.Errorf("unknown provider: %d", p)
}

// getECRToken gets a token for Elastic Container Registry using the AWS SDK.
func getECRToken(options string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	opts, err := parseECROptions(options)
	if err != nil {
		return "", err
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return "", err
	}

	client := ecr.NewFromConfig(cfg)

	result, err := client.GetAuthorizationToken(ctx, &ecr.GetAuthorizationTokenInput{})
	if err != nil {
		return "", err
	}

	if len(result.AuthorizationData) > 0 && result.AuthorizationData[0].AuthorizationToken != nil {
		return *result.AuthorizationData[0].AuthorizationToken, nil
	}
	return "", fmt.Errorf("no authorization token returned")
}

// parseECROptions parses provider options for the ECR provider.
func parseECROptions(options string) ([]func(*config.LoadOptions) error, error) {
	opts := []func(*config.LoadOptions) error{}

	if options == "" {
		return opts, nil
	}

	for opt := range strings.SplitSeq(options, ",") {
		kv := strings.Split(opt, "=")
		if len(kv) != 2 {
			return nil, fmt.Errorf("unable to parse configuration option %s for provider", kv)
		}
		key := strings.ToLower(kv[0])
		val := strings.ToLower(kv[1])
		switch key {
		case "profile":
			opts = append(opts, config.WithSharedConfigProfile(val))
		case "region":
			opts = append(opts, config.WithRegion(val))
		default:
			return nil, fmt.Errorf("unable to parse configuration option %s=%s for provider", key, val)
		}
	}
	return opts, nil
}
