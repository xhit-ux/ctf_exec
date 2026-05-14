package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/ctf_exec/backend/internal/config"
	_ "github.com/go-sql-driver/mysql"
)

// EnsureDatabaseExists checks whether the target database exists on the MySQL server.
// If not, it reads and executes sqlFile (which must contain CREATE DATABASE IF NOT EXISTS
// and all DDL statements) to bootstrap the database. Call this before InitDB.
func EnsureDatabaseExists(dbCfg config.DatabaseConfig, sqlFile string) error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/?charset=%s&parseTime=%t&loc=%s&multiStatements=true",
		dbCfg.User, dbCfg.Password, dbCfg.Host, dbCfg.Port,
		dbCfg.Charset, dbCfg.ParseTime, dbCfg.Local,
	)

	srv, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("连接 MySQL 失败: %w", err)
	}
	defer srv.Close()

	if err := srv.Ping(); err != nil {
		return fmt.Errorf("MySQL 连接测试失败: %w", err)
	}

	var dbExists int
	if err := srv.QueryRow(
		"SELECT COUNT(1) FROM INFORMATION_SCHEMA.SCHEMATA WHERE SCHEMA_NAME = ?",
		dbCfg.Name,
	).Scan(&dbExists); err != nil {
		return fmt.Errorf("检查数据库存在性失败: %w", err)
	}

	if dbExists > 0 {
		return nil
	}

	raw, err := os.ReadFile(sqlFile)
	if err != nil {
		return fmt.Errorf("读取 SQL 初始化文件失败 (%s): %w", sqlFile, err)
	}

	if _, err := srv.Exec(string(raw)); err != nil {
		return fmt.Errorf("执行数据库初始化脚本失败: %w", err)
	}

	log.Printf("✓ 数据库 %s 初始化完成 (来源: %s)", dbCfg.Name, sqlFile)
	return nil
}

// InitDB establishes a MySQL connection using configuration.
func InitDB(dbCfg config.DatabaseConfig) (*sql.DB, error) {
	// 构建 DSN
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=%t&loc=%s",
		dbCfg.User,
		dbCfg.Password,
		dbCfg.Host,
		dbCfg.Port,
		dbCfg.Name,
		dbCfg.Charset,
		dbCfg.ParseTime,
		dbCfg.Local,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开数据库连接失败: %w", err)
	}

	// 连接池配置
	db.SetMaxOpenConns(dbCfg.MaxOpenConns)
	db.SetMaxIdleConns(dbCfg.MaxIdleConns)
	db.SetConnMaxLifetime(dbCfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(dbCfg.ConnMaxIdleTime)

	// 测试连接
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("数据库连接测试失败: %w", err)
	}

	return db, nil
}
