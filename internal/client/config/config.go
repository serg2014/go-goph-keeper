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

type config struct {
	// WorkingDir path to dir where data will store
	WorkingDir string `env:"WORKING_DIR" json:"working_dir"`
	LogLevel   string `env:"LOG_LEVEL" json:"log_level"`
	// ConfigPath path to the config file json
	ConfigPath string `env:"CONFIG,unset" json:"-"`
}

// newConfig create a new *config
func NewConfig() (*config, error) {
	c := &config{}
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

func (c *config) setDefaults() error {
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

func (c *config) Init() error {
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

func configFromFileWithFlags(c *config) (*config, error) {
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

func getConfigFromFile(path string) (*config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("can not open %s: %w", path, err)
	}

	var configFromFile config
	err = json.NewDecoder(f).Decode(&configFromFile)
	if err != nil {
		return nil, err
	}

	configFromFile.setDefaults()
	return &configFromFile, nil
}

func (c *config) tmpDirPath() string {
	return path.Join(c.WorkingDir, TmpDirName)
}

func (c *config) DbPath() string {
	return path.Join(c.WorkingDir, DbName)
}

func (c *config) Clean() error {
	return os.RemoveAll(c.tmpDirPath())
}

func (c *config) createDirs() error {
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
	err = os.Mkdir(c.tmpDirPath(), 0700)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrTmpDir, err)
	}

	return nil
}
