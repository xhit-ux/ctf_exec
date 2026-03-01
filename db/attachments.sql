-- Attachments / files for challenges
CREATE TABLE attachments (
    id CHAR(36) PRIMARY KEY,
    problem_id INT NOT NULL,
    file_name VARCHAR(255),
    storage_path VARCHAR(1024),
    file_type VARCHAR(50),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (problem_id) REFERENCES problems(id) ON DELETE CASCADE
) ENGINE=InnoDB;
