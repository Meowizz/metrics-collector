CREATE TABLE IF NOT EXISTS metrics (
    id    VARCHAR PRIMARY KEY,
    type  VARCHAR NOT NULL CHECK (type IN ('gauge', 'counter')),
    value DOUBLE PRECISION
);

