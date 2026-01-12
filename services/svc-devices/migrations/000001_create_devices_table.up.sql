CREATE TYPE device_state AS ENUM ('available', 'in-use', 'inactive');

CREATE TABLE IF NOT EXISTS devices (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    brand VARCHAR(255) NOT NULL,
    state device_state NOT NULL DEFAULT 'available',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    search_vector tsvector GENERATED ALWAYS AS (
        to_tsvector('english', coalesce(name, '') || ' ' || coalesce(brand, ''))
    ) STORED
);

CREATE INDEX idx_devices_brand ON devices(brand);
CREATE INDEX idx_devices_state ON devices(state);
CREATE INDEX idx_devices_created_at ON devices(created_at DESC);
CREATE INDEX idx_devices_search ON devices USING GIN (search_vector);

COMMENT ON COLUMN devices.id IS 'Unique identifier for the device';
COMMENT ON COLUMN devices.name IS 'Human-readable name of the device';
COMMENT ON COLUMN devices.brand IS 'Manufacturer or brand name of the device';
COMMENT ON COLUMN devices.state IS 'Current availability state: available, in-use, or inactive';
COMMENT ON COLUMN devices.created_at IS 'Timestamp when the device record was created';
COMMENT ON COLUMN devices.updated_at IS 'Timestamp when the device record was last modified';
COMMENT ON COLUMN devices.search_vector IS 'Full-text search vector generated from name and brand fields';
