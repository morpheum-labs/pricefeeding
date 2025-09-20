package shared

const (
	INTERNAL_NETWORK_TCP_PORT = 9888
	INTERNAL_NETWORK_WS_PORT  = 9801
	END_IP_INT                = 99
	START_IP_INT              = 10
)

// secretKey should be securely loaded from config or environment in production.
var ONCHAIN_MODULE_KEY = []byte("your-very-secret-key")

const (
	DEFAULT_TEST_CONN_POSTGRES = "postgresql://hkg_kl:123123@hub.msbit.cn:29091/tstdex?sslmode=disable"
	DEFAULT_DEX_CONN_POSTGRES  = "postgresql://hkg_kl:123123@hub.msbit.cn:29091/dex?sslmode=disable"
)
