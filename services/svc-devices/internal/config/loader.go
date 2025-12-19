package config

import (
	"fmt"

	"github.com/kelseyhightower/envconfig"
)

func Init() (*ServiceConfig, error) {
	cfg := &ServiceConfig{}

	err := envconfig.Process("", cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to parse service configuration: %w", err)
	}

	if len(ServiceVersion) != 0 {
		cfg.App.ServiceVersion = ServiceVersion
	}

	if len(CommitSHA) != 0 {
		cfg.App.CommitSHA = CommitSHA
	}

	if len(APIVersion) != 0 {
		cfg.App.APIVersion = APIVersion
	}

	return cfg, nil
}
