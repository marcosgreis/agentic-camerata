-- Sessions table for tracking Claude coding sessions
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,

    -- Session info
    workflow_type TEXT NOT NULL DEFAULT 'general',  -- general, research, plan, implement, fix, play, review
    status TEXT NOT NULL DEFAULT 'waiting',          -- waiting, working, completed, abandoned
    working_directory TEXT NOT NULL,
    task_description TEXT,
    prefix TEXT,  -- CMT_PREFIX environment variable value

    -- Claude CLI info
    claude_session_id TEXT,

    -- Tmux location (for jumping back)
    tmux_session TEXT NOT NULL,
    tmux_window INTEGER NOT NULL,
    tmux_pane INTEGER NOT NULL,

    -- Output tracking
    output_file TEXT,
    playbook_file TEXT,
    pid INTEGER,
    deleted_at DATETIME,
    parent_id TEXT
);

CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status);
CREATE INDEX IF NOT EXISTS idx_sessions_workflow ON sessions(workflow_type);
CREATE INDEX IF NOT EXISTS idx_sessions_created ON sessions(created_at DESC);
-- Note: idx_sessions_deleted is created in db.go after the deleted_at column migration

CREATE TABLE IF NOT EXISTS todos (
    id TEXT PRIMARY KEY,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    status TEXT NOT NULL DEFAULT 'todo',   -- 'todo' | 'done'
    summary TEXT NOT NULL,
    date DATETIME,
    source TEXT,
    url TEXT,
    channel TEXT,
    sender TEXT,
    idempotency_key TEXT UNIQUE,
    full_message TEXT,
    deleted_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_todos_status ON todos(status);
CREATE INDEX IF NOT EXISTS idx_todos_created ON todos(created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_todos_idempotency ON todos(idempotency_key) WHERE idempotency_key IS NOT NULL;
