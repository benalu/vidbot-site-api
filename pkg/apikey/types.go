package apikey

type Data struct {
	KeyHash   string `json:"key_hash"`
	KeyPrefix string `json:"key_prefix"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Active    bool   `json:"active"`
	Quota     int    `json:"quota"`
	CreatedAt string `json:"created_at"`
}
