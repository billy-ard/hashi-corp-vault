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
	Path  []string
}

// Config holds connection settings and the set of secrets to load.
// Used with LoadSecrets to fetch multiple named secrets in one call.
type Config struct {
	ProxyURL  string            // base URL of the Vault proxy (e.g. from VAULT_PROXY_URL)
	Namespace string            // optional Vault namespace (e.g. from VAULT_NAMESPACE)
	Token string            // optional Vault token (e.g. from VAULT_NAMESPACE)
	Vars      map[string]VaultVar // name -> VaultVar; names become keys in the returned map
}

// defaultKeyPath is the path used when VaultVar.Path is nil or empty.
var defaultKeyPath = []string{"data", "data", "data", "secrets"}

// LoadSecrets fetches all secrets in cfg.Vars from the Vault proxy.
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

		keyPath := v.Path
		if len(keyPath) == 0 {
			keyPath = defaultKeyPath
		}
		secrets, err := fetchFromVault(path, cfg.ProxyURL, cfg.Namespace, cfg.Token, keyPath)

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

func fetchFromVault(path, baseURL, namespace, token string, keyPath []string) (map[string]string, error) {
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
	if token != "" {
		req.Header.Set("X-Vault-Token", token)
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

	var raw interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("invalid vault response: %w", err)
	}

	current := raw
	for _, key := range keyPath {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("vault response: expected object before %q", key)
		}
		v, ok := m[key]
		if !ok {
			return nil, fmt.Errorf("vault response: key %q not found", key)
		}
		current = v
	}

	// Recursively walk until we find string leaves and flatten them into a map.
	out := make(map[string]string)
	if err := collectStringLeaves("", current, out); err != nil {
		return nil, err
	}
	return out, nil
}

// collectStringLeaves walks an arbitrary decoded JSON value and collects all
// leaf values (scalars: string, number, bool, etc.) into out as strings, using
// dot-separated keys for nested objects and [index] notation for arrays.
// Only objects and arrays are recursed into; any other type is converted to string.
func collectStringLeaves(prefix string, v interface{}, out map[string]string) error {
	switch t := v.(type) {
	case map[string]interface{}:
		for k, child := range t {
			next := k
			if prefix != "" {
				next = prefix + "." + k
			}
			if err := collectStringLeaves(next, child, out); err != nil {
				return err
			}
		}
	case []interface{}:
		for i, child := range t {
			idxKey := fmt.Sprintf("%s[%d]", prefix, i)
			if prefix == "" {
				idxKey = fmt.Sprintf("[%d]", i)
			}
			if err := collectStringLeaves(idxKey, child, out); err != nil {
				return err
			}
		}
	default:
		// Any scalar (string, int, float, bool, nil) — record as string.
		if prefix != "" {
			out[prefix] = fmt.Sprint(t)
		}
	}
	return nil
}
