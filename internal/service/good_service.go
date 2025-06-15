package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/nats-io/nats.go"
	"goods-service/internal/models"
	"goods-service/internal/repository"
	"log"
)

type GoodService struct {
	postgresRepo   *repository.PostgresRepository
	redisRepo      *repository.RedisRepository
	clickhouseRepo *repository.ClickhouseRepository
	natsConn       *nats.Conn
}

func NewGoodService(
	postgresRepo *repository.PostgresRepository,
	redisRepo *repository.RedisRepository,
	clickhouseRepo *repository.ClickhouseRepository,
	natsConn *nats.Conn,
) *GoodService {
	return &GoodService{
		postgresRepo:   postgresRepo,
		redisRepo:      redisRepo,
		clickhouseRepo: clickhouseRepo,
		natsConn:       natsConn,
	}
}

func (s *GoodService) CreateGood(ctx context.Context, good *models.Good) error {
	if err := s.postgresRepo.CreateGood(ctx, good); err != nil {
		return err
	}

	// Инвалидируем кэш счетчиков
	if err := s.redisRepo.InvalidateCounts(ctx); err != nil {
		log.Printf("Failed to invalidate counts cache: %v", err)
	}

	// Кэшируем новую запись
	if err := s.redisRepo.SetGood(ctx, good); err != nil {
		return err
	}

	// Отправляем событие в NATS для логирования в ClickHouse
	if err := s.publishEvent("good.created", good); err != nil {
		return err
	}

	return nil
}

func (s *GoodService) DeleteGood(ctx context.Context, id, projectID int) error {
	// Проверяем существование записи
	exists, err := s.postgresRepo.CheckGoodExists(ctx, id, projectID)
	if err != nil {
		return err
	}
	if !exists {
		return models.ErrNotFound
	}

	good := models.Good{ID: id, ProjectID: projectID}

	// Отмечаем удалённым
	if err := s.postgresRepo.MarkAsRemoved(ctx, &good); err != nil {
		return err
	}

	// Инвалидируем кэш счетчиков
	if err := s.redisRepo.InvalidateCounts(ctx); err != nil {
		log.Printf("Failed to invalidate counts cache: %v", err)
	}

	// Инвалидируем кэш записи
	if err := s.redisRepo.InvalidateGood(ctx, id, projectID); err != nil {
		return err
	}

	// Отправляем событие в NATS
	if err := s.publishEvent("good.deleted", &good); err != nil {
		return err
	}

	return nil
}

func (s *GoodService) UpdateGood(ctx context.Context, good *models.Good) error {
	// Проверяем существование записи
	exists, err := s.postgresRepo.CheckGoodExists(ctx, good.ID, good.ProjectID)
	if err != nil {
		return err
	}
	if !exists {
		return models.ErrNotFound
	}

	// Обновляем в PostgreSQL
	if err := s.postgresRepo.UpdateGood(ctx, good); err != nil {
		return err
	}

	// Инвалидируем кэш
	if err := s.redisRepo.InvalidateGood(ctx, good.ID, good.ProjectID); err != nil {
		return err
	}

	// Отправляем событие в NATS
	if err := s.publishEvent("good.updated", good); err != nil {
		return err
	}

	return nil
}

func (s *GoodService) GetGood(ctx context.Context, id, projectID int) (*models.Good, error) {
	// Проверяем существование записи
	exists, err := s.postgresRepo.CheckGoodExists(ctx, id, projectID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, models.ErrNotFound
	}

	// Пробуем получить из Redis
	good, err := s.redisRepo.GetGood(ctx, id, projectID)
	if err != nil {
		return nil, err
	}
	if good != nil {
		return good, nil
	}

	// Если нет в Redis, получаем из PostgreSQL
	good, err = s.postgresRepo.GetGood(ctx, id, projectID)
	if err != nil {
		return nil, err
	}
	if good == nil {
		return nil, errors.New("good not found")
	}

	// Кэшируем результат
	if err := s.redisRepo.SetGood(ctx, good); err != nil {
		return nil, err
	}

	return good, nil
}

func (s *GoodService) ListGoods(ctx context.Context, limit, offset int) ([]models.Good, error) {
	return s.postgresRepo.ListGoods(ctx, limit, offset)
}

func (s *GoodService) ReprioritizeGood(ctx context.Context, id, projectID, newPriority int) (*models.PriorityResponse, error) {
	// Обновляем приоритеты
	updatedPriorities, err := s.postgresRepo.ReprioritizeGoods(ctx, id, projectID, newPriority)
	if err != nil {
		return nil, err
	}

	var priorityItems []models.PriorityItem

	for _, item := range updatedPriorities {
		// Добавляем в ответ
		priorityItems = append(priorityItems, models.PriorityItem{
			ID:       item.ID,
			Priority: item.Priority,
		})

		// Инвалидируем кэш для всех затронутых записей
		if err := s.redisRepo.InvalidateGood(ctx, item.ID, projectID); err != nil {
			log.Printf("error invalidating Redis cache for good %d: %v", item.ID, err)
		}

		// Отправляем события в NATS
		if err := s.publishEvent("good.reprioritized", &item); err != nil {
			log.Printf("error publishing reprioritize event: %v", err)
		}
	}

	return &models.PriorityResponse{
		Priorities: priorityItems,
	}, nil
}

// GetTotalCount возвращает общее количество неудаленных записей
func (s *GoodService) GetTotalCount(ctx context.Context) (int, error) {
	// Пробуем получить из Redis
	count, err := s.redisRepo.GetTotalCount(ctx)
	if err == nil {
		return count, nil
	}

	// Если нет в Redis, получаем из PostgreSQL
	count, err = s.postgresRepo.GetTotalCount(ctx)
	if err != nil {
		return 0, err
	}

	// Кэшируем результат
	if err := s.redisRepo.SetTotalCount(ctx, count); err != nil {
		// Логируем ошибку, но не прерываем выполнение
		log.Printf("Failed to cache total count: %v", err)
	}

	return count, nil
}

// GetRemovedCount возвращает количество удаленных записей
func (s *GoodService) GetRemovedCount(ctx context.Context) (int, error) {
	// Пробуем получить из Redis
	count, err := s.redisRepo.GetRemovedCount(ctx)
	if err == nil {
		return count, nil
	}

	// Если нет в Redis, получаем из PostgreSQL
	count, err = s.postgresRepo.GetRemovedCount(ctx)
	if err != nil {
		return 0, err
	}

	// Кэшируем результат
	if err := s.redisRepo.SetRemovedCount(ctx, count); err != nil {
		// Логируем ошибку, но не прерываем выполнение
		log.Printf("Failed to cache removed count: %v", err)
	}

	return count, nil
}

// publishEvent Публикация события в NATS
func (s *GoodService) publishEvent(subject string, good *models.Good) error {
	event := models.NewClickhouseEvent(good)

	bytes, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("error marshaling event: %v", err)
	}

	if err := s.natsConn.Publish(subject, bytes); err != nil {
		return fmt.Errorf("error publishing to NATS: %v", err)
	}

	return nil
}
