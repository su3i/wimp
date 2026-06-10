package application

import (
	"errors"

	projectService "github.com/su3i/wimp/internal/application/project"
	"github.com/su3i/wimp/internal/config"
	"github.com/su3i/wimp/internal/domain/apppool"
	"github.com/su3i/wimp/internal/domain/application"
	"github.com/su3i/wimp/internal/domain/machine"
	"github.com/su3i/wimp/internal/domain/site"
	"github.com/su3i/wimp/internal/infrastructure/database"
)

type AppPoolWithDetails struct {
	apppool.AppPool
	Machine machine.Machine `json:"machine"`
	Sites   []site.Site     `json:"sites"`
}

type ApplicationDetail struct {
	application.Application
	AppPools []AppPoolWithDetails `json:"app_pools"`
}

func Create(name, projectKey string, cfg *config.DatabaseConfig) (*application.Application, error) {
	proj, err := projectService.RetrieveProject(projectKey, cfg)
	if err != nil || proj == nil {
		return nil, errors.New("project not found")
	}

	app := &application.Application{
		ProjectID: proj.ID,
		Name:      name,
	}
	return database.NewApplicationRepository(cfg).Create(app)
}

func RetrieveAll(projectKey string, cfg *config.DatabaseConfig) (*[]application.Application, error) {
	proj, err := projectService.RetrieveProject(projectKey, cfg)
	if err != nil || proj == nil {
		return nil, errors.New("project not found")
	}
	return database.NewApplicationRepository(cfg).FindByProjectID(proj.ID)
}

func GetDetail(id uint, projectKey string, cfg *config.DatabaseConfig) (*ApplicationDetail, error) {
	proj, err := projectService.RetrieveProject(projectKey, cfg)
	if err != nil || proj == nil {
		return nil, errors.New("project not found")
	}

	appRepo := database.NewApplicationRepository(cfg)
	app, err := appRepo.FindOneByID(id)
	if err != nil || app == nil || app.ProjectID != proj.ID {
		return nil, errors.New("application not found")
	}

	relations, err := appRepo.FindAppPoolRelations(id)
	if err != nil {
		return nil, err
	}

	appPoolRepo := database.NewAppPoolRepository(cfg)
	machineRepo := database.NewMachineRepository(cfg)
	siteRepo := database.NewSiteRepository(cfg)

	poolDetails := make([]AppPoolWithDetails, 0, len(*relations))
	for _, rel := range *relations {
		pool, err := appPoolRepo.FindOneByID(rel.AppPoolID)
		if err != nil || pool == nil {
			continue
		}
		m, err := machineRepo.FindOneByID(pool.MachineID)
		if err != nil || m == nil {
			continue
		}
		sites, err := siteRepo.FindByMachineAndAppPool(pool.MachineID, pool.Name)
		if err != nil || sites == nil {
			sites = &[]site.Site{}
		}
		poolDetails = append(poolDetails, AppPoolWithDetails{
			AppPool: *pool,
			Machine: *m,
			Sites:   *sites,
		})
	}

	return &ApplicationDetail{
		Application: *app,
		AppPools:    poolDetails,
	}, nil
}

func AddAppPool(applicationID, machineID, appPoolID uint, projectKey string, cfg *config.DatabaseConfig) error {
	proj, err := projectService.RetrieveProject(projectKey, cfg)
	if err != nil || proj == nil {
		return errors.New("project not found")
	}

	appRepo := database.NewApplicationRepository(cfg)
	app, err := appRepo.FindOneByID(applicationID)
	if err != nil || app == nil || app.ProjectID != proj.ID {
		return errors.New("application not found")
	}

	pool, err := database.NewAppPoolRepository(cfg).FindOneByID(appPoolID)
	if err != nil || pool == nil {
		return errors.New("app pool not found")
	}
	if pool.MachineID != machineID {
		return errors.New("app pool does not belong to the specified machine")
	}

	exists, err := appRepo.HasAppPool(applicationID, appPoolID)
	if err != nil {
		return err
	}
	if exists {
		return errors.New("app pool already added to this application")
	}

	return appRepo.AddAppPool(&application.ApplicationAppPool{
		ApplicationID: applicationID,
		AppPoolID:     appPoolID,
	})
}
