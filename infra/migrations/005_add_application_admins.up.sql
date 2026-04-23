CREATE TABLE application_admins (
    application_id UUID NOT NULL REFERENCES application(id) ON DELETE CASCADE,
    moderator_id   UUID NOT NULL REFERENCES moderator(id)   ON DELETE CASCADE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (application_id, moderator_id)
);

CREATE INDEX idx_app_admins_moderator ON application_admins (moderator_id);
