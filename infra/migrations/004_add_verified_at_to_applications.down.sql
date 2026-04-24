-- Migration 004: Remove verified_at column from applications table

-- Drop index if exists
DROP INDEX IF EXISTS idx_application_verified_at;

-- Remove verified_at column from applications table
ALTER TABLE application 
DROP COLUMN verified_at;