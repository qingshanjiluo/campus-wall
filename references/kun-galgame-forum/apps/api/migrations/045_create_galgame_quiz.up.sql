-- 045: galgame_quiz — user-authored galgame quiz questions (答题), plus a
-- per-user answer table (galgame_quiz_answer) that doubles as the participant
-- roster and the quality-vote store.
--
-- Question types (`type`):
--   single / multiple / judge / fill  → auto-graded server-side; a correct
--                                        answer grants moemoepoint.
--   essay                             → NOT graded; only reveals a reference
--                                        answer (no moemoepoint on answering).
-- The type-specific payload (options, correct-answer key, accepted answers,
-- essay reference) lives in the `content` JSONB column. The answer KEY is
-- never sent to the client until the viewer has answered — grading is
-- server-only.
--
-- MVP has NO review gate: anyone publishes immediately (creator/审核 roles are
-- deferred until the upstream RBAC/AI-review infra lands). Authoring grants
-- moemoepoint at create time; deleting the question refunds it — mirrors the
-- galgame_rating create-grant / delete-refund model.
--
-- `galgame_id` is OPTIONAL: a question may be about one specific game or be
-- general galgame trivia (NULL). No FK to `galgame` — the linked game is
-- resolved via the wiki client, never a local join (matches
-- galgame_collection_item).
-- Idempotent.
BEGIN;

CREATE TABLE IF NOT EXISTS galgame_quiz (
    id            BIGSERIAL    PRIMARY KEY,
    user_id       INTEGER      NOT NULL,                    -- author, aligns with OAuth users.id
    galgame_id    INTEGER,                                  -- optional linked game; NULL = general trivia
    category      VARCHAR(16)  NOT NULL DEFAULT 'other'
                    CHECK (category IN ('plot','character','system','music','voice','company','trivia','other')),
    type          VARCHAR(16)  NOT NULL
                    CHECK (type IN ('single','multiple','judge','fill','essay')),
    difficulty    SMALLINT     NOT NULL DEFAULT 1 CHECK (difficulty BETWEEN 1 AND 10),
    question      TEXT         NOT NULL,                    -- prompt (markdown)
    content       JSONB        NOT NULL DEFAULT '{}',       -- type-specific payload incl. answer key (server-only)
    explanation   TEXT         NOT NULL DEFAULT '',         -- shown after answering
    view          INTEGER      NOT NULL DEFAULT 0,
    answer_count  INTEGER      NOT NULL DEFAULT 0,          -- number of genuine answerers (excludes the author row)
    correct_count INTEGER      NOT NULL DEFAULT 0,
    quality_sum   INTEGER      NOT NULL DEFAULT 0,          -- Σ quality votes (avg = sum / count)
    quality_count INTEGER      NOT NULL DEFAULT 0,
    created       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated       TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- List / browse: newest first, filterable by linked game, author, category, type.
CREATE INDEX IF NOT EXISTS idx_galgame_quiz_created ON galgame_quiz (created DESC);
CREATE INDEX IF NOT EXISTS idx_galgame_quiz_galgame ON galgame_quiz (galgame_id);
CREATE INDEX IF NOT EXISTS idx_galgame_quiz_user    ON galgame_quiz (user_id);

COMMENT ON COLUMN public.galgame_quiz.content IS
  'Type-specific payload including the correct-answer key. NEVER serialize verbatim to a client that has not answered — the read layer strips the key.';

-- One row per (quiz, user). Serves three roles at once:
--   role='author'    → the author auto-row (created with the question). Marks the
--                       author as a participant so they cannot answer their own
--                       question; carries no submitted answer / grade.
--   role='answerer'  → a real attempt. `is_correct` is the graded result (NULL
--                       for essay). `rewarded` guards the one-time correct-answer
--                       moemoepoint grant. `quality_rating` is their 1-10 vote.
CREATE TABLE IF NOT EXISTS galgame_quiz_answer (
    id             BIGSERIAL    PRIMARY KEY,
    quiz_id        BIGINT       NOT NULL REFERENCES galgame_quiz (id) ON DELETE CASCADE,
    user_id        INTEGER      NOT NULL,
    role           VARCHAR(16)  NOT NULL DEFAULT 'answerer'
                     CHECK (role IN ('author','answerer')),
    submitted      JSONB,                                   -- what the user answered (NULL for the author row)
    is_correct     BOOLEAN,                                 -- grade (NULL = author row / essay / ungraded)
    rewarded       BOOLEAN      NOT NULL DEFAULT false,     -- correct-answer moemoepoint idempotency guard
    quality_rating SMALLINT     CHECK (quality_rating BETWEEN 1 AND 10),  -- their quality vote (NULL = not rated)
    created        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (quiz_id, user_id)
);

-- "My answered quizzes", newest attempt first.
CREATE INDEX IF NOT EXISTS idx_gqa_user ON galgame_quiz_answer (user_id, created DESC);
-- Roster / stats per quiz.
CREATE INDEX IF NOT EXISTS idx_gqa_quiz ON galgame_quiz_answer (quiz_id);

COMMIT;
