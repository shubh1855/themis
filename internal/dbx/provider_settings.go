package dbx

import (
	"context"
	"encoding/json"
	"fmt"
)

const providerConfigKey = "llm_provider_config"

type ProviderConfigRow struct {
	Provider string `json:"provider"`
	APIKey   string `json:"api_key"`
	BaseURL  string `json:"base_url"`
	Model    string `json:"model"`
}

func (d *DB) SaveProviderConfig(ctx context.Context, cfg ProviderConfigRow) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("dbx: marshal provider config: %w", err)
	}
	if err := d.SetSetting(ctx, providerConfigKey, string(data)); err != nil {
		return fmt.Errorf("dbx: save provider config: %w", err)
	}
	return nil
}

func (d *DB) LoadProviderConfig(ctx context.Context) (ProviderConfigRow, bool, error) {
	value, ok, err := d.GetSetting(ctx, providerConfigKey)
	if err != nil {
		return ProviderConfigRow{}, false, err
	}
	if !ok {
		return ProviderConfigRow{}, false, nil
	}
	var cfg ProviderConfigRow
	if err := json.Unmarshal([]byte(value), &cfg); err != nil {
		return ProviderConfigRow{}, false, fmt.Errorf("dbx: unmarshal provider config: %w", err)
	}
	return cfg, true, nil
}
