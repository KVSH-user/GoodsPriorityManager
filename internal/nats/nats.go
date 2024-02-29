package nats

import (
	"encoding/json"
	"fmt"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/nats-io/nats.go"
	"hezzl_test/internal/entity"
	"hezzl_test/internal/storage/clickhouse"
)

func SubscribeToNATSEvents(natsConn *nats.Conn, chDB driver.Conn) error {
	const op = "internal.nats.SubscribeToNATSEvents"

	go func() {
		_, err := natsConn.Subscribe("goods.created", func(m *nats.Msg) {
			var event entity.GoodEvent
			if err := json.Unmarshal(m.Data, &event); err != nil {
				fmt.Errorf("%s : %w", op, err)
				return
			}

			if err := clickhouse.InsertLogToClickHouse(chDB, event); err != nil {
				fmt.Errorf("%s : %w", op, err)
				return
			}
		})
		if err != nil {
			fmt.Errorf("%s : %w", op, err)
			return
		}
	}()

	go func() {
		_, err := natsConn.Subscribe("goods.updated", func(m *nats.Msg) {
			var event entity.GoodEvent
			if err := json.Unmarshal(m.Data, &event); err != nil {
				fmt.Errorf("%s : %w", op, err)
				return
			}

			if err := clickhouse.InsertLogToClickHouse(chDB, event); err != nil {
				fmt.Errorf("%s : %w", op, err)
				return
			}
		})
		if err != nil {
			fmt.Errorf("%s : %w", op, err)
			return
		}
	}()

	go func() {
		_, err := natsConn.Subscribe("goods.removed", func(m *nats.Msg) {
			var event entity.GoodEvent
			if err := json.Unmarshal(m.Data, &event); err != nil {
				fmt.Errorf("%s : %w", op, err)
				return
			}

			if err := clickhouse.InsertLogToClickHouse(chDB, event); err != nil {
				fmt.Errorf("%s : %w", op, err)
				return
			}
		})
		if err != nil {
			fmt.Errorf("%s : %w", op, err)
			return
		}
	}()
	return nil
}
