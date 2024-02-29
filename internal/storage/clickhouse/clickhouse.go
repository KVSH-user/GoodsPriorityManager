package clickhouse

import (
	"context"
	"fmt"
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"hezzl_test/internal/entity"
	"time"
)

func SetupClickHouseConnection(host, port, user, password, dbName string) (driver.Conn, error) {
	const op = "storage.clickhouse.SetupClickHouseConnection"

	ctx := context.Background()
	chDB, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%s", host, port)},
		Auth: clickhouse.Auth{
			Database: dbName,
			Username: user,
			Password: password,
		},
		Debug:           true,
		DialTimeout:     10 * time.Second,
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: time.Hour,
	})

	if err != nil {
		return nil, fmt.Errorf("%s : %w", op, err)
	}

	if err := chDB.Ping(ctx); err != nil {
		if exception, ok := err.(*clickhouse.Exception); ok {
			fmt.Printf("Exception [%d] %s \n%s\n", exception.Code, exception.Message, exception.StackTrace)
		}
		return nil, fmt.Errorf("%s : %w", op, err)
	}
	return chDB, nil
}

func InsertLogToClickHouse(chDB driver.Conn, event entity.GoodEvent) error {
	const op = "storage.clickhouse.InsertLogToClickHouse"

	ctx := context.Background()

	removed := 0
	if event.Removed {
		removed = 1
	}

	createdAt := event.EventTime.Format("2006-01-02 15:04:05")

	err := chDB.Exec(ctx, `
		INSERT INTO events (id, ProjectId, Name, Description, Priority, Removed, EventTime)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`,
		event.Id,
		event.ProjectId,
		event.Name,
		event.Description,
		event.Priority,
		removed,
		createdAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert event to ClickHouse: %s: %w", op, err)
	}
	return nil
}
