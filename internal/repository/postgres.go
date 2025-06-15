package repository

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"goods-service/internal/models"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) CreateGood(ctx context.Context, good *models.Good) error {
	query := `
        INSERT INTO goods (project_id, name, description, priority)
        VALUES ($1, $2, $3, (
            SELECT COALESCE(MAX(priority), 0) + 1 
            FROM goods 
            WHERE project_id = $1 AND NOT removed
        ))
        RETURNING id, priority, created_at`

	return r.pool.QueryRow(ctx, query,
		good.ProjectID,
		good.Name,
		good.Description,
	).Scan(&good.ID, &good.Priority, &good.CreatedAt)
}

func (r *PostgresRepository) UpdateGood(ctx context.Context, good *models.Good) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	query := `
        UPDATE goods 
        SET name = $1, description = $2
        WHERE id = $3 AND project_id = $4
        RETURNING name, description, priority, created_at`

	err = tx.QueryRow(ctx, query,
		good.Name,
		good.Description,
		good.ID,
		good.ProjectID,
	).Scan(&good.Name, &good.Description, &good.Priority, &good.CreatedAt)
	if err != nil {
		return err
	}

	if err = tx.Commit(ctx); err != nil {
		return err
	}

	return nil
}

func (r *PostgresRepository) ReprioritizeGoods(ctx context.Context, id, projectID, newPriority int) ([]models.Good, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Проверяем существование записи
	exists, err := r.CheckGoodExists(ctx, id, projectID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, models.ErrNotFound
	}

	// Обновляем приоритеты у всех записей с большим или равным приоритетом
	_, err = tx.Exec(ctx, `
		UPDATE goods
		SET priority = priority + 1
		WHERE project_id = $1
		AND priority >= $2
		AND id != $3
		AND NOT removed`,
		projectID, newPriority, id)
	if err != nil {
		return nil, err
	}

	// Устанавливаем новый приоритет целевой записи
	_, err = tx.Exec(ctx, `
		UPDATE goods
		SET priority = $1
		WHERE id = $2 AND project_id = $3`,
		newPriority, id, projectID)
	if err != nil {
		return nil, err
	}

	// Обновляем приоритет у остальных записей
	_, err = tx.Exec(ctx, `
		UPDATE goods
		SET priority = priority + 1
		WHERE id != $1
		AND project_id != $2
		AND priority >= $3
		AND NOT removed`,
		id, projectID, newPriority)
	if err != nil {
		return nil, err
	}

	// Получаем все обновлённые записи
	rows, err := tx.Query(ctx, `
		SELECT id, project_id, name, description, priority, removed
		FROM goods
		WHERE priority >= $1
		AND NOT removed
		ORDER BY priority`,
		newPriority)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var updatedPriorities []models.Good
	for rows.Next() {
		var item models.Good
		err := rows.Scan(
			&item.ID,
			&item.ProjectID,
			&item.Name,
			&item.Description,
			&item.Priority,
			&item.Removed)
		if err != nil {
			return nil, err
		}
		updatedPriorities = append(updatedPriorities, item)
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}

	return updatedPriorities, nil
}

func (r *PostgresRepository) ListGoods(ctx context.Context, limit, offset int) ([]models.Good, error) {
	if limit == 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	query := `
        SELECT id, project_id, name, description, priority, removed, created_at
        FROM goods
        WHERE removed = false
        LIMIT $1 OFFSET $2`

	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var goods []models.Good
	for rows.Next() {
		var good models.Good
		err := rows.Scan(
			&good.ID,
			&good.ProjectID,
			&good.Name,
			&good.Description,
			&good.Priority,
			&good.Removed,
			&good.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		goods = append(goods, good)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return goods, nil
}

func (r *PostgresRepository) GetGood(ctx context.Context, id, projectID int) (*models.Good, error) {
	query := `
        SELECT id, project_id, name, description, priority, removed, created_at
        FROM goods
        WHERE id = $1 AND project_id = $2 AND removed = false`

	var good models.Good
	err := r.pool.QueryRow(ctx, query, id, projectID).Scan(
		&good.ID,
		&good.ProjectID,
		&good.Name,
		&good.Description,
		&good.Priority,
		&good.Removed,
		&good.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &good, nil
}

func (r *PostgresRepository) MarkAsRemoved(ctx context.Context, good *models.Good) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Проверка записи
	exists, err := r.CheckGoodExists(ctx, good.ID, good.ProjectID)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New("good not found")
	}

	query := `
        UPDATE goods 
        SET removed = true 
        WHERE id = $1 AND project_id = $2 AND removed = false
        RETURNING name, description, priority`

	err = tx.QueryRow(ctx, query, good.ID, good.ProjectID).Scan(
		&good.Name, &good.Description, &good.Priority)
	if err != nil {
		return err
	}

	if err = tx.Commit(ctx); err != nil {
		return err
	}

	return nil
}

func (r *PostgresRepository) GetTotalCount(ctx context.Context) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
        SELECT COUNT(*) 
        FROM goods 
        WHERE removed = false`,
	).Scan(&count)

	return count, err
}

func (r *PostgresRepository) GetRemovedCount(ctx context.Context) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
        SELECT COUNT(*) 
        FROM goods 
        WHERE removed = true`,
	).Scan(&count)

	return count, err
}

func (r *PostgresRepository) CheckGoodExists(ctx context.Context, id, projectId int) (bool, error) {
	var exists bool
	query := `
		SELECT EXISTS (
			SELECT 1
			FROM goods
			WHERE id = $1
			AND project_id = $2
			AND NOT removed
		)`

	err := r.pool.QueryRow(ctx, query, id, projectId).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}
