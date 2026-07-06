package authentication

import "time"

type JWTParams struct {
	Subject   uint
	Username  string
	Roles     []string
	Issuer    string
	Audience  string
	TTL       time.Duration
	SecretKey []byte
}