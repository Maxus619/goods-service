package service

import (
	"context"
	"encoding/json"
	"github.com/nats-io/nats.go"
	"goods-service/internal/models"
	"goods-service/internal/repository"
	"log"
	"time"
)

type NATSSubscriber struct {
	natsConn       *nats.Conn
	clickhouseRepo *repository.ClickhouseRepository
}

func NewNATSSubscriber(natsConn *nats.Conn, clickhouseRepo *repository.ClickhouseRepository) *NATSSubscriber {
	return &NATSSubscriber{
		natsConn:       natsConn,
		clickhouseRepo: clickhouseRepo,
	}
}

func (s *NATSSubscriber) Subscribe() error {
	_, err := s.natsConn.Subscribe("good.*", func(msg *nats.Msg) {
		log.Printf("recieved NATS message: subject=%s", msg.Subject)

		var good models.Good
		if err := json.Unmarshal(msg.Data, &good); err != nil {
			log.Printf("error unmarshalling NATS message: %v", err)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := s.clickhouseRepo.LogGoodEvent(ctx, &good); err != nil {
			log.Printf("error logging NATS message: %v", err)
			return
		}

		log.Printf("successfully logged event to ClickHouse: good_id=%d, project_id=%d",
			good.ID,
			good.ProjectID)
	})

	if err != nil {
		return err
	}

	log.Println("successfully subscribed to NATS topics: good.*")

	return nil
}
