-- Add contest submission context to submissions table.
-- This migration aligns runtime schema with db/final_01.sql.

ALTER TABLE submissions
    ADD COLUMN contest_id CHAR(36) NULL AFTER problem_id;

CREATE INDEX idx_sub_contest_user_problem_correct
    ON submissions(contest_id, user_id, problem_id, is_correct);
