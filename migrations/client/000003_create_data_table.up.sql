CREATE TABLE IF NOT EXISTS data (
    id integer NOT NULL PRIMARY KEY,
    secret_id integer NOT NULL,
    version integer,
    updated_at integer NOT NULL DEFAULT (strftime('%s', 'now')),
    need_update integer(1) NOT NULL DEFAULT (0),
    data blob NOT NULL
);
CREATE INDEX data_secret_id_idx ON data (secret_id);