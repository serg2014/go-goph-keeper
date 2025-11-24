package config

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path"

	"github.com/caarlos0/env/v11"
)

const (
	DefaultDirName = "goph-keeper-client-data"
	TmpDirName     = "tmp"
	DbName         = "keeper.db"
)

var (
	ErrTmpDir = errors.New("can not create tmp dir")
	ErrWrkDir = errors.New("can not create working dir")
)

type Config struct {
	// WorkingDir path to dir where data will store
	WorkingDir string `env:"WORKING_DIR" json:"working_dir"`
	LogLevel   string `env:"LOG_LEVEL" json:"log_level"`
	// ConfigPath path to the config file json
	ConfigPath string `env:"CONFIG,unset" json:"-"`
}

// newConfig create a new *config
func NewConfig() (*Config, error) {
	c := &Config{}
	err := c.setDefaults()
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
		dir, err := os.Getwd()
		if err != nil {
			return err
		}
		c.WorkingDir = path.Join(dir, DefaultDirName)
	}
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
	return nil
}

func (c *Config) Init() error {
	flag.StringVar(&c.WorkingDir, "w", c.WorkingDir, "working directory")
	flag.StringVar(&c.LogLevel, "l", c.LogLevel, "log level")
	flag.StringVar(&c.ConfigPath, "config", "", "path to config")
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

	err = c.createDirs()
	if err != nil {
		return err
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

func (c *Config) Clean() error {
	return os.RemoveAll(c.TmpDirPath())
}

func (c *Config) createDirs() error {
	_, err := os.Stat(c.WorkingDir)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		err = os.Mkdir(c.WorkingDir, 0700)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrWrkDir, err)
		}
	}

	c.Clean()
	err = os.Mkdir(c.TmpDirPath(), 0700)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrTmpDir, err)
	}

	return nil
}
