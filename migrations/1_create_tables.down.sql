DROP TABLE IF EXISTS markets;
DROP TABLE IF EXISTS securities;
DROP TABLE IF EXISTS orders;
DROP TABLE IF EXISTS portfolio_securities;
DROP TABLE IF EXISTS portfolios;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS security_costs;

DROP INDEX IF EXISTS security_costs_date_index;
DROP INDEX IF EXISTS markets_uuid_index;
DROP INDEX IF EXISTS securities_uuid_index;
DROP INDEX IF EXISTS orders_uuid_index;