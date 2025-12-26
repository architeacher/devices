package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/architeacher/devices/services/svc-api-gateway/internal/ports"
	"github.com/hashicorp/vault/api"
	"github.com/kelseyhightower/envconfig"
)

type Loader struct {
	cfg              *ServiceConfig
	secretsRepo      ports.SecretsRepository
	configSignalChan chan os.Signal
	reloadErrors     chan error
	ticker           *time.Ticker
	lastVersion      uint
}

func NewLoader(cfg *ServiceConfig, secretsRepo ports.SecretsRepository, initialVersion uint) *Loader {
	return &Loader{
		cfg:              cfg,
		secretsRepo:      secretsRepo,
		configSignalChan: make(chan os.Signal, 1),
		reloadErrors:     make(chan error, 1),
		lastVersion:      initialVersion,
	}
}

func (l *Loader) WatchConfigSignals(ctx context.Context) <-chan error {
	signal.Notify(l.configSignalChan, syscall.SIGHUP, syscall.SIGUSR1)

	if l.cfg.SecretsStorage.Enabled && l.cfg.SecretsStorage.PollInterval > 0 {
		l.ticker = time.NewTicker(l.cfg.SecretsStorage.PollInterval)
	}

	go func() {
		defer signal.Stop(l.configSignalChan)
		defer close(l.configSignalChan)
		defer close(l.reloadErrors)

		if l.ticker != nil {
			defer l.ticker.Stop()
		}

		var reloadTickerChan <-chan time.Time
		if l.ticker != nil {
			reloadTickerChan = l.ticker.C
		}

		for {
			select {
			case <-ctx.Done():
				return

			case <-reloadTickerChan:
				l.configSignalChan <- syscall.SIGHUP

			case sig := <-l.configSignalChan:
				switch sig {
				case syscall.SIGHUP:
					l.handleConfigReload(ctx)

				case syscall.SIGUSR1:
					l.DumpConfig()
				}
			}
		}
	}()

	return l.reloadErrors
}

func (l *Loader) DumpConfig() {
	configJSON, err := json.MarshalIndent(l.cfg, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stdout, "Error marshaling config: %v\n", err)

		return
	}

	fmt.Fprintf(os.Stdout, "\n=== Configuration Dump ===\n%s\n=== End Configuration ===\n\n", string(configJSON))
}

func (l *Loader) Load(ctx context.Context, secretsRepo ports.SecretsRepository, cfg *ServiceConfig) (uint, error) {
	if !cfg.SecretsStorage.Enabled {
		return 0, fmt.Errorf("secret storage is not enabled")
	}

	if err := l.authenticateVault(ctx, secretsRepo, cfg.SecretsStorage); err != nil {
		return 0, fmt.Errorf("failed to authenticate with Vault: %w", err)
	}

	data, err := l.loadSecretsFromPath(ctx, secretsRepo, cfg, "data")
	if err != nil {
		return 0, fmt.Errorf("failed to load secrets from Vault: %w", err)
	}

	if err := l.applySecretsToConfig(cfg, data); err != nil {
		return 0, fmt.Errorf("failed to apply secrets to config: %w", err)
	}

	metadata, err := l.loadSecretsFromPath(ctx, secretsRepo, cfg, "metadata")
	if err != nil {
		return 0, fmt.Errorf("failed to load secret metadata: %w", err)
	}

	version, err := l.getSecretVersion(metadata)
	if err != nil {
		return 0, fmt.Errorf("failed to get secret version: %w", err)
	}

	return version, nil
}

func Init() (*ServiceConfig, error) {
	cfg := &ServiceConfig{}

	err := envconfig.Process("", cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to parse service configuration: %w", err)
	}

	return cfg, nil
}

func (l *Loader) authenticateVault(ctx context.Context, client ports.SecretsRepository, config SecretsStorage) error {
	switch strings.ToLower(config.AuthMethod) {
	case "token":
		if config.Token == "" {
			return fmt.Errorf("token is required for token auth method")
		}
		client.SetToken(config.Token)

		return nil

	case "approle":
		if config.RoleID == "" || config.SecretID == "" {
			return fmt.Errorf("role_id and secret_id are required for approle auth method")
		}

		data := map[string]any{
			"role_id":   config.RoleID,
			"secret_id": config.SecretID,
		}

		resp, err := client.WriteWithContext(ctx, "auth/approle/login", data)
		if err != nil {
			return fmt.Errorf("failed to authenticate via approle: %w", err)
		}

		if resp.Auth == nil {
			return fmt.Errorf("no auth info returned from Vault")
		}

		client.SetToken(resp.Auth.ClientToken)

		return nil

	default:
		return fmt.Errorf("unsupported auth method: %s", config.AuthMethod)
	}
}

func (l *Loader) handleConfigReload(ctx context.Context) {
	metadata, err := l.loadSecretsFromPath(ctx, l.secretsRepo, l.cfg, "metadata")
	if err != nil {
		l.reportReloadStatus(fmt.Errorf("failed to load secret metadata: %w", err))

		return
	}

	currentVersion, err := l.getSecretVersion(metadata)
	if err != nil {
		l.reportReloadStatus(fmt.Errorf("failed to get secret version: %w", err))

		return
	}

	if currentVersion == l.lastVersion {
		return
	}

	version, err := l.Load(ctx, l.secretsRepo, l.cfg)
	if err != nil {
		l.reportReloadStatus(err)

		return
	}

	l.lastVersion = version
	l.reportReloadStatus(nil)
}

func getSecretsWithRetry(ctx context.Context, secretsRepo ports.SecretsRepository, cfg *ServiceConfig, pathType, mountPath string) (*api.Secret, error) {
	path := fmt.Sprintf("apps/%s/%s", pathType, mountPath)

	ctx, cancel := context.WithTimeout(ctx, cfg.SecretsStorage.Timeout)
	defer cancel()

	var secret *api.Secret
	var err error

	for attempt := uint(0); attempt <= cfg.SecretsStorage.MaxRetries; attempt++ {
		secret, err = secretsRepo.GetSecrets(ctx, path)
		if err == nil {
			break
		}

		if attempt < cfg.SecretsStorage.MaxRetries {
			time.Sleep(time.Duration(attempt+1) * time.Second)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to read from path %s after %d retries: %w", path, cfg.SecretsStorage.MaxRetries, err)
	}

	return secret, nil
}

func (l *Loader) getSecretVersion(metadata map[string]any) (uint, error) {
	if metadata == nil {
		return 0, nil
	}

	currentVersion, ok := metadata["current_version"]
	if !ok {
		return 0, nil
	}

	switch v := currentVersion.(type) {
	case float64:
		return uint(v), nil
	case uint:
		return v, nil
	case json.Number:
		version, err := v.Int64()
		if err != nil {
			return 0, fmt.Errorf("failed to parse version: %w", err)
		}

		return uint(version), nil
	default:
		return 0, fmt.Errorf("unexpected version type: %T", currentVersion)
	}
}

func (l *Loader) loadSecretsFromPath(ctx context.Context, secretsRepo ports.SecretsRepository, cfg *ServiceConfig, pathType string) (map[string]any, error) {
	secret, err := getSecretsWithRetry(ctx, secretsRepo, cfg, "data", cfg.SecretsStorage.MountPath)
	if err != nil {
		return nil, err
	}

	if secret == nil || secret.Data == nil {
		return nil, nil
	}

	result, ok := secret.Data[pathType].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid secret format at path apps/data/%s, missing '%s' key", cfg.SecretsStorage.MountPath, pathType)
	}

	return result, nil
}

func (l *Loader) applySecretsToConfig(cfg *ServiceConfig, data map[string]any) error {
	for key, value := range data {
		if strValue, ok := value.(string); ok && strValue != "" {
			if err := l.applySecretToConfig(cfg, key, strValue); err != nil {
				return fmt.Errorf("failed to apply secrets to config: %w", err)
			}
		}
	}

	return nil
}

func (l *Loader) applySecretToConfig(cfg *ServiceConfig, key, value string) error {
	if err := os.Setenv(key, value); err != nil {
		return fmt.Errorf("failed to set environment variable %s: %w", key, err)
	}

	switch key {
	case "AUTH_SECRET_KEY":
		cfg.Auth.SecretKey = value
	case "AUTH_FALLBACK_KEY_HEX":
		cfg.Auth.FallbackKeyHex = value
	case "DEVICES_GRPC_ADDRESS":
		cfg.DevicesGRPCClient.Address = value
	}

	return nil
}

func (l *Loader) reportReloadStatus(err error) {
	select {
	case l.reloadErrors <- err:
	default:
	}
}
