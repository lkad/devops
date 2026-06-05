-- tests/fixtures/db/seed.sql
--
-- Minimal PostgreSQL seed for the dev environment. Per spec Tier 1
-- requirements:
--   2 project_types  (Platform, Product)
--   1 business_line
--   1 system
--   1 project
--   2 devices
--   1 k8s cluster
--
-- Idempotent: every INSERT uses ON CONFLICT DO NOTHING so the file can be
-- re-applied (e.g. by reset.sh). GORM AutoMigrate creates the tables first
-- (this script is run AFTER db.AutoMigrate).

BEGIN;

-- project_types
INSERT INTO project_types (id, name, description, created_at, updated_at)
VALUES
  ('pt-platform', 'Platform', 'Internal infrastructure projects', NOW(), NOW()),
  ('pt-product',  'Product',  'Customer-facing product projects',     NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- business_lines
INSERT INTO business_lines (id, name, description, weight, created_at, updated_at)
VALUES
  ('bl-ecommerce', 'E-Commerce', 'E-commerce business line', 0.6, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- systems
INSERT INTO systems (id, name, business_line_id, created_at, updated_at)
VALUES
  ('sys-order', 'Order System', 'bl-ecommerce', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- projects
INSERT INTO projects (id, name, system_id, type_id, created_at, updated_at)
VALUES
  ('proj-order-backend', 'order-backend', 'sys-order', 'pt-platform', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- devices (2 physical hosts)
INSERT INTO devices (id, name, type, ip, dc, status, ssh_port, created_at, updated_at)
VALUES
  ('dev-dc1-web-21', 'dc1-web-21', 'physical_host', '172.30.30.21', 'dc1', 'online',  22, NOW(), NOW()),
  ('dev-dc2-web-41', 'dc2-web-41', 'physical_host', '172.30.30.41', 'dc2', 'online',  22, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- k8s clusters
INSERT INTO kubernetes_clusters (id, name, api_server, created_at, updated_at)
VALUES
  ('cl-dev', 'dev-cluster', 'https://kubernetes.default.svc:443', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

COMMIT;
