package shared

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
