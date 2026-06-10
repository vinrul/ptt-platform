-- +goose Up
-- +goose StatementBegin
ALTER TABLE devices ADD COLUMN push_token TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE devices DROP COLUMN push_token;
-- +goose StatementEnd
