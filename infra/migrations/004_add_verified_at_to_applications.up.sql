-- Migration 004: Add verified_at column to applications table for Telegram bot verification

-- Add verified_at column to applications table
ALTER TABLE application 
ADD COLUMN verified_at TIMESTAMP WITH TIME ZONE;

-- Set default value for existing active Telegram bots
UPDATE application 
SET verified_at = created_at 
WHERE status = 'active' AND platform = 'telegram';

-- Add index for better query performance
CREATE INDEX IF NOT EXISTS idx_application_verified_at 
ON application (verified_at) 
WHERE verified_at IS NOT NULL;

-- Add comment to describe the new column
COMMENT ON COLUMN application.verified_at IS 'Timestamp of last successful verification for Telegram bots';