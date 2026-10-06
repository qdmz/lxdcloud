package db

import (
	"database/sql"
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"lxdapi/internal/core"
	"lxdapi/models"
	_ "modernc.org/sqlite"
)

var DB *gorm.DB

func Init() error {
	var err error
	cfg := core.GlobalConfig.System.Database

	switch cfg.Type {
	case "mysql":
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			cfg.MySQL.User,
			cfg.MySQL.Password,
			cfg.MySQL.Host,
			cfg.MySQL.Port,
			cfg.MySQL.Database,
		)
		DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	case "postgres":
		dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			cfg.Postgres.Host,
			cfg.Postgres.Port,
			cfg.Postgres.User,
			cfg.Postgres.Password,
			cfg.Postgres.Database,
			cfg.Postgres.SSLMode,
		)
		DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	default:
		dsn := cfg.SQLite.Path + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)"
		sqlDB, err := sql.Open("sqlite", dsn)
		if err != nil {
			return err
		}
		DB, err = gorm.Open(sqlite.Dialector{Conn: sqlDB}, &gorm.Config{})
	}

	if err != nil {
		return err
	}

	return DB.AutoMigrate(
		&models.Container{},
		&models.User{},
		&models.Traffic{},
		&models.PortMappingV4{},
		&models.PortMappingV6{},
		&models.NATConfigV4{},
		&models.NATConfigV6{},
		&models.IPv4Binding{},
		&models.IPv4Pool{},
		&models.IPv6Binding{},
		&models.IPv6Pool{},
		&models.Task{},
		&models.ContainerCredential{},
		&models.Template{},
		&models.BrandSettings{},
		&models.PortRangeConfig{},
		&models.IPPoolSettings{},
		&models.CacheAutoRefreshSettings{},
		&models.StoragePool{},
		&models.AccessToken{},
		&models.Node{},
		&models.SMTPConfig{},
		&models.MailTemplate{},
		&models.PayConfig{},
		&models.Product{},
		&models.Order{},
		&models.UserProduct{},
		&models.EmailToken{},
		&models.Ticket{},
		&models.TicketReply{},
		&models.Notification{},
	)
}

