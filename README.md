# hashi-corp-vault

Go package for loading secrets from a HashiCorp Vault proxy. It talks to an HTTP proxy that forwards to Vault. Response shape can vary per secret; you specify where in the JSON to read from via a key path. Paths and connection settings are driven by configuration and environment variables.

## Requirements

- **Vault proxy**: A service that accepts `GET {baseURL}/{path}` and returns JSON. The structure can differ per path (e.g. one might be `data.data.data.secrets`, another `data.payload`).
- **Environment**: Per-secret paths are read from env vars you define (e.g. `VAULT_PRIVATE_KEY_PATH`).

## Installation

```bash
go get github.com/billy-ard/hashi-corp-vault
```

## Quick start

1. Set connection env vars (or pass them in `Config`):
   - `VAULT_PROXY_URL` – base URL of the proxy (required)
   - `VAULT_NAMESPACE` – optional; sent as `X-Vault-Namespace`
   - `VAULT_TOKEN` – optional; sent as `X-Vault-Token`

2. For each secret, set an env var to the Vault path (e.g. `VAULT_MY_SECRET_PATH=secret/data/myapp/key`).

3. Build a config and call `LoadSecrets`:

```go
package main

import (
	"fmt"
	"os"

	"github.com/billy-ard/hashi-corp-vault/vault"
)

func main() {
	cfg := &vault.Config{
		ProxyURL:   os.Getenv("VAULT_PROXY_URL"),
		Namespace:  os.Getenv("VAULT_NAMESPACE"),
		Token:      "", // Set to os.Getenv("VAULT_TOKEN") only if token is required
		Vars: map[string]vault.VaultVar{
			"my_secret": {
				Env:   "VAULT_MY_SECRET_PATH",
				Field: "value",
				// Path omitted: defaults to ["data","data","data","secrets"]
			},
		},
	}

	secrets, err := vault.LoadSecrets(cfg)
	if err != nil {
		panic(err)
	}

	val := secrets["my_secret"].(string)
	fmt.Println("loaded:", val)
}
```

## Package API

### Types

- **`vault.Config`** – Connection and list of secrets to load.
  - `ProxyURL` – Base URL of the Vault proxy.
  - `Namespace` – Optional Vault namespace (e.g. for Enterprise).
  - `Token` – Optional Vault token (sent as `X-Vault-Token`).
  - `Vars` – `map[string]VaultVar`: logical name → how to load (env, optional field, optional key path).

- **`vault.VaultVar`** – Describes one secret.
  - `Env` – Environment variable name that contains the Vault path for this secret.
  - `Field` – Key to read from the flattened secret map (e.g. `"value"`). Leave empty to get the full `map[string]string`.
  - `Path` – JSON key path from the response root to the object to flatten (e.g. `[]string{"data","data","data","secrets"}`). If nil or empty, defaults to `data.data.data.secrets`. Each var can use a different path so one can read from an object, another from an object-in-object, etc.

### Functions

- **`LoadSecrets(cfg *Config) (map[string]interface{}, error)`**  
  Fetches all secrets in `cfg.Vars`. For each entry, the path is taken from `os.Getenv(v.Env)`. The response is walked using `v.Path` (or the default), then all scalar leaves (string, number, bool, etc.) are collected into a flat map with dot-separated and `[index]` keys. Returns a map from your chosen names to either a single string (when `Field` is set) or the full flattened `map[string]string`. Errors if config/vars are nil, proxy URL is empty, or any path is missing or fails.

### Response handling

- After following `Path`, the package recursively walks the JSON. Only **objects** and **arrays** are descended into; any **scalar** (string, number, bool, nil) is recorded as a string in the flattened map.
- Nested keys become dot-separated (e.g. `a.b.c`); array elements use `[index]` (e.g. `items[0].id`). So each var can have a different response shape (object, object-in-object, arrays) and still get a consistent flat map of string values.

## Environment variables

| Variable            | Used by        | Description                                  |
|---------------------|----------------|----------------------------------------------|
| `VAULT_PROXY_URL`   | All            | Base URL of the Vault proxy (required).      |
| `VAULT_NAMESPACE`   | All            | Optional Vault namespace (header).           |
| `VAULT_TOKEN`       | All            | Optional Vault token (header).               |
| Per-secret (e.g. `VAULT_MY_SECRET_PATH`) | `LoadSecrets` | You define these; each holds one Vault path. |

## Example: multiple secrets with default path

```go
cfg := &vault.Config{
	ProxyURL:   os.Getenv("VAULT_PROXY_URL"),
	Namespace: os.Getenv("VAULT_NAMESPACE"),
	Token:     os.Getenv("VAULT_TOKEN"),
	Vars: map[string]vault.VaultVar{
		"public_key": {
			Env:   "VAULT_PUBLIC_KEY_PATH",
			Field: "value",
		},
		"private_key": {
			Env:   "VAULT_PRIVATE_KEY_PATH",
			Field: "value",
		},
		"credentials": {
			Env:   "VAULT_CREDENTIAL_PATH",
			Field: "",  // entire flattened map returned
		},
	},
}
secrets, err := vault.LoadSecrets(cfg)
// secrets["public_key"], secrets["private_key"] are strings; secrets["credentials"] is map[string]string
```

## Example: different response shapes per var

```go
cfg := &vault.Config{
	ProxyURL:  os.Getenv("VAULT_PROXY_URL"),
	Namespace: os.Getenv("VAULT_NAMESPACE"),
	Token:     os.Getenv("VAULT_TOKEN"),
	Vars: map[string]vault.VaultVar{
		"legacy_secret": {
			Env:   "VAULT_LEGACY_PATH",
			Field: "value",
			Path:  []string{"data", "data", "data", "secrets"},
		},
		"api_config": {
			Env:   "VAULT_API_CONFIG_PATH",
			Field: "",
			Path:  []string{"data", "payload"},  // different shape
		},
	},
}
secrets, err := vault.LoadSecrets(cfg)
```

## License

Use according to your project's terms.
