package organization

type OrganizationRepository interface {
	FindOne(key string) (*Organization, error)
	Create(payload *Organization) (*Organization, error)
	Update(payload *Organization) error
}