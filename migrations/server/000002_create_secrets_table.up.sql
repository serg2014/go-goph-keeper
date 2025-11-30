CREATE TABLE IF NOT EXISTS secrets (
    id BIGSERIAL PRIMARY KEY,
    user_id uuid NOT NULL
);

CREATE INDEX IF NOT EXISTS secrets_user_id_idx ON secrets (user_id);
