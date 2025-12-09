package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"path"
	"strconv"

	"github.com/caarlos0/env/v11"
)

const (
	DefaultDirName = "goph-keeper-client-data"
	LogsDir        = "logs"
	DataDir        = "data"
	TmpDirName     = "tmp"
	DbName         = "keeper.db"
)

type Config struct {
	// WorkingDir path to dir where data will store
	WorkingDir string `env:"WORKING_DIR" json:"working_dir"`
	LogLevel   string `env:"LOG_LEVEL" json:"log_level"`
	// ConfigPath path to the config file json
	ConfigPath string `env:"CONFIG,unset" json:"-"`
	// ServerAddress remote server to sync data
	ServerAddress ServerAddress `env:"SERVER_ADDRESS" json:"server_address"`
	// cwd current working directory
	cwd string
	// password for aes key and hmac
	Password string `env:"PASSWORD,notEmpty" json:"-"`
}

type ServerAddress struct {
	// Host is hostname where app will work
	Host string
	// Port is the number of port where app will work
	Port uint64
}

// String implemetation of flags.Valur interface
func (s *ServerAddress) String() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

// Set implemetation of flags.Valur interface
func (s *ServerAddress) Set(flagValue string) error {
	host, portStr, err := net.SplitHostPort(flagValue)
	if err != nil {
		return err
	}

	port, err := strconv.ParseUint(portStr, 10, 32)
	if err != nil {
		return err
	}
	s.Host = host
	s.Port = port
	return nil
}

// UnmarshalJSON for parse ServerAddress from json config
func (s *ServerAddress) UnmarshalJSON(data []byte) error {
	var str string
	err := json.Unmarshal(data, &str)
	if err != nil {
		return err
	}
	err = s.Set(str)
	if err != nil {
		return err
	}
	return nil
}

// newConfig create a new *config
func NewConfig() (*Config, error) {
	c := &Config{}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	c.cwd = cwd

	err = c.setDefaults()
	if err != nil {
		return nil, err
	}

	err = c.Init()
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Config) setDefaults() error {
	if c.WorkingDir == "" {
		c.WorkingDir = DefaultDirName
	}

	if c.LogLevel == "" {
		c.LogLevel = "info"
	}

	if c.ServerAddress.Host == "" {
		c.ServerAddress.Host = "127.0.0.1"
	}
	if c.ServerAddress.Port == 0 {
		c.ServerAddress.Port = 3030
	}
	return nil
}

func (c *Config) Init() error {
	flag.StringVar(&c.WorkingDir, "w", c.WorkingDir, "working directory")
	flag.StringVar(&c.LogLevel, "l", c.LogLevel, "log level")
	flag.StringVar(&c.ConfigPath, "config", "", "path to config(format json)")
	flag.Var(&c.ServerAddress, "a", "remote server address")
	flag.StringVar(&c.Password, "p", "", "password")
	flag.Parse()

	err := env.Parse(c)
	if err != nil {
		return fmt.Errorf("bad env: %w", err)
	}

	if c.ConfigPath != "" {
		newconfig, err := configFromFileWithFlags(c)
		if err != nil {
			return err
		}
		*c = *newconfig
	}

	return nil
}

func configFromFileWithFlags(c *Config) (*Config, error) {
	newconfig, err := getConfigFromFile(c.ConfigPath)
	if err != nil {
		return nil, err
	}
	newconfig.ConfigPath = c.ConfigPath
	// сбросить переменную окружения CONFIG и флаг -config
	// переменную окружения CONFIG сбрасывает пакет env тегом ,unset
	newArgs := make([]string, 0, len(os.Args))
	for i := 0; i < len(os.Args); i++ {
		if os.Args[i] == "-config" {
			i++
			continue
		}
		newArgs = append(newArgs, os.Args[i])
	}
	os.Args = newArgs
	// reset flag
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	err = newconfig.Init()
	if err != nil {
		return nil, err
	}
	newconfig.ConfigPath = c.ConfigPath
	return newconfig, nil
}

func getConfigFromFile(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("can not open %s: %w", path, err)
	}

	var configFromFile Config
	err = json.NewDecoder(f).Decode(&configFromFile)
	if err != nil {
		return nil, err
	}

	configFromFile.setDefaults()
	return &configFromFile, nil
}

func (c *Config) TmpDirPath() string {
	return path.Join(c.WorkingDir, TmpDirName)
}

func (c *Config) DbPath() string {
	return path.Join(c.WorkingDir, DbName)
}

func (c *Config) DataDir() string {
	return path.Join(c.WorkingDir, DataDir)
}

func (c *Config) LogDir() string {
	return path.Join(c.WorkingDir, LogsDir)
}
