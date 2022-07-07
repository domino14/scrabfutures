-- all dates are ISO8601
CREATE TABLE IF NOT EXISTS markets (
    id INTEGER PRIMARY KEY autoincrement,
    uuid TEXT,
    description TEXT,
    date_created TEXT,
    is_open TINYINT,
    date_closed TEXT -- or resolved
);

CREATE TABLE IF NOT EXISTS securities (
    id INTEGER PRIMARY KEY autoincrement,
    uuid TEXT,
    description TEXT,
    shortname TEXT,
    date_created TEXT,
    market_id INTEGER,
    shares_outstanding REAL,
    last_price REAL,
    FOREIGN KEY (market_id) REFERENCES markets(id)
);

CREATE TABLE IF NOT EXISTS orders (
    id INTEGER PRIMARY KEY autoincrement,
    uuid TEXT,
    user_id INTEGER,
    security_id INTEGER,
    amount REAL,  -- how many securities
    cost REAL,   -- total cost (negative if sale)
    date TEXT,
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (security_id) REFERENCES securities(id)
);

CREATE INDEX IF NOT EXISTS markets_uuid_index ON markets(uuid);
CREATE INDEX IF NOT EXISTS securities_uuid_index ON securities(uuid);
CREATE INDEX IF NOT EXISTS orders_uuid_index ON orders(uuid);


CREATE TABLE IF NOT EXISTS portfolio_securities (
    user_id INTEGER,
    security_id INTEGER,
    amount REAL,
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (security_id) REFERENCES securities(id)
);

CREATE TABLE IF NOT EXISTS security_costs (
    security_id INTEGER,
    cost REAL,
    date TEXT,
    FOREIGN KEY(security_id) REFERENCES securities
);

CREATE INDEX IF NOT EXISTS security_costs_date_index ON security_costs(date);

CREATE TABLE IF NOT EXISTS portfolio (
    user_id INTEGER,
    tokens REAL,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS users (
    id  INTEGER PRIMARY KEY autoincrement,
    username TEXT,
    email TEXT,
    password TEXT
);