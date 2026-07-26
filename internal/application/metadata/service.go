package metadata

import (
	"log"

	"github.com/su3i/wimp/internal/config"
	"github.com/su3i/wimp/internal/domain/metadata"
	"github.com/su3i/wimp/internal/infrastructure/database"
)

func LoadBootstrapToken(bootstrapToken string, cfg *config.DatabaseConfig) error {
	log.Print("Loading bootstrap token..")

	_metadataRepository := database.NewMetadataRepository(cfg)

	_metadata, err := _metadataRepository.FindOne()

	if err != nil {
		return err
	}

	if _metadata != nil {
		if _metadata.BootstrapToken == bootstrapToken {
			return nil
		}
		panic("Do not change bootstrap token!")
	}

	var _newMetadata metadata.Metadata

	_newMetadata.BootstrapToken = bootstrapToken

	_metadataRepository.Create(&_newMetadata)

	return nil
}

func SetLanguage(language string, cfg *config.DatabaseConfig) error {
	_metadataRepository := database.NewMetadataRepository(cfg)

	_metadata, err := _metadataRepository.FindOne()

	if err != nil {
		return err
	}

	if _metadata != nil {
		_metadata.Language = language
	}

	return _metadataRepository.Update(_metadata)
}

func RetrieveLanguage(cfg *config.DatabaseConfig) (*string, error) {
	_metadataRepository := database.NewMetadataRepository(cfg)

	_metadata, err := _metadataRepository.FindOne()

	if err != nil || _metadata == nil {
		return nil, err
	}

	return &_metadata.Language, nil
}
