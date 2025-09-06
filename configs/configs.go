package configs

import "github.com/go-sql-driver/mysql"

type Configs struct {
	Env     string `env:"APP_ENV,required"`
	Version string `env:"APP_VERSION,required"`
	URL     string `env:"APP_URL,required"`
	Port    int    `env:"APP_PORT,required"`

	Mysql *mysql.Config
}

func New() *Configs {
	return &Configs{}
}
