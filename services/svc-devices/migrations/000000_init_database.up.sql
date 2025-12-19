-- Create database if not exists (run this manually or via init script)
-- Note: This cannot be run inside a transaction, so it's typically
-- executed via docker-entrypoint-initdb.d or manually.

-- The database 'devices' should be created before running migrations.
-- This file serves as documentation for the required setup.

-- CREATE DATABASE devices;

-- Grant privileges
-- GRANT ALL PRIVILEGES ON DATABASE devices TO postgres;
