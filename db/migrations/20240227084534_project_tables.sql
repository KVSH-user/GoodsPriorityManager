-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS projects (
                                        id SERIAL PRIMARY KEY NOT NULL,
                                        name VARCHAR NOT NULL,
                                        created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

INSERT INTO projects (name) VALUES ('Первая запись');

CREATE TABLE IF NOT EXISTS goods (
                                     id SERIAL PRIMARY KEY NOT NULL,
                                     project_id INTEGER NOT NULL,
                                     name VARCHAR NOT NULL,
                                     description VARCHAR,
                                     priority INTEGER NOT NULL,
                                     removed BOOLEAN NOT NULL DEFAULT FALSE,
                                     created_at TIMESTAMP NOT NULL DEFAULT NOW(),
                                     FOREIGN KEY (project_id) REFERENCES projects(id)
);

CREATE OR REPLACE FUNCTION update_goods_priority() RETURNS TRIGGER AS $$
BEGIN
    NEW.priority := COALESCE((SELECT MAX(priority) FROM goods WHERE project_id = NEW.project_id), 0) + 1;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER set_goods_priority BEFORE INSERT ON goods
    FOR EACH ROW EXECUTE FUNCTION update_goods_priority();

CREATE INDEX projects_id ON projects (id);
CREATE INDEX goods_indexes ON goods (id, project_id, name);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP FUNCTION IF EXISTS reprioritize_goods;
DROP TABLE IF EXISTS goods;
DROP TABLE IF EXISTS projects;
-- +goose StatementEnd
