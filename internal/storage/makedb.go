package storage

const CreateDB = `
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    login varchar(40) UNIQUE,
    password varchar(256),
    balance bigint DEFAULT 0,
    created_at timestamp DEFAULT current_timestamp
);


CREATE TABLE IF NOT EXISTS bonuses   (
    id SERIAL PRIMARY KEY,
    user_id bigint,
    order_id bigint,
    change bigint,
    type varchar(40) CHECK (type IN ('top_up', 'withdraw')),
    status varchar(40) CHECK (status in ('NEW', 'REGISTERED', 'INVALID', 'PROCESSING', 'PROCESSED')),
    change_date timestamp DEFAULT current_timestamp,
    FOREIGN KEY(user_id) REFERENCES users(id)
);

`
