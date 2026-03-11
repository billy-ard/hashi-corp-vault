# microsoft-vault

Go package for loading secrets from a Vault proxy. It talks to an HTTP proxy that forwards to Vault and returns JSON in a fixed shape. Paths and connection settings are driven by configuration and environment variables.

## Requirements

- **Vault proxy**: A service that accepts `GET {baseURL}/{path}` and returns JSON:

  ```json
  { "data": { "data": { "data": { "secrets": { "key": "value", ... } } } } }
  ```

- **Environment**: Per-secret paths are read from env vars you define (e.g. `VAULT_PUBLIC_KEY_PATH`).

## Installation

```bash
go get microsoft-vault
```

In your module, ensure the module path in `go.mod` matches (e.g. `module microsoft-vault` if using a local module).

## Quick start

1. Set connection env vars (or pass them in `Config`):
   - `VAULT_PROXY_URL` – base URL of the proxy (required for `GetSecrets`; required in `Config` for `LoadSecrets`)
   - `VAULT_NAMESPACE` – optional; sent as `X-Vault-Namespace`

2. For each secret, set an env var to the Vault path (e.g. `VAULT_MY_SECRET_PATH=secret/data/myapp/key`).

3. Build a config and call `LoadSecrets`:

```go
package main

import (
	"fmt"
	"os"

	"microsoft-vault"
)

func main() {
	cfg := &vault.Config{
		ProxyURL:  os.Getenv("VAULT_PROXY_URL"),
		Namespace: os.Getenv("VAULT_NAMESPACE"),
		Vars: map[string]vault.VaultVar{
			"my_secret": {
				Env:   "VAULT_MY_SECRET_PATH",  // env var that holds the path
				Field: "value",                  // key inside secrets map; leave "" for full map
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
  - `Vars` – `map[string]VaultVar`: logical name → how to load (env var for path + optional field).

- **`vault.VaultVar`** – Describes one secret.
  - `Env` – Environment variable name that contains the Vault path for this secret.
  - `Field` – Key to read from the secret’s `secrets` map (e.g. `"value"`). Leave empty to get the full `map[string]string`.

### Functions

- **`LoadSecrets(cfg *Config) (map[string]interface{}, error)`**  
  Fetches all secrets in `cfg.Vars`. For each entry, the path is taken from `os.Getenv(v.Env)`. Returns a map from your chosen names (the keys of `cfg.Vars`) to the secret value (string) or full secrets map. Errors if config/vars are nil, proxy URL is empty, or any path is missing or fails.

- **`GetSecrets(path string) (map[string]string, error)`**  
  Fetches the raw `secrets` map for a single path. Uses `VAULT_PROXY_URL` and `VAULT_NAMESPACE` from the environment.

## Environment variables

| Variable            | Used by        | Description                                  |
|---------------------|----------------|----------------------------------------------|
| `VAULT_PROXY_URL`   | All            | Base URL of the Vault proxy (required).     |
| `VAULT_NAMESPACE`   | All            | Optional Vault namespace (header).           |
| Per-secret (e.g. `VAULT_MY_SECRET_PATH`) | `LoadSecrets` | You define these; each holds one Vault path. |

## Example: multiple secrets

```go
cfg := &vault.Config{
	ProxyURL:  os.Getenv("VAULT_PROXY_URL"),
	Namespace: os.Getenv("VAULT_NAMESPACE"),
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
			Field: "",  // entire secrets map returned
		},
	},
}
secrets, err := vault.LoadSecrets(cfg)
// secrets["public_key"], secrets["private_key"] are strings; secrets["credentials"] is map[string]string
```

## Example: single path (env-based)

```go
// Uses VAULT_PROXY_URL and VAULT_NAMESPACE from env
m, err := vault.GetSecrets("secret/data/myapp")
if err != nil {
	return err
}
fmt.Println(m["value"])
```

## License

Use according to your project’s terms.
