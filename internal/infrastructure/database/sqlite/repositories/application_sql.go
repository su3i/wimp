package repositories

import "github.com/su3i/wimp/internal/domain/application"

func (r *applicationRepository) AddSqlInstance(rel *application.ApplicationSqlInstance) error {
	return r.db.Create(rel).Error
}

func (r *applicationRepository) RemoveSqlInstance(applicationID, sqlInstanceID uint) error {
	return r.db.Unscoped().
		Where("application_id = ? AND sql_instance_id = ?", applicationID, sqlInstanceID).
		Delete(&application.ApplicationSqlInstance{}).Error
}

func (r *applicationRepository) FindSqlInstanceRelations(applicationID uint) (*[]application.ApplicationSqlInstance, error) {
	var rels []application.ApplicationSqlInstance
	if err := r.db.Where("application_id = ?", applicationID).Order("created_at ASC").Find(&rels).Error; err != nil {
		return nil, err
	}
	return &rels, nil
}

func (r *applicationRepository) FindSqlCountsByApplicationIDs(appIDs []uint) (map[uint]application.PoolCounts, error) {
	if len(appIDs) == 0 {
		return map[uint]application.PoolCounts{}, nil
	}
	type row struct {
		ApplicationID uint
		Total         int64
		Healthy       int64
	}
	var rows []row
	// A member row is one node's view. For a clustered instance any row of the same instance
	// (same virtual server name and instance name) running on an online host counts, so the
	// instance stays healthy after failing over away from the node it was added from.
	err := r.db.Raw(`
		SELECT asi.application_id,
		       COUNT(si.id) AS total,
		       SUM(CASE WHEN EXISTS (
		             SELECT 1 FROM sql_instances s2
		             JOIN machines m2 ON m2.id = s2.machine_id AND m2.deleted_at IS NULL
		             WHERE s2.deleted_at IS NULL AND s2.state = 'Running' AND m2.status = 'online'
		               AND (s2.id = si.id OR (si.clustered AND s2.clustered
		                    AND s2.virtual_server_name = si.virtual_server_name AND s2.name = si.name))
		           ) THEN 1 ELSE 0 END) AS healthy
		FROM application_sql_instances asi
		JOIN sql_instances si ON si.id = asi.sql_instance_id AND si.deleted_at IS NULL
		WHERE asi.application_id IN ? AND asi.deleted_at IS NULL
		GROUP BY asi.application_id
	`, appIDs).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[uint]application.PoolCounts, len(rows))
	for _, r := range rows {
		out[r.ApplicationID] = application.PoolCounts{Total: r.Total, Healthy: r.Healthy}
	}
	return out, nil
}
