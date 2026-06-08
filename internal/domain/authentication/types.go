package authentication

import "time"

type JWTParams struct {
	Subject   uint
	Email     string
	Roles     []string
	Issuer    string
	Audience  string
	TTL       time.Duration
	SecretKey []byte
}