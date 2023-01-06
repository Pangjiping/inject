package singleton_pointer

type Config struct {
	Port   string
	Memory string
}

type DB struct {
	Username string
	Password string
}

type UserService struct {
	Db   *DB     `inject:""`
	Conf *Config `inject:""`
}

type PostService struct {
	Db *DB `inject:""`
}

type UserController struct {
	UserService *UserService `inject:""`
	Conf        *Config      `inject:""`
}

type PostController struct {
	UserService *UserService `inject:""`
	PostService *PostService `inject:""`
	Conf        *Config      `inject:""`
}

type Server struct {
	UserApi *UserService    `inject:""`
	PostApi *PostController `inject:""`
}

func loadConfig() *Config {
	return &Config{
		Port:   "8080",
		Memory: "500MB",
	}
}

func connectDB() *DB {
	return &DB{
		Username: "root",
		Password: "123456",
	}
}
