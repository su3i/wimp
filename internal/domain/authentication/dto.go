package authentication

type LoginDTO struct {
	AccessToken string `json:"AccessToken"`
	RefreshToken string `json:"RefreshToken"`
}