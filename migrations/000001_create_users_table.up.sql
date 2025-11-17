CREATE TABLE IF NOT EXISTS users (
    user_id uuid NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    login varchar(255) NOT NULL UNIQUE,
    hash text NOT NULL
);
