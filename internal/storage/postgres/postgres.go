package postgres

import (
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/lib/pq"
	"github.com/pressly/goose"
	"hezzl_test/internal/entity"
	"log"
	"os"
)

type Storage struct {
	db *sql.DB
}

var ErrNotFound = errors.New("record not found")

func New(host, port, user, password, dbName string) (*Storage, error) {
	const op = "storage.postgres.New"

	psqlInfo := fmt.Sprintf("host=%s port=%s user=%s "+
		"password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbName)

	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	err = db.Ping()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	storage := &Storage{db: db}

	cwd, _ := os.Getwd()
	log.Println("Current working directory:", cwd)

	err = goose.Up(storage.db, "db/migrations")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return storage, nil
}

func (s *Storage) CreateGood(projectId int, name string) (entity.GoodCreateResponse, error) {
	const op = "storage.postgres.CreateGood"

	var response entity.GoodCreateResponse
	var description sql.NullString

	query := `
		INSERT INTO goods (project_id, name) VALUES ($1, $2) RETURNING *;
		`

	tx, err := s.db.Begin()
	if err != nil {
		return response, fmt.Errorf("%s: %w", op, err)
	}

	err = tx.QueryRow(query, projectId, name).Scan(&response.Id,
		&response.ProjectId,
		&response.Name,
		&description,
		&response.Priority,
		&response.Removed,
		&response.CreatedAt)
	if err != nil {
		tx.Rollback()
		return response, fmt.Errorf("%s: %w", op, err)
	}

	if description.Valid {
		response.Description = description.String
	} else {
		response.Description = ""
	}

	err = tx.Commit()
	if err != nil {
		return response, fmt.Errorf("%s: %w", op, err)
	}

	return response, nil
}

func (s *Storage) UpdateGood(id, projectId int, name, description string) (entity.GoodUpdateResponse, error) {
	const op = "storage.postgres.UpdateGood"

	var response entity.GoodUpdateResponse

	query := `
		UPDATE goods SET name = $1, description = $2 WHERE id = $3 AND project_id = $4 RETURNING *;
		`

	tx, err := s.db.Begin()
	if err != nil {
		return response, fmt.Errorf("%s: %w", op, err)
	}

	err = tx.QueryRow(query, name, description, id, projectId).Scan(&response.Id,
		&response.ProjectId,
		&response.Name,
		&response.Description,
		&response.Priority,
		&response.Removed,
		&response.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return response, ErrNotFound
		}
		tx.Rollback()
		return response, fmt.Errorf("%s: %w", op, err)
	}

	err = tx.Commit()
	if err != nil {
		return response, fmt.Errorf("%s: %w", op, err)
	}

	return response, nil
}

func (s *Storage) DeleteGood(id, projectId int) (entity.GoodRemoveResponse, string, string, int, error) {
	const op = "storage.postgres.DeleteGood"

	var (
		response    entity.GoodRemoveResponse
		name        string
		description string
		priority    int
	)

	query := `
		UPDATE goods SET removed = true WHERE id = $1 AND project_id = $2 RETURNING id, project_id, name, description, priority, removed;
		`

	tx, err := s.db.Begin()
	if err != nil {
		return response, name, description, priority, fmt.Errorf("%s: %w", op, err)
	}

	err = tx.QueryRow(query, id, projectId).Scan(&response.Id,
		&response.ProjectId,
		&name,
		&description,
		&priority,
		&response.Removed)
	if err != nil {
		if err == sql.ErrNoRows {
			return response, name, description, priority, ErrNotFound
		}
		tx.Rollback()
		return response, name, description, priority, fmt.Errorf("%s: %w", op, err)
	}

	err = tx.Commit()
	if err != nil {
		return response, name, description, priority, fmt.Errorf("%s: %w", op, err)
	}

	return response, name, description, priority, nil
}

func (s *Storage) GetGoodByID(key int) (entity.GoodsForList, error) {
	const op = "storage.postgres.GetGoodByID"

	var response entity.GoodsForList
	var description sql.NullString

	query := `
	SELECT 
	    *
	FROM goods
	WHERE id = $1;
	`

	err := s.db.QueryRow(query, key).Scan(
		&response.Id,
		&response.ProjectId,
		&response.Name,
		&description,
		&response.Priority,
		&response.Removed,
		&response.CreatedAt,
	)
	if err != nil {
		return response, fmt.Errorf("%s: %w", op, err)
	}

	if description.Valid {
		response.Description = description.String
	} else {
		response.Description = ""
	}

	return response, nil
}

func (s *Storage) CalculateTotalAndRemoved() (int, int, error) {
	const op = "storage.postgres.CalculateTotalAndRemoved"

	var total, removed int

	query := `
	SELECT
  COUNT(*) AS total_count,
  COUNT(*) FILTER (WHERE removed = true) AS removed_count
FROM
  goods;
	`

	err := s.db.QueryRow(query).Scan(&total, &removed)
	if err != nil {
		return 0, 0, fmt.Errorf("%s: %w", op, err)
	}

	return total, removed, nil
}

func (s *Storage) Reprioritize(goodID, projectID, newPriority int) (string, string, error) {
	const op = "storage.postgres.Reprioritize"

	var (
		name           string
		descriptionStr string
		description    sql.NullString
	)

	tx, err := s.db.Begin()
	if err != nil {
		return "", "", fmt.Errorf("%s: %w", op, err)
	}
	defer tx.Rollback()

	var currentPriority int

	err = tx.QueryRow(`SELECT priority FROM goods WHERE id = $1 AND project_id = $2`, goodID, projectID).Scan(&currentPriority)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", "", ErrNotFound
		}
		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	if newPriority < currentPriority {
		_, err = tx.Exec(`UPDATE goods SET priority = priority + 1 WHERE project_id = $1 AND priority >= $2 AND priority < $3`, projectID, newPriority, currentPriority)
	} else if newPriority > currentPriority {
		_, err = tx.Exec(`UPDATE goods SET priority = priority - 1 WHERE project_id = $1 AND priority <= $2 AND priority > $3`, projectID, newPriority, currentPriority)
	}
	if err != nil {
		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	err = tx.QueryRow(`UPDATE goods SET priority = $1 WHERE id = $2 AND project_id = $3 RETURNING name, description`, newPriority, goodID, projectID).Scan(&name, &description)
	if err != nil {
		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	if err := tx.Commit(); err != nil {
		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	if description.Valid {
		descriptionStr = description.String
	} else {
		descriptionStr = ""
	}

	return name, descriptionStr, nil
}
