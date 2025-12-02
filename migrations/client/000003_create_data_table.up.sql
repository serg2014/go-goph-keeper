CREATE TABLE IF NOT EXISTS data (
    secret_id text(255) NOT NULL PRIMARY KEY,
    version integer,
    updated_at integer NOT NULL DEFAULT (strftime('%s', 'now')),
    data blob NOT NULL
);