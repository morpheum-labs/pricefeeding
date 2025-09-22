package shared

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

func LoadYamlConf(file_path string) *Configuration {
	var config *Configuration
	ctx := context.Background()
	for {
		config, err := readConfig(file_path)
		if err != nil {
			fmt.Printf("Error: %v. Retrying in 5 seconds...\n", err)
			time.Sleep(5 * time.Second)
			continue
		}

		ctx = context.WithValue(ctx, "config", config)
		fmt.Println("Configuration loaded successfully")
		break
	}

	return config
}

func readConfig(path string) (*Configuration, error) {
	// If the path is a directory, append the file name
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		path = filepath.Join(path, "vault_config.yaml")
	}

	fileContent, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}

	var config Configuration
	if err := yaml.Unmarshal(fileContent, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML file: %w", err)
	}
	// Check if required fields are present
	if config.Port == 0 {
		return nil, fmt.Errorf("port configuration parameter not found")
	}
	if config.SecretHash == "" {
		return nil, fmt.Errorf("secret_hash configuration parameter not found")
	}
	if config.Database.Postgres.DBConn == "" {
		return nil, fmt.Errorf("db_conn configuration parameter not found")
	}
	if config.Database.Postgres.DBConnPool == 0 {
		return nil, fmt.Errorf("db_conn_pool configuration parameter not found")
	}
	if len(config.ArbitrumRPCs.URLs) == 0 {
		return nil, fmt.Errorf("arbitrum_rpcs urls configuration parameter not found")
	}
	if len(config.EthereumRPCs.URLs) == 0 {
		return nil, fmt.Errorf("ethereum_rpcs urls configuration parameter not found")
	}
	return &config, nil
}
