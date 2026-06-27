
-- SCHEMAS

CREATE SCHEMA IF NOT EXISTS files AUTHORIZATION pguser;

-- TABLES

CREATE TABLE IF NOT EXISTS files.blacklist
(
    bagid character varying(64) COLLATE pg_catalog."default" NOT NULL,
    admin character varying(64) COLLATE pg_catalog."default" NOT NULL,
    reason text COLLATE pg_catalog."default" NOT NULL,
    comment text COLLATE pg_catalog."default" NOT NULL,
    created_at timestamp with time zone DEFAULT now(),
    CONSTRAINT blacklist_pkey PRIMARY KEY (bagid)
);

CREATE TABLE IF NOT EXISTS files.blacklist_history
(
    id SERIAL NOT NULL,
    bagid character varying(64) COLLATE pg_catalog."default" NOT NULL,
    admin character varying(64) COLLATE pg_catalog."default" NOT NULL,
    reason text COLLATE pg_catalog."default" NOT NULL,
    comment text COLLATE pg_catalog."default" NOT NULL,
    banned boolean NOT NULL,
    created_at timestamp with time zone DEFAULT now(),
    archived_at timestamp with time zone DEFAULT now(),
    CONSTRAINT blacklist_history_pkey PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS files.reports
(
    id SERIAL NOT NULL,
    bagid character varying(64) COLLATE pg_catalog."default" NOT NULL,
    sender text COLLATE pg_catalog."default" NOT NULL,
    reason text COLLATE pg_catalog."default" NOT NULL,
    comment text COLLATE pg_catalog."default" NOT NULL,
    created_at timestamp with time zone DEFAULT now(),
    CONSTRAINT reports_pkey PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS files.reports_archive
(
    id SERIAL NOT NULL,
    bagid character varying(64) COLLATE pg_catalog."default" NOT NULL,
    admin text COLLATE pg_catalog."default" NOT NULL,
    sender text COLLATE pg_catalog."default" NOT NULL,
    reason text COLLATE pg_catalog."default" NOT NULL,
    comment text COLLATE pg_catalog."default" NOT NULL,
    created_at timestamp with time zone,
    accepted boolean NOT NULL DEFAULT false,
    CONSTRAINT reports_archive_pkey PRIMARY KEY (id),
    CONSTRAINT reports_archive_bagid_admin_key UNIQUE (bagid, admin)
);

-- TRIGGERS AND FUNCTIONS

CREATE FUNCTION files.log_blacklist_changes()
    RETURNS trigger
    LANGUAGE plpgsql
    COST 100
    VOLATILE NOT LEAKPROOF
AS $BODY$
BEGIN
    -- Handle DELETE
    IF TG_OP = 'DELETE' THEN
        INSERT INTO files.blacklist_history (
            bagid,
            admin,
            reason,
            comment,
            banned,
            created_at
        ) VALUES (
            OLD.bagid,
            OLD.admin,
            OLD.reason,
            OLD.comment,
            false,
            NOW()
        );
        RETURN OLD;
    END IF;

    -- Handle INSERT and UPDATE
    INSERT INTO files.blacklist_history (
        bagid,
        admin,
        reason,
        comment,
        banned,
        created_at
    ) VALUES (
        NEW.bagid,
        NEW.admin,
        NEW.reason,
        NEW.comment,
        true,
        COALESCE(NEW.created_at, NOW())
    );
    RETURN NEW;
END;
$BODY$;

CREATE OR REPLACE TRIGGER trigger_blacklist_delete
    AFTER DELETE
    ON files.blacklist
    FOR EACH ROW
    EXECUTE FUNCTION files.log_blacklist_changes();

CREATE OR REPLACE TRIGGER trigger_blacklist_insert
    AFTER INSERT
    ON files.blacklist
    FOR EACH ROW
    EXECUTE FUNCTION files.log_blacklist_changes();

CREATE OR REPLACE TRIGGER trigger_blacklist_update
    AFTER UPDATE
    ON files.blacklist
    FOR EACH ROW
    EXECUTE FUNCTION files.log_blacklist_changes();
