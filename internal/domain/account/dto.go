package account

type AccountDTO struct {
	ID         uint       `json:"ID"`
	Name       string     `json:"Name"`
	Username   string     `json:"Username"`
	Role       AccountRole `json:"Role"`
	MFAEnabled bool       `json:"MFAEnabled"`
	CreatedAt  string	  `json:"CreatedAt"`
	UpdatedAt  string	  `json:"UpdatedAt"`
}

func ToAccountDTO(acc *Account) *AccountDTO {
	return &AccountDTO{
		ID: acc.ID,
		Name: acc.Name,
		Username: acc.Username,
		Role: acc.Role,
		MFAEnabled: acc.MFAEnabled,
		CreatedAt: acc.CreatedAt.String(),
		UpdatedAt: acc.UpdatedAt.String(),
	}
}

func ToAccountDTOs(accounts *[]Account) *[]AccountDTO {
	if accounts == nil {
		return &[]AccountDTO{}
	}

	dtos := make([]AccountDTO, len(*accounts))
	for i, acc := range *accounts {
		dtos[i] = AccountDTO{
			ID:         acc.ID,
			Name:       acc.Name,
			Username:   acc.Username,
			Role:       acc.Role,
			MFAEnabled: acc.MFAEnabled,
			CreatedAt: acc.CreatedAt.String(),
			UpdatedAt: acc.UpdatedAt.String(),
		}
	}
	return &dtos
}