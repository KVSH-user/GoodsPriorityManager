package clickhouse

import (
	"context"
	"fmt"
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/sirupsen/logrus"
	"hezzl_test/internal/entity"
	"sync"
	"time"
)

var (
	buffer        []entity.GoodEvent
	mutex         sync.Mutex
	maxBatchSize  = 100             // Максимальный размер батча
	flushInterval = 5 * time.Second // Интервал для отправки данных
	chDBConn      driver.Conn
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

	chDBConn = chDB

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

func BufferEvent(event entity.GoodEvent) {
	mutex.Lock()
	defer mutex.Unlock()
	buffer = append(buffer, event)

	if len(buffer) >= maxBatchSize {
		flushBuffer()
	}
}

func flushBuffer() {
	if len(buffer) == 0 {
		return
	}

	eventsToFlush := make([]entity.GoodEvent, len(buffer))
	copy(eventsToFlush, buffer)
	buffer = make([]entity.GoodEvent, 0)

	go InsertLogBatchToClickHouse(chDBConn, eventsToFlush)
}

func StartFlusher() {
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	for {
		<-ticker.C
		mutex.Lock()
		flushBuffer()
		mutex.Unlock()
	}
}

func InsertLogBatchToClickHouse(chDB driver.Conn, events []entity.GoodEvent) error {
	const op = "storage.clickhouse.InsertLogBatchToClickHouse"
	ctx := context.Background()

	batch, err := chDB.PrepareBatch(ctx, "INSERT INTO events (id, ProjectId, Name, Description, Priority, Removed, EventTime)")
	if err != nil {
		return fmt.Errorf("%s: prepare batch: %w", op, err)
	}

	for _, event := range events {
		removed := 0
		if event.Removed {
			removed = 1
		}
		createdAt := event.EventTime.Format("2006-01-02 15:04:05")
		if err := batch.Append(
			event.Id,
			event.ProjectId,
			event.Name,
			event.Description,
			event.Priority,
			removed,
			createdAt,
		); err != nil {
			return fmt.Errorf("%s: append to batch: %w", op, err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("%s: batch send: %w", op, err)
	}

	return nil
}

func CreateTableClickHouse(chDB driver.Conn) error {
	const op = "storage.clickhouse.CreateTableClickHouse"

	ctx := context.Background()

	err := chDB.Exec(ctx, `
	CREATE TABLE IF NOT EXISTS events(
                        id Int32,
                        ProjectId Int32,
                        Name String,
                        Description String,
                        Priority Int32,
                        Removed UInt8,
                        EventTime DateTime
) ENGINE = MergeTree()
      ORDER BY (id, ProjectId, Name);
	`)
	if err != nil {
		return fmt.Errorf("failed create ClickHouse table: %s: %w", op, err)
	}
	logrus.Info("Clickhouse table created")
	return nil
}
