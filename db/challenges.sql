-- Problems (Challenges) and Tags
CREATE TABLE problems (
    id INT PRIMARY KEY AUTO_INCREMENT,
    short_id INT UNIQUE, -- optional numeric id for simple references
    title VARCHAR(255) NOT NULL,
    category VARCHAR(32) NOT NULL,
    type VARCHAR(32) NOT NULL DEFAULT 'static', -- static / dynamic / containerized
    points INT NOT NULL DEFAULT 0,
    difficulty VARCHAR(16) NOT NULL,
    tags JSON,
    description TEXT,
    flag VARCHAR(255) NOT NULL, -- Used as the answer
    flag_rule VARCHAR(255), -- e.g. 'fixed', 'regex'
    flag_example VARCHAR(255),
    docker_image VARCHAR(255),
    run_command TEXT,
    visible_from DATETIME,
    visible_to DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB;

-- Demo problems matching frontend demo data
INSERT INTO problems (short_id, title, category, points, difficulty, description, flag, flag_example, tags)
VALUES
  (1, 'Hello Web', 'web', 100, '简单', '一个简单的 Web 题，目标是找到隐藏在页面中的 flag。', 'FLAG{demo}', 'FLAG{demo}', '["http", "xss"]'),
  (2, 'babyheap', 'pwn', 300, '困难', 'pwn 题，练习 heap 漏洞利用。', 'FLAG{pwn_demo}', 'FLAG{pwn_demo}', '["heap", "exploit"]'),
  (3, 'easycrypto', 'crypto', 150, '中等', '基础的密码学题目，包含简短的 RSA 破解思路。', 'FLAG{crypto_demo}', 'FLAG{crypto_demo}', '["rsa", "crypto"]'),
  (4, 'reverse-me', 'reverse', 200, '中等', '反汇编与逆向题，需要分析二进制行为。', 'FLAG{rev_demo}', 'FLAG{rev_demo}', '["binary", "reverse"]'),
  (5, 'misc-challenge', 'misc', 120, '简单', '杂项题目，考察逻辑与观察能力。', 'FLAG{misc_demo}', 'FLAG{misc_demo}', '["logic"]');
