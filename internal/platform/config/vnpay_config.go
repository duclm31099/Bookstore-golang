package config

type VNPayConfig struct {
	TMNCode    string
	HashSecret string
	PaymentURL string
	ReturnURL  string
	NotifyURL  string
}

func loadVNPayConfig() VNPayConfig {
	return VNPayConfig{
		TMNCode:    mustGetEnv("VNPAY_TMN_CODE"),
		HashSecret: mustGetEnv("VNPAY_HASH_SECRET"),
		PaymentURL: mustGetEnv("VNPAY_PAYMENT_URL"),
		ReturnURL:  mustGetEnv("VNPAY_RETURN_URL"),
		NotifyURL:  mustGetEnv("VNPAY_NOTIFY_URL"),
	}
}
