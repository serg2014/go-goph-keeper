CREATE TABLE IF NOT EXISTS actions (
    secret_id text(255) NOT NULL PRIMARY KEY,
    action_type integer NOT NULL,
    conflict integer NOT NULL DEFAULT (0)
);

CREATE TABLE IF NOT EXISTS actions_type (
    id integer NOT NULL PRIMARY KEY,
    type text(255) NOT NULL
);

INSERT INTO actions_type (id, type) VALUES (0, 'create');
INSERT INTO actions_type (id, type) VALUES (1, 'update');
INSERT INTO actions_type (id, type) VALUES (2, 'delete');
