CREATE TABLE IF NOT EXISTS meta (
    id BIGSERIAL PRIMARY KEY,
    user_id uuid NOT NULL,
    secret_id bigint NOT NULL,
    version integer NOT NULL DEFAULT 0,
    updated_at integer NOT NULL,
    data bytea NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS meta_user_id_secret_id_idx ON meta USING btree (user_id, secret_id);
