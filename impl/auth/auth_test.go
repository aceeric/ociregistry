package auth

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
)

func TestToProvider(t *testing.T) {
	tests := []struct {
		name        string
		providerStr string
		want        provider
		wantErr     bool
	}{
		{
			name:        "valid ecr provider lowercase",
			providerStr: "ecr",
			want:        ecrProvider,
			wantErr:     false,
		},
		{
			name:        "valid ecr provider uppercase",
			providerStr: "ECR",
			want:        ecrProvider,
			wantErr:     false,
		},
		{
			name:        "valid ecr provider mixed case",
			providerStr: "EcR",
			want:        ecrProvider,
			wantErr:     false,
		},
		{
			name:        "unknown provider",
			providerStr: "unknown",
			want:        unknownProvider,
			wantErr:     true,
		},
		{
			name:        "empty provider",
			providerStr: "",
			want:        unknownProvider,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := toProvider(tt.providerStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("toProvider() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("toProvider() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetTokenGetter(t *testing.T) {
	tests := []struct {
		name     string
		provider provider
		wantErr  bool
	}{
		{
			name:     "ecr provider",
			provider: ecrProvider,
			wantErr:  false,
		},
		{
			name:     "unknown provider",
			provider: unknownProvider,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getTokenGetter(tt.provider)
			if (err != nil) != tt.wantErr {
				t.Errorf("getTokenGetter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got == nil {
				t.Errorf("getTokenGetter() returned nil function")
			}
		})
	}
}

func TestIsInitialized(t *testing.T) {
	// Reset tokenProviders before test
	originalProviders := tokenProviders
	defer func() { tokenProviders = originalProviders }()
	tokenProviders = make(map[provider]*tokenProvider)

	tests := []struct {
		name        string
		setup       func()
		providerStr string
		want        bool
		wantErr     bool
	}{
		{
			name: "provider not initialized",
			setup: func() {
				tokenProviders = make(map[provider]*tokenProvider)
			},
			providerStr: "ecr",
			want:        false,
			wantErr:     false,
		},
		{
			name: "provider initialized",
			setup: func() {
				tokenProviders = make(map[provider]*tokenProvider)
				tokenProviders[ecrProvider] = &tokenProvider{}
			},
			providerStr: "ecr",
			want:        true,
			wantErr:     false,
		},
		{
			name: "unknown provider",
			setup: func() {
				tokenProviders = make(map[provider]*tokenProvider)
			},
			providerStr: "unknown",
			want:        false,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			got, err := IsInitialized(tt.providerStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsInitialized() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("IsInitialized() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInit(t *testing.T) {
	// Reset tokenProviders before and after test
	originalProviders := tokenProviders
	defer func() { tokenProviders = originalProviders }()

	tests := []struct {
		name        string
		setup       func()
		providerStr string
		options     string
		expiry      string
		wantErr     bool
		errContains string
	}{
		{
			name: "init with invalid expiry format",
			setup: func() {
				tokenProviders = make(map[provider]*tokenProvider)
			},
			providerStr: "ecr",
			options:     "",
			expiry:      "invalid",
			wantErr:     true,
		},
		{
			name: "init with unknown provider",
			setup: func() {
				tokenProviders = make(map[provider]*tokenProvider)
			},
			providerStr: "unknown",
			options:     "",
			expiry:      "5m",
			wantErr:     true,
			errContains: "unknown provider",
		},
		{
			name: "init already initialized provider",
			setup: func() {
				tokenProviders = make(map[provider]*tokenProvider)
				tokenProviders[ecrProvider] = &tokenProvider{}
			},
			providerStr: "ecr",
			options:     "",
			expiry:      "5m",
			wantErr:     true,
			errContains: "already initialized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()

			err := Init(tt.providerStr, tt.options, tt.expiry)
			if (err != nil) != tt.wantErr {
				t.Errorf("Init() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Init() error = %v, want error containing %q", err, tt.errContains)
				}
			}

			// Verify provider was initialized if no error
			if !tt.wantErr {
				p, _ := toProvider(tt.providerStr)
				if _, ok := tokenProviders[p]; !ok {
					t.Errorf("Init() did not initialize provider")
				}
			}
		})
	}
}

// Note: Testing successful Init() would require either:
// 1. Mocking the AWS SDK (using aws-sdk-go-v2-testing or similar)
// 2. Running integration tests with real AWS credentials
// 3. Refactoring auth.go to allow dependency injection of the token getter

func TestGetToken(t *testing.T) {
	// Reset tokenProviders before test
	originalProviders := tokenProviders
	defer func() { tokenProviders = originalProviders }()

	tests := []struct {
		name        string
		setup       func()
		providerStr string
		want        string
		wantErr     bool
		errContains string
	}{
		{
			name: "get token from initialized provider",
			setup: func() {
				tokenProviders = make(map[provider]*tokenProvider)
				tokenProviders[ecrProvider] = &tokenProvider{
					token: "test-token-123",
				}
			},
			providerStr: "ecr",
			want:        "test-token-123",
			wantErr:     false,
		},
		{
			name: "get token from uninitialized provider",
			setup: func() {
				tokenProviders = make(map[provider]*tokenProvider)
			},
			providerStr: "ecr",
			want:        "",
			wantErr:     true,
			errContains: "not initialized",
		},
		{
			name: "get token with unknown provider",
			setup: func() {
				tokenProviders = make(map[provider]*tokenProvider)
			},
			providerStr: "unknown",
			want:        "",
			wantErr:     true,
			errContains: "unknown provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			got, err := GetToken(tt.providerStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetToken() = %v, want %v", got, tt.want)
			}
			if tt.wantErr && tt.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("GetToken() error = %v, want error containing %q", err, tt.errContains)
				}
			}
		})
	}
}

func TestTokenRefresher(t *testing.T) {
	callCount := 0
	var mu sync.Mutex
	mockGetter := func(opts string) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		callCount++
		return fmt.Sprintf("token-%d", callCount), nil
	}

	tp := &tokenProvider{
		providerStr:  "ecr",
		providerOpts: "",
		getter:       mockGetter,
		lastTokenGet: time.Now(),
		token:        "initial-token",
		expiry:       100 * time.Millisecond,
	}

	// Start the refresher in a goroutine
	done := make(chan bool)
	go func() {
		tokenRefresher(tp)
		done <- true
	}()

	// Wait for at least 2 refreshes
	time.Sleep(250 * time.Millisecond)

	// Check that the token was updated
	tp.RLock()
	token := tp.token
	tp.RUnlock()

	if token == "initial-token" {
		t.Errorf("tokenRefresher() did not update token")
	}

	mu.Lock()
	if callCount < 2 {
		t.Errorf("tokenRefresher() callCount = %d, want >= 2", callCount)
	}
	mu.Unlock()
}

func TestTokenRefresherWithError(t *testing.T) {
	callCount := 0
	var mu sync.Mutex
	mockGetter := func(opts string) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		callCount++
		return "", fmt.Errorf("mock error")
	}

	tp := &tokenProvider{
		providerStr:  "ecr",
		providerOpts: "",
		getter:       mockGetter,
		lastTokenGet: time.Now(),
		token:        "initial-token",
		expiry:       100 * time.Millisecond,
	}

	// Start the refresher in a goroutine
	go tokenRefresher(tp)

	// Wait for at least 2 refresh attempts
	time.Sleep(250 * time.Millisecond)

	// Check that the token was NOT updated (returns early on error)
	tp.RLock()
	token := tp.token
	tp.RUnlock()

	if token != "initial-token" {
		t.Errorf("tokenRefresher() updated token despite error, got %q", token)
	}

	mu.Lock()
	if callCount < 2 {
		t.Errorf("tokenRefresher() callCount = %d, want >= 2", callCount)
	}
	mu.Unlock()
}

func TestTokenProviderConcurrency(t *testing.T) {
	tp := &tokenProvider{
		providerStr:  "ecr",
		providerOpts: "",
		getter:       func(opts string) (string, error) { return "new-token", nil },
		lastTokenGet: time.Now(),
		token:        "initial-token",
		expiry:       1 * time.Second,
	}

	// Simulate concurrent reads and writes
	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Start multiple readers
	for range 50 {
		wg.Go(func() {
			for range 10 {
				tp.RLock()
				_ = tp.token
				tp.RUnlock()
				time.Sleep(time.Millisecond)
			}
		})
	}

	// Start multiple writers
	for i := range 10 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := range 5 {
				tp.Lock()
				tp.token = fmt.Sprintf("token-%d-%d", id, j)
				tp.Unlock()
				time.Sleep(2 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check if any errors occurred
	for err := range errors {
		if err != nil {
			t.Errorf("Concurrent access error: %v", err)
		}
	}
}

func TestParseECROptions(t *testing.T) {
	tests := []struct {
		name        string
		options     string
		wantErr     bool
		errContains string
		validate    func(t *testing.T, opts []func(*config.LoadOptions) error)
	}{
		{
			name:    "empty options",
			options: "",
			wantErr: false,
			validate: func(t *testing.T, opts []func(*config.LoadOptions) error) {
				if len(opts) != 0 {
					t.Errorf("expected 0 options, got %d", len(opts))
				}
			},
		},
		{
			name:    "single profile option",
			options: "profile=myprofile",
			wantErr: false,
			validate: func(t *testing.T, opts []func(*config.LoadOptions) error) {
				if len(opts) != 1 {
					t.Errorf("expected 1 option, got %d", len(opts))
				}
			},
		},
		{
			name:    "single region option",
			options: "region=us-east-1",
			wantErr: false,
			validate: func(t *testing.T, opts []func(*config.LoadOptions) error) {
				if len(opts) != 1 {
					t.Errorf("expected 1 option, got %d", len(opts))
				}
			},
		},
		{
			name:    "multiple options",
			options: "profile=myprofile,region=ap-southeast-2",
			wantErr: false,
			validate: func(t *testing.T, opts []func(*config.LoadOptions) error) {
				if len(opts) != 2 {
					t.Errorf("expected 2 options, got %d", len(opts))
				}
			},
		},
		{
			name:        "invalid format - no equals",
			options:     "profilemyprofile",
			wantErr:     true,
			errContains: "unable to parse configuration option",
		},
		{
			name:        "invalid format - multiple equals",
			options:     "profile=my=profile",
			wantErr:     true,
			errContains: "unable to parse configuration option",
		},
		{
			name:        "unknown option key",
			options:     "unknown=value",
			wantErr:     true,
			errContains: "unable to parse configuration option",
		},
		{
			name:    "case insensitive keys and values",
			options: "PROFILE=MyProfile,REGION=US-EAST-1",
			wantErr: false,
			validate: func(t *testing.T, opts []func(*config.LoadOptions) error) {
				if len(opts) != 2 {
					t.Errorf("expected 2 options, got %d", len(opts))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := parseECROptions(tt.options)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseECROptions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("parseECROptions() error = %v, want error containing %q", err, tt.errContains)
				}
			}
			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, opts)
			}
		})
	}
}
