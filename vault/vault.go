package vault

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

// VaultVar describes one secret to load from Vault.
// Env is the name of an environment variable that holds the Vault path for this secret.
// Field is the key to extract from the secret's "secrets" map (e.g. "value"); leave empty to get the full map.
type VaultVar struct {
	Env   string
	Field string
}

// Config holds connection settings and the set of secrets to load.
// Used with LoadSecrets to fetch multiple named secrets in one call.
type Config struct {
	ProxyURL  string            // base URL of the Vault proxy (e.g. from VAULT_PROXY_URL)
	Namespace string            // optional Vault namespace (e.g. from VAULT_NAMESPACE)
	Vars      map[string]VaultVar // name -> VaultVar; names become keys in the returned map
}

// vaultResponse matches the nested structure: data.data.data.secrets
type vaultResponse struct {
	Data struct {
		Data struct {
			Data struct {
				Secrets map[string]string `json:"secrets"`
			} `json:"data"`
		} `json:"data"`
	} `json:"data"`
}

// LoadSecrets fetches all secrets in cfg.Vars from the Vault proxy.
// For each entry, the path is read from os.Getenv(v.Env). Results are returned as map[name]value;
// value is either a string (when VaultVar.Field is set) or the full secrets map (when Field is empty).
// Returns an error if cfg or cfg.Vars is nil, ProxyURL is empty, or any path/env is missing or fails to fetch.
func LoadSecrets(cfg *Config) (map[string]interface{}, error) {
	if cfg == nil || cfg.Vars == nil {
		return nil, fmt.Errorf("vault config and vars are required")
	}
	if cfg.ProxyURL == "" {
		return nil, fmt.Errorf("ProxyURL is not set")
	}

	out := make(map[string]interface{}, len(cfg.Vars))

	for name, v := range cfg.Vars {
		path := strings.TrimSpace(os.Getenv(v.Env))
		if path == "" {
			return nil, fmt.Errorf("%s is not set or empty", v.Env)
		}

		secrets, err := fetchFromVault(path, cfg.ProxyURL, cfg.Namespace)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch secrets from vault (%s=%s): %w", v.Env, path, err)
		}

		if v.Field != "" {
			val, ok := secrets[v.Field]
			if !ok {
				return nil, fmt.Errorf("vault value %q not found at path %s (env %s)", v.Field, path, v.Env)
			}
			out[name] = val
		} else {
			out[name] = secrets
		}
	}

	return out, nil
}

// GetSecrets fetches the raw secrets map for a single path.
// Proxy URL and namespace are taken from VAULT_PROXY_URL and VAULT_NAMESPACE.
func GetSecrets(path string) (map[string]string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("vault path cannot be empty")
	}
	return fetchFromVault(path, os.Getenv("VAULT_PROXY_URL"), os.Getenv("VAULT_NAMESPACE"))
}

func fetchFromVault(path, baseURL, namespace string) (map[string]string, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("VAULT_PROXY_URL is not set")
	}

	url := strings.TrimSuffix(baseURL, "/") + "/" + strings.TrimPrefix(path, "/")
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if namespace != "" {
		req.Header.Set("X-Vault-Namespace", namespace)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Failed to fetch secrets from Vault for path %s: %v", path, err)
		return nil, fmt.Errorf("vault request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("Vault returned status %d for path %s", resp.StatusCode, path)
		return nil, fmt.Errorf("vault returned status %d", resp.StatusCode)
	}

	var v vaultResponse
	if err := json.Unmarshal(body, &v); err != nil {
		log.Printf("Invalid Vault response structure: %v", err)
		return nil, fmt.Errorf("invalid vault response: %w", err)
	}

	secrets := v.Data.Data.Data.Secrets
	if secrets == nil {
		return map[string]string{}, nil
	}
	return secrets, nil
}
