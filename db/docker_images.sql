-- Docker images registry for challenges
CREATE TABLE docker_images (
    id CHAR(36) PRIMARY KEY,
    problem_id INT NOT NULL,
    image_name VARCHAR(255) NOT NULL,
    image_tag VARCHAR(128),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (problem_id) REFERENCES problems(id) ON DELETE CASCADE
) ENGINE=InnoDB;
