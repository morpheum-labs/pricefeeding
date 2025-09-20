package shared

import (
	"github.com/google/uuid"
)

// Configuration struct to hold the configuration parameters
type Configuration struct {
	Port       int    `yaml:"port"`
	SecretHash string `yaml:"secret_hash"`
	Database   struct {
		Postgres struct {
			DBConn     string `yaml:"db_conn"`
			DBConnPool int    `yaml:"db_conn_pool"`
		} `yaml:"postgres"`
	} `yaml:"database"`
	ArbitrumRPCs struct {
		URLs []string `yaml:"urls"`
	} `yaml:"arbitrum_rpcs"`
	EthereumRPCs struct {
		URLs []string `yaml:"urls"`
	} `yaml:"ethereum_rpcs"`
}
type DepositRequest struct {
	DepositChainID string `json:"deposit_chain_id"`
	AmountCode     string `json:"amount_code"`
	DisplayAmount  string `json:"display_amount"`
	Nonce          string `json:"nonce"`
	Signature      string `json:"signature"`
	WalletAddress  string
	RequestID      string
	UserID         string
}

type WithdrawRequest struct {
	DestinationChainID string `json:"destination_chain_id"`
	AmountCode         string `json:"amount_code"`
	DisplayAmount      string `json:"display_amount"`
	Nonce              string `json:"nonce"`
	Signature          string `json:"signature"`
	WalletAddress      string
	RequestID          uuid.UUID
	UserID             uuid.UUID
}
