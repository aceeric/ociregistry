package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
)

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
