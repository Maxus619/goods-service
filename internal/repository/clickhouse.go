package repository

import (
	"context"
	"github.com/ClickHouse/clickhouse-go/v2"
	"goods-service/internal/models"
	"time"
)

type ClickhouseRepository struct {
	conn clickhouse.Conn
}

func NewClickhouseRepository(conn clickhouse.Conn) *ClickhouseRepository {
	return &ClickhouseRepository{conn: conn}
}

func (r *ClickhouseRepository) LogGoodEvent(ctx context.Context, good *models.Good) error {
	query := `
        INSERT INTO goods_log (
            Id, ProjectId, Name, Description, Priority, Removed, EventTime
        ) VALUES (?, ?, ?, ?, ?, ?, ?)`

	return r.conn.Exec(ctx, query,
		good.ID,
		good.ProjectID,
		good.Name,
		good.Description,
		good.Priority,
		good.Removed,
		time.Now(),
	)
}
