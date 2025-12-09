CREATE TABLE IF NOT EXISTS meta (
    secret_id text(255) NOT NULL PRIMARY KEY,
    version integer,
    updated_at integer NOT NULL DEFAULT (strftime('%s', 'now')),
    need_update integer NULL DEFAULT (1),
    data blob NOT NULL
);
