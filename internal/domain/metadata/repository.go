package metadata

type MetadataRepository interface {
	FindOne() (*Metadata, error)
	Create(payload *Metadata) (*Metadata, error)
	Update(payload *Metadata) error
}