package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	projectService "github.com/su3i/wimp/internal/application/project"
	"github.com/su3i/wimp/internal/config"
	"github.com/su3i/wimp/internal/domain/apppool"
	"github.com/su3i/wimp/internal/domain/application"
	"github.com/su3i/wimp/internal/domain/machine"
	"github.com/su3i/wimp/internal/domain/protocol"
	"github.com/su3i/wimp/internal/domain/site"
	"github.com/su3i/wimp/internal/hub"
	"github.com/su3i/wimp/internal/infrastructure/database"
)

type AppPoolWithDetails struct {
	apppool.AppPool
	Machine machine.Machine `json:"machine"`
	Sites   []site.Site     `json:"sites"`
	LogPath *string         `json:"log_path"`
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
			LogPath: rel.LogPath,
		})
	}

	return &ApplicationDetail{
		Application: *app,
		AppPools:    poolDetails,
	}, nil
}

func AddAppPools(applicationID, machineID uint, appPoolIDs []uint, projectKey string, cfg *config.DatabaseConfig) error {
	proj, err := projectService.RetrieveProject(projectKey, cfg)
	if err != nil || proj == nil {
		return errors.New("project not found")
	}

	appRepo := database.NewApplicationRepository(cfg)
	app, err := appRepo.FindOneByID(applicationID)
	if err != nil || app == nil || app.ProjectID != proj.ID {
		return errors.New("application not found")
	}

	// Validate all incoming pools belong to the specified machine
	appPoolRepo := database.NewAppPoolRepository(cfg)
	incoming := make(map[uint]bool, len(appPoolIDs))
	for _, id := range appPoolIDs {
		pool, err := appPoolRepo.FindOneByID(id)
		if err != nil || pool == nil {
			return errors.New("app pool not found")
		}
		if pool.MachineID != machineID {
			return errors.New("app pool does not belong to the specified machine")
		}
		incoming[id] = true
	}

	existing, err := appRepo.FindAppPoolRelations(applicationID)
	if err != nil {
		return err
	}

	// Remove pools not in the incoming list
	for _, rel := range *existing {
		if !incoming[rel.AppPoolID] {
			if err := appRepo.RemoveAppPool(applicationID, rel.AppPoolID); err != nil {
				return err
			}
		}
	}

	// Add pools not already linked
	existingSet := make(map[uint]bool, len(*existing))
	for _, rel := range *existing {
		existingSet[rel.AppPoolID] = true
	}
	for id := range incoming {
		if !existingSet[id] {
			if err := appRepo.AddAppPool(&application.ApplicationAppPool{
				ApplicationID: applicationID,
				AppPoolID:     id,
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

func UpdateAppPoolLogPath(applicationID, appPoolID uint, logPath string, projectKey string, cfg *config.DatabaseConfig) error {
	proj, err := projectService.RetrieveProject(projectKey, cfg)
	if err != nil || proj == nil {
		return errors.New("project not found")
	}

	appRepo := database.NewApplicationRepository(cfg)
	app, err := appRepo.FindOneByID(applicationID)
	if err != nil || app == nil || app.ProjectID != proj.ID {
		return errors.New("application not found")
	}

	rel, err := appRepo.FindAppPoolRelation(applicationID, appPoolID)
	if err != nil || rel == nil {
		return errors.New("app pool not assigned to this application")
	}

	currentPath := ""
	if rel.LogPath != nil {
		currentPath = *rel.LogPath
	}
	if currentPath == logPath {
		return nil
	}

	rel.LogPath = &logPath
	if err := appRepo.UpdateAppPoolRelation(rel); err != nil {
		return err
	}

	pool, err := database.NewAppPoolRepository(cfg).FindOneByID(appPoolID)
	if err != nil || pool == nil {
		return errors.New("app pool not found")
	}

	_ = PushFluentConfig(pool.MachineID, cfg) // best-effort
	return nil
}

func ListApplicationFiles(ctx context.Context, applicationID uint, projectKey string, cfg *config.DatabaseConfig) ([]string, error) {
	detail, err := GetDetail(applicationID, projectKey, cfg)
	if err != nil {
		return nil, err
	}

	type pair struct {
		machineID uint
		logPath   string
	}
	seen := map[string]bool{}
	var pairs []pair
	for _, pool := range detail.AppPools {
		if pool.LogPath == nil || *pool.LogPath == "" {
			continue
		}
		key := fmt.Sprintf("%d:%s", pool.Machine.ID, *pool.LogPath)
		if !seen[key] {
			seen[key] = true
			pairs = append(pairs, pair{pool.Machine.ID, *pool.LogPath})
		}
	}

	if len(pairs) == 0 {
		return []string{}, nil
	}

	type result struct{ files []string }
	results := make(chan result, len(pairs))
	for _, p := range pairs {
		go func(machineID uint, path string) {
			results <- result{queryMachineFiles(ctx, machineID, path)}
		}(p.machineID, p.logPath)
	}

	fileSet := map[string]bool{}
	for range pairs {
		r := <-results
		for _, f := range r.files {
			fileSet[f] = true
		}
	}

	all := make([]string, 0, len(fileSet))
	for f := range fileSet {
		all = append(all, f)
	}
	sort.Strings(all)
	return all, nil
}

func queryMachineFiles(ctx context.Context, machineID uint, path string) []string {
	if !hub.Get().IsOnline(machineID) {
		return nil
	}

	reqID := uuid.New().String()
	ch := hub.RegisterCommand(reqID)
	defer hub.DeregisterCommand(reqID)

	payload, _ := json.Marshal(protocol.ListFilesPayload{RequestID: reqID, Path: path})
	msg, _ := json.Marshal(protocol.Message{Type: protocol.TypeListFiles, Payload: json.RawMessage(payload)})

	if err := hub.Get().Send(machineID, msg); err != nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	select {
	case res := <-ch:
		if !res.Success {
			return nil
		}
		var files []string
		json.Unmarshal([]byte(res.Output), &files) //nolint:errcheck
		return files
	case <-ctx.Done():
		return nil
	}
}

// BuildFluentConfig assembles the full fluent-bit config payload for a machine,
// covering all app pool log paths currently configured.
func BuildFluentConfig(machineID uint, cfg *config.DatabaseConfig) (*protocol.FluentConfigPayload, error) {
	common := config.Common()
	pools, err := database.NewAppPoolRepository(cfg).FindByMachineID(machineID)
	if err != nil {
		return nil, err
	}

	payload := &protocol.FluentConfigPayload{
		MachineID:         machineID,
		LokiHost:          common.LokiHost,
		LokiPort:          common.LokiPort,
		LokiTlsEnabled:    common.LokiTlsEnabled,
		LokiTlsSkipVerify: common.LokiTlsSkipVerify,
		Configs:           []protocol.FluentAppConfig{},
	}

	if len(*pools) == 0 {
		return payload, nil
	}

	poolIDs := make([]uint, 0, len(*pools))
	for _, p := range *pools {
		poolIDs = append(poolIDs, p.ID)
	}

	relations, err := database.NewApplicationRepository(cfg).FindAppPoolRelationsByPoolIDs(poolIDs)
	if err != nil {
		return nil, err
	}

	for _, rel := range *relations {
		if rel.LogPath != nil && *rel.LogPath != "" {
			payload.Configs = append(payload.Configs, protocol.FluentAppConfig{
				ApplicationID: rel.ApplicationID,
				PoolID:        rel.AppPoolID,
				LogPath:       *rel.LogPath,
			})
		}
	}

	return payload, nil
}

// PushFluentConfig sends the current fluent-bit config for a machine over the WebSocket.
// Silently no-ops if the machine is offline — it will receive the config on reconnect.
func PushFluentConfig(machineID uint, cfg *config.DatabaseConfig) error {
	payload, err := BuildFluentConfig(machineID, cfg)
	if err != nil {
		return err
	}

	payloadData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	msgData, err := json.Marshal(protocol.Message{
		Type:    protocol.TypeFluentConfig,
		Payload: json.RawMessage(payloadData),
	})
	if err != nil {
		return err
	}

	_ = hub.Get().Send(machineID, msgData)
	return nil
}
