package app

type Config struct {
	Addr string
	Env  string
}

type Application struct {
	Config Config
}